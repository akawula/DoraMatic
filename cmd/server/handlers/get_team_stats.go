package handlers

import (
	"context" // Added
	"encoding/json"
	"fmt" // Added
	"net/http"
	"strings" // Added for splitting members string
	"time"

	"log/slog"

	"github.com/akawula/DoraMatic/internal/timeutils" // Added for business time calculation
	"github.com/akawula/DoraMatic/store"
	"github.com/akawula/DoraMatic/store/sqlc" // Import the sqlc package
	"github.com/jackc/pgx/v5/pgtype"         // Added for pgtype.Timestamptz
)

// SinglePeriodStats defines the stats for one period.
type SinglePeriodStats struct {
	DeploymentsCount              int     `json:"deployments_count"`
	CommitsCount                  int     `json:"commits_count"`
	MergedPRsCount                int     `json:"merged_prs_count"`
	ClosedPRsCount                int     `json:"closed_prs_count"`
	AvgTimeToCloseSeconds         float64 `json:"avg_time_to_close_seconds"`
	RollbacksCount                int     `json:"rollbacks_count"`
	AvgLeadTimeToCodeSeconds      float64 `json:"avg_lead_time_to_code_seconds"`
	CountPRsForAvgLeadTime        int     `json:"count_prs_for_avg_lead_time"`
	AvgLeadTimeToReviewSeconds    float64 `json:"avg_lead_time_to_review_seconds"` // Time from review request to merge
	CountPRsForAvgLeadTimeToReview int    `json:"count_prs_for_avg_lead_time_to_review"`
	AvgLeadTimeToMergeSeconds     float64 `json:"avg_lead_time_to_merge_seconds"` // Time from first commit to merge
	CountPRsForAvgLeadTimeToMerge int     `json:"count_prs_for_avg_lead_time_to_merge"`
	AvgTimeToFirstActualReviewSeconds float64 `json:"avg_time_to_first_actual_review_seconds"` // Time from first commit to first actual review
	CountPrsForAvgTimeToFirstActualReview int `json:"count_prs_for_avg_time_to_first_actual_review"`
	AvgReviewsRequestedPerPR      float64 `json:"avg_reviews_requested_per_pr"`
	TotalReviewsRequestedOnPRs    int64   `json:"total_reviews_requested_on_prs"` // For diagnostics or future use
	CountPRsWithReviewRequests    int32   `json:"count_prs_with_review_requests"` // For diagnostics or future use
	AvgPRSizeLines                float64 `json:"avg_pr_size_lines"`
	ChangeFailureRate             float64 `json:"change_failure_rate_percentage"`
	AvgCommitsPerMergedPR         float64 `json:"avg_commits_per_merged_pr"`
	QuietHeroName                 string  `json:"quiet_hero_name,omitempty"`
	QuietHeroStat                 int64   `json:"quiet_hero_stat,omitempty"`
}

// TeamStatsComparisonResponse defines the structure for comparing two periods and including trend data.
type TeamStatsComparisonResponse struct {
	Current  SinglePeriodStats   `json:"current"`
	Previous SinglePeriodStats   `json:"previous"`
	Trend    map[string][]float64 `json:"trend"`
}

// GetTeamStatsHandler handles requests for team statistics comparison.
func GetTeamStatsHandler(store store.Store) http.HandlerFunc { //nolint:maintidx // Allow higher cyclomatic complexity for this handler
	return func(w http.ResponseWriter, r *http.Request) {
		teamName := r.PathValue("teamName")
		if teamName == "" {
			http.Error(w, "Team name is required", http.StatusBadRequest)
			return
		}

		// Parse start and end date query parameters
		startDateStr := r.URL.Query().Get("start_date")
		endDateStr := r.URL.Query().Get("end_date")
		membersStr := r.URL.Query().Get("members") // Read new members query parameter

		var selectedMembers []string
		if membersStr != "" {
			selectedMembers = strings.Split(membersStr, ",")
		}

		if startDateStr == "" || endDateStr == "" {
			http.Error(w, "start_date and end_date query parameters are required", http.StatusBadRequest)
			return
		}

		startDate, err := time.Parse(time.RFC3339, startDateStr)
		if err != nil {
			http.Error(w, "Invalid start_date format (use RFC3339)", http.StatusBadRequest)
			return
		}

		endDate, err := time.Parse(time.RFC3339, endDateStr)
		if err != nil {
			http.Error(w, "Invalid end_date format (use RFC3339)", http.StatusBadRequest)
			return
		}

		// Calculate previous period dates
		duration := endDate.Sub(startDate)
		prevEndDate := startDate.Add(-1 * time.Nanosecond) // End just before the current start date
		prevStartDate := prevEndDate.Add(-duration)

		// --- Fetch stats for the CURRENT period ---
		currentStats, err := fetchStatsForPeriod(r.Context(), store, teamName, startDate, endDate, selectedMembers)
		if err != nil {
			slog.Error("Failed to get current period stats", "team", teamName, "start", startDate, "end", endDate, "members", selectedMembers, "error", err)
			http.Error(w, "Failed to retrieve current period statistics", http.StatusInternalServerError)
			return
		}

		// --- Fetch stats for the PREVIOUS period ---
		// For the previous period, we use the same member selection as the current period.
		var previousStats *SinglePeriodStats
		var previousStatsErr error
		previousStats, previousStatsErr = fetchStatsForPeriod(r.Context(), store, teamName, prevStartDate, prevEndDate, selectedMembers)
		if previousStatsErr != nil {
			// Log the error but don't fail the request; we can still show current stats
			slog.Warn("Failed to get previous period stats", "team", teamName, "start", prevStartDate, "end", prevEndDate, "members", selectedMembers, "error", previousStatsErr)
			// Initialize previousStats with zero values if fetching failed
			previousStats = &SinglePeriodStats{}
		}

		// --- Fetch trend data for the last 6 periods ---
		trendData := make(map[string][]float64)
		numTrendPeriods := 6
		periodStatsList := make([]*SinglePeriodStats, numTrendPeriods)

		// The last period in the trend is the current period
		periodStatsList[numTrendPeriods-1] = currentStats

		// Fetch stats for the 5 preceding periods
		for i := 0; i < numTrendPeriods-1; i++ {
			// Calculate start and end dates for this past period
			// (numTrendPeriods-1-i) ensures we go back further for smaller i
			// Example: if numTrendPeriods = 6
			// i=0: go back 5 durations (oldest) -> k_val = 5
			// i=4: go back 1 duration (just before current) -> k_val = 1 (this is previousStats' period)

			// k_val determines how many periods to step back:
			// k_val = 1 for the period immediately preceding the current period.
			// k_val = numTrendPeriods - 1 for the oldest period in the trend.
			// Loop index i goes from 0 (oldest) to numTrendPeriods-2 (slot for previousStats).
			k_val := (numTrendPeriods - 1) - i

			var trendPeriodStartDate, trendPeriodEndDate time.Time
			// For past periods (k_val >= 1, which is always true in this loop)
			// Calculate end date: Start from current period's start_date, subtract 1ns (to get prev_period's end_date),
			// then subtract (k_val - 1) full durations.
			trendPeriodEndDate = startDate.Add(-1 * time.Nanosecond).Add(-time.Duration(k_val-1) * duration)
			// Calculate start date: It's one duration before its trendPeriodEndDate.
			trendPeriodStartDate = trendPeriodEndDate.Add(-duration)

			slog.Info("Fetching trend period stats", "period_index", i, "k_val_from_current", k_val, "start_date", trendPeriodStartDate, "end_date", trendPeriodEndDate)
			stats, fetchErr := fetchStatsForPeriod(r.Context(), store, teamName, trendPeriodStartDate, trendPeriodEndDate, selectedMembers)
			if fetchErr != nil {
				slog.Warn("Failed to get trend period stats", "period_index", i, "k_val_from_current", k_val, "team", teamName, "start", trendPeriodStartDate, "end", trendPeriodEndDate, "error", fetchErr)
				periodStatsList[i] = &SinglePeriodStats{} // Use zero stats on error
			} else {
				periodStatsList[i] = stats
			}
		}
		// Ensure periodStatsList[numTrendPeriods-2] is previousStats if it was successfully fetched.
		// This handles the case where previousStats might have failed but the loop for trends succeeded for that slot,
		// or vice-versa. Now, if both succeed, they are for the same date range.
		if previousStatsErr == nil { // if previousStats was fetched successfully
			periodStatsList[numTrendPeriods-2] = previousStats
		}


		// Define keys for trend data (must match frontend)
		trendStatKeys := []string{
			"merged_prs_count", "closed_prs_count", "avg_time_to_close_seconds", "commits_count", "rollbacks_count",
			"avg_lead_time_to_code_seconds", "avg_lead_time_to_review_seconds", "avg_lead_time_to_merge_seconds", "avg_time_to_first_actual_review_seconds",
			"avg_reviews_requested_per_pr",
			"avg_pr_size_lines", "change_failure_rate_percentage", "avg_commits_per_merged_pr",
		}

		for _, key := range trendStatKeys {
			values := make([]float64, numTrendPeriods)
			for j, pStats := range periodStatsList {
				if pStats == nil { // Should be initialized to empty struct, but defensive check
					values[j] = 0
					continue
				}
				switch key {
				case "merged_prs_count":
					values[j] = float64(pStats.MergedPRsCount)
				case "closed_prs_count":
					values[j] = float64(pStats.ClosedPRsCount)
				case "avg_time_to_close_seconds":
					values[j] = pStats.AvgTimeToCloseSeconds
				case "commits_count":
					values[j] = float64(pStats.CommitsCount)
				case "rollbacks_count":
					values[j] = float64(pStats.RollbacksCount)
				case "avg_lead_time_to_code_seconds":
					values[j] = pStats.AvgLeadTimeToCodeSeconds
				case "avg_lead_time_to_review_seconds":
					values[j] = pStats.AvgLeadTimeToReviewSeconds
				case "avg_lead_time_to_merge_seconds":
					values[j] = pStats.AvgLeadTimeToMergeSeconds
				case "avg_time_to_first_actual_review_seconds":
					values[j] = pStats.AvgTimeToFirstActualReviewSeconds
				case "avg_reviews_requested_per_pr": // New trend key
					values[j] = pStats.AvgReviewsRequestedPerPR
				case "avg_pr_size_lines":
					values[j] = pStats.AvgPRSizeLines
				case "change_failure_rate_percentage":
					values[j] = pStats.ChangeFailureRate
				case "avg_commits_per_merged_pr":
					values[j] = pStats.AvgCommitsPerMergedPR
				// case "avg_reviews_per_user": // Removed
				// 	values[j] = pStats.AvgReviewsPerUser
				default:
					values[j] = 0 // Should not happen if keys are correct
				}
			}
			trendData[key] = values
		}

		// Populate comparison response
		comparisonResponse := TeamStatsComparisonResponse{
			Current:  *currentStats,
			Previous: *previousStats, // This remains the single previous period for direct comparison
			Trend:    trendData,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(comparisonResponse); err != nil {
			slog.Error("Failed to encode team stats comparison response", "error", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

// fetchStatsForPeriod fetches commit and PR stats for a given period, optionally filtered by members.
func fetchStatsForPeriod(ctx context.Context, store store.Store, teamName string, startDate, endDate time.Time, selectedMembers []string) (*SinglePeriodStats, error) {
	if startDate.IsZero() || endDate.IsZero() || endDate.Before(startDate) {
		slog.Warn("Invalid date range for fetchStatsForPeriod", "team", teamName, "start", startDate, "end", endDate)
		return &SinglePeriodStats{}, nil
	}

	// --- Fetch raw PR time data for lead time calculations ---
	prTimeDataParams := sqlc.GetPullRequestTimeDataForStatsParams{
		TeamName:  teamName,
		StartDate: startDate,
		EndDate:   endDate,
		Members:   selectedMembers,
	}
	prTimeDataRows, err := store.GetPullRequestTimeDataForStats(ctx, prTimeDataParams)
	if err != nil {
		slog.Error("fetchStatsForPeriod: failed to get PR time data for stats", "team", teamName, "start", startDate, "end", endDate, "members", selectedMembers, "error", err)
		return nil, fmt.Errorf("failed to get PR time data for period %s to %s: %w", startDate, endDate, err)
	}

	// Initialize accumulators for lead time calculations
	var totalLeadTimeToCodeBizSeconds float64
	var countLeadTimeToCode int
	var totalLeadTimeToReviewBizSeconds float64 // From review request to merge
	var countLeadTimeToReview int
	var totalLeadTimeToMergeBizSeconds float64 // From first commit to merge
	var countLeadTimeToMerge int
	var totalTimeToFirstActualReviewBizSeconds float64 // From first commit to first actual review
	var countTimeToFirstActualReview int
	var totalReviewsRequestedOnPRs int64 // Sum of reviews_requested on PRs
	var countPRsWithReviewRequests int32   // Count of PRs considered for the average

	for _, prData := range prTimeDataRows {
		var fcAt, faReviewAt pgtype.Timestamptz

		// Handle FirstCommitAt
		if prData.FirstCommitAt != nil {
			if t, ok := prData.FirstCommitAt.(time.Time); ok {
				fcAt = pgtype.Timestamptz{Time: t, Valid: true}
			} else if tPG, ok := prData.FirstCommitAt.(pgtype.Timestamptz); ok {
				fcAt = tPG
			} else {
				slog.WarnContext(ctx, "Unexpected type for FirstCommitAt, treating as invalid", "pr_id", prData.PrID, "type", fmt.Sprintf("%T", prData.FirstCommitAt), "value", prData.FirstCommitAt)
				fcAt = pgtype.Timestamptz{Valid: false}
			}
		} else {
			fcAt = pgtype.Timestamptz{Valid: false} // SQL NULL
		}

		// Handle FirstActualReviewAt
		if prData.FirstActualReviewAt != nil {
			if t, ok := prData.FirstActualReviewAt.(time.Time); ok {
				faReviewAt = pgtype.Timestamptz{Time: t, Valid: true}
			} else if tPG, ok := prData.FirstActualReviewAt.(pgtype.Timestamptz); ok {
				faReviewAt = tPG
			} else {
				slog.WarnContext(ctx, "Unexpected type for FirstActualReviewAt, treating as invalid", "pr_id", prData.PrID, "type", fmt.Sprintf("%T", prData.FirstActualReviewAt), "value", prData.FirstActualReviewAt)
				faReviewAt = pgtype.Timestamptz{Valid: false}
			}
		} else {
			faReviewAt = pgtype.Timestamptz{Valid: false} // SQL NULL
		}

		slog.DebugContext(ctx, "Processing PR for lead time calculation",
			"pr_id", prData.PrID,
			"first_commit_at_valid", fcAt.Valid,
			"first_commit_at_time", slog.TimeValue(func() time.Time { if fcAt.Valid { return fcAt.Time }; return time.Time{} }()),
			"first_actual_review_at_valid", faReviewAt.Valid,
			"first_actual_review_at_time", slog.TimeValue(func() time.Time { if faReviewAt.Valid { return faReviewAt.Time }; return time.Time{} }()),
			"pr_review_requested_at_valid", prData.PrReviewRequestedAt.Valid,
			"pr_review_requested_at_time", slog.TimeValue(func() time.Time { if prData.PrReviewRequestedAt.Valid { return prData.PrReviewRequestedAt.Time }; return time.Time{} }()),
			"pr_merged_at_valid", prData.PrMergedAt.Valid,
			"pr_merged_at_time", slog.TimeValue(func() time.Time { if prData.PrMergedAt.Valid { return prData.PrMergedAt.Time }; return time.Time{} }()),
			// PrCreatedAt is time.Time, so it's always "valid" in Go terms if not zero.
			// The zero value of time.Time can be checked with .IsZero()
			"pr_created_at_is_zero", prData.PrCreatedAt.IsZero(),
			"pr_created_at_time", slog.TimeValue(prData.PrCreatedAt),
			"pr_reviews_requested_valid", prData.PrReviewsRequested.Valid, // Assuming PrReviewsRequested is pgtype.Int4
			"pr_reviews_requested_count", func() int32 { // Safely get count
				if prData.PrReviewsRequested.Valid {
					return prData.PrReviewsRequested.Int32
				}
				return 0
			}(),
			"pr_state_valid", prData.PrState.Valid,
			"pr_state_string", slog.StringValue(func() string { if prData.PrState.Valid { return prData.PrState.String }; return "N/A" }()),
		)

		// Lead Time to Code (first commit to review requested)
		if fcAt.Valid && prData.PrReviewRequestedAt.Valid && prData.PrReviewRequestedAt.Time.After(fcAt.Time) {
			bizSecs := timeutils.CalculateBusinessSeconds(fcAt.Time, prData.PrReviewRequestedAt.Time)
			totalLeadTimeToCodeBizSeconds += bizSecs
			countLeadTimeToCode++
		}

		// Accumulate Reviews Requested Count for PRs created in the period
		// We consider PRs created within the [startDate, endDate] for this average.
		if !prData.PrCreatedAt.IsZero() &&
			(prData.PrCreatedAt.After(startDate) || prData.PrCreatedAt.Equal(startDate)) &&
			(prData.PrCreatedAt.Before(endDate) || prData.PrCreatedAt.Equal(endDate)) {
			if prData.PrReviewsRequested.Valid {
				totalReviewsRequestedOnPRs += int64(prData.PrReviewsRequested.Int32)
			}
			// We count all PRs created in the period for the denominator, even if reviews_requested is 0 or null.
			countPRsWithReviewRequests++
		}

		// Lead Time to Review (review requested to merge)
		if prData.PrState.Valid && prData.PrState.String == "MERGED" &&
			prData.PrReviewRequestedAt.Valid && prData.PrMergedAt.Valid &&
			prData.PrMergedAt.Time.After(prData.PrReviewRequestedAt.Time) {
			bizSecs := timeutils.CalculateBusinessSeconds(prData.PrReviewRequestedAt.Time, prData.PrMergedAt.Time)
			totalLeadTimeToReviewBizSeconds += bizSecs
			countLeadTimeToReview++
		}

		// Lead Time to Merge (first commit to merge)
		if prData.PrState.Valid && prData.PrState.String == "MERGED" &&
			fcAt.Valid && prData.PrMergedAt.Valid &&
			prData.PrMergedAt.Time.After(fcAt.Time) {
			bizSecs := timeutils.CalculateBusinessSeconds(fcAt.Time, prData.PrMergedAt.Time)
			totalLeadTimeToMergeBizSeconds += bizSecs
			countLeadTimeToMerge++
		}

		// Time to First Actual Review (first commit to first actual review submission)
		if fcAt.Valid && faReviewAt.Valid &&
			faReviewAt.Time.After(fcAt.Time) {
			bizSecs := timeutils.CalculateBusinessSeconds(fcAt.Time, faReviewAt.Time)
			totalTimeToFirstActualReviewBizSeconds += bizSecs
			countTimeToFirstActualReview++
		}
	}

	// Calculate average lead times
	var avgLeadTimeToCodeSeconds float64
	if countLeadTimeToCode > 0 {
		avgLeadTimeToCodeSeconds = totalLeadTimeToCodeBizSeconds / float64(countLeadTimeToCode)
	}
	var avgLeadTimeToReviewSeconds float64
	if countLeadTimeToReview > 0 {
		avgLeadTimeToReviewSeconds = totalLeadTimeToReviewBizSeconds / float64(countLeadTimeToReview)
	}
	var avgLeadTimeToMergeSeconds float64
	if countLeadTimeToMerge > 0 {
		avgLeadTimeToMergeSeconds = totalLeadTimeToMergeBizSeconds / float64(countLeadTimeToMerge)
	}
	var avgTimeToFirstActualReviewSeconds float64
	if countTimeToFirstActualReview > 0 {
		avgTimeToFirstActualReviewSeconds = totalTimeToFirstActualReviewBizSeconds / float64(countTimeToFirstActualReview)
	}
	var avgReviewsRequestedPerPR float64
	if countPRsWithReviewRequests > 0 {
		avgReviewsRequestedPerPR = float64(totalReviewsRequestedOnPRs) / float64(countPRsWithReviewRequests)
	}

	// --- Fetch other non-lead-time stats (counts, sums) ---
	// Using the old GetTeamPullRequestStatsByDateRange for these, as it's already aggregating them.
	// Its lead time calculations will be ignored.
	otherPrStatsParams := sqlc.GetTeamPullRequestStatsByDateRangeParams{
		TeamName:  teamName,
		StartDate: startDate,
		EndDate:   endDate,
		Members:   selectedMembers,
	}
	otherPrStats, err := store.GetTeamPullRequestStatsByDateRange(ctx, otherPrStatsParams)
	if err != nil {
		slog.Error("fetchStatsForPeriod: failed to get other PR stats", "team", teamName, "start", startDate, "end", endDate, "members", selectedMembers, "error", err)
		return nil, fmt.Errorf("failed to get other PR stats for period %s to %s: %w", startDate, endDate, err)
	}

	commitParams := sqlc.CountTeamCommitsByDateRangeParams{
		TeamName:          teamName,
		Members:           selectedMembers,
		MergedAtStartDate: startDate, // Commits are counted for PRs merged in the period
		MergedAtEndDate:   endDate,
	}
	commitCount, err := store.CountTeamCommitsByDateRange(ctx, commitParams)
	if err != nil {
		slog.Error("fetchStatsForPeriod: failed to get commit count", "team", teamName, "start", startDate, "end", endDate, "members", selectedMembers, "error", err)
		return nil, fmt.Errorf("failed to get commit count for period %s to %s: %w", startDate, endDate, err)
	}

	stats := &SinglePeriodStats{
		DeploymentsCount:              int(otherPrStats.MergedCount), // Assuming deployments = merged PRs
		CommitsCount:                  int(commitCount),
		MergedPRsCount:                int(otherPrStats.MergedCount),
		ClosedPRsCount:                int(otherPrStats.ClosedCount),
		AvgTimeToCloseSeconds:         otherPrStats.AvgTimeToCloseSeconds,
		RollbacksCount:                int(otherPrStats.RollbacksCount),
		AvgLeadTimeToCodeSeconds:      avgLeadTimeToCodeSeconds,
		CountPRsForAvgLeadTime:        countLeadTimeToCode,
		AvgLeadTimeToReviewSeconds:    avgLeadTimeToReviewSeconds,
		CountPRsForAvgLeadTimeToReview: countLeadTimeToReview,
		AvgLeadTimeToMergeSeconds:     avgLeadTimeToMergeSeconds,
		CountPRsForAvgLeadTimeToMerge: countLeadTimeToMerge,
		AvgTimeToFirstActualReviewSeconds: avgTimeToFirstActualReviewSeconds,
		CountPrsForAvgTimeToFirstActualReview: countTimeToFirstActualReview,
		AvgReviewsRequestedPerPR:      avgReviewsRequestedPerPR,
		TotalReviewsRequestedOnPRs:    totalReviewsRequestedOnPRs,
		CountPRsWithReviewRequests:    countPRsWithReviewRequests,
		AvgPRSizeLines: calculateAvgPRSize(otherPrStats.TotalAdditions, otherPrStats.TotalDeletions, otherPrStats.MergedCount),
		ChangeFailureRate: calculateChangeFailureRate(otherPrStats.RollbacksCount, otherPrStats.MergedCount),
		AvgCommitsPerMergedPR: calculateAvgCommitsPerMergedPR(int64(commitCount), otherPrStats.MergedCount),
		// AvgReviewsPerUser removed
	}

	// --- Fetch Quiet Hero (Top Reviewer) ---
	teamMemberReviewStatsParams := sqlc.GetTeamMemberReviewStatsByDateRangeParams{
		TeamName:  teamName,
		StartDate: startDate,
		EndDate:   endDate,
		Members:   selectedMembers,
	}
	teamMemberReviewStats, err := store.GetTeamMemberReviewStatsByDateRange(ctx, teamMemberReviewStatsParams)
	if err != nil {
		// Log error but don't fail the whole stats retrieval, hero is supplementary
		slog.Error("fetchStatsForPeriod: failed to get team member review stats for quiet hero", "team", teamName, "start", startDate, "end", endDate, "members", selectedMembers, "error", err)
	} else if len(teamMemberReviewStats) > 0 {
		// The query orders by total_reviews_submitted DESC, so the first one is the top reviewer
		topReviewer := teamMemberReviewStats[0]
		if topReviewer.AuthorLogin.Valid {
			stats.QuietHeroName = topReviewer.AuthorLogin.String
			stats.QuietHeroStat = topReviewer.TotalReviewsSubmitted
		}
	}

	return stats, nil
}

func calculateAvgPRSize(totalAdditions, totalDeletions int64, mergedPRsCount int32) float64 {
	if mergedPRsCount == 0 {
		return 0
	}
	return float64(totalAdditions+totalDeletions) / float64(mergedPRsCount)
}

func calculateChangeFailureRate(rollbacksCount, deploymentsCount int32) float64 {
	if deploymentsCount == 0 {
		return 0 // Avoid division by zero; if no deployments, CFR is 0 or undefined.
	}
	return (float64(rollbacksCount) / float64(deploymentsCount)) * 100
}

func calculateAvgCommitsPerMergedPR(totalCommitsInMergedPRs int64, mergedPRsCount int32) float64 {
	if mergedPRsCount == 0 {
		return 0
	}
	// Note: commitCount from CountTeamCommitsByDateRange already filters commits
	// by PRs merged within the specified date range.
	return float64(totalCommitsInMergedPRs) / float64(mergedPRsCount)
}

func calculateAvgReviewsPerUser(totalTeamReviewsSubmitted int64, distinctTeamReviewersCount int32) float64 {
	if distinctTeamReviewersCount == 0 {
		return 0
	}
	return float64(totalTeamReviewsSubmitted) / float64(distinctTeamReviewersCount)
}
