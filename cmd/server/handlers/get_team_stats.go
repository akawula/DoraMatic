package handlers

import (
	"context" // Added
	"encoding/json"
	"fmt" // Added
	"net/http"
	"strings" // Added for splitting members string
	"time"

	"log/slog"

	"github.com/akawula/DoraMatic/store"
	"github.com/akawula/DoraMatic/store/sqlc" // Import the sqlc package
)

// SinglePeriodStats defines the stats for one period.
type SinglePeriodStats struct {
	DeploymentsCount              int     `json:"deployments_count"`
	CommitsCount                  int     `json:"commits_count"`
	MergedPRsCount                int     `json:"merged_prs_count"`
	ClosedPRsCount                int     `json:"closed_prs_count"`
	RollbacksCount                int     `json:"rollbacks_count"`
	AvgLeadTimeToCodeSeconds      float64 `json:"avg_lead_time_to_code_seconds"` // Changed from Hours
	CountPRsForAvgLeadTime        int     `json:"count_prs_for_avg_lead_time"`   // Kept for debugging
	AvgLeadTimeToReviewSeconds    float64 `json:"avg_lead_time_to_review_seconds"`
	AvgLeadTimeToMergeSeconds     float64 `json:"avg_lead_time_to_merge_seconds"`
	CountPRsForAvgLeadTimeToMerge int     `json:"count_prs_for_avg_lead_time_to_merge"`
	AvgPRSizeLines                float64 `json:"avg_pr_size_lines"`
	ChangeFailureRate             float64 `json:"change_failure_rate_percentage"`
	AvgCommitsPerMergedPR         float64 `json:"avg_commits_per_merged_pr"`
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
		previousStats, err := fetchStatsForPeriod(r.Context(), store, teamName, prevStartDate, prevEndDate, selectedMembers)
		if err != nil {
			// Log the error but don't fail the request; we can still show current stats
			slog.Warn("Failed to get previous period stats", "team", teamName, "start", prevStartDate, "end", prevEndDate, "members", selectedMembers, "error", err)
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
			// i=0: go back 5 durations (oldest)
			// i=4: go back 1 duration (just before current)
			offsetMultiplier := time.Duration(numTrendPeriods - 1 - i)
			periodStartDate := startDate.Add(-offsetMultiplier * duration)
			periodEndDate := endDate.Add(-offsetMultiplier * duration)

			// Adjust periodEndDate to be just before the start of the next period to avoid overlap
			// This is similar to how prevEndDate was calculated for the 'previous' period.
			// If it's not the period immediately preceding the 'previous' period, adjust relative to its own start.
			if i < numTrendPeriods-2 { // Not the period immediately before currentStats's previous period
				periodEndDate = periodStartDate.Add(duration).Add(-1 * time.Nanosecond)
			} else { // This is the period that would be 'previousStats.Previous' if we went further back
				periodEndDate = periodStartDate.Add(duration).Add(-1 * time.Nanosecond)
			}


			slog.Info("Fetching trend period stats", "period_index", i, "start_date", periodStartDate, "end_date", periodEndDate)
			stats, err := fetchStatsForPeriod(r.Context(), store, teamName, periodStartDate, periodEndDate, selectedMembers)
			if err != nil {
				slog.Warn("Failed to get trend period stats", "period_index", i, "team", teamName, "start", periodStartDate, "end", periodEndDate, "error", err)
				periodStatsList[i] = &SinglePeriodStats{} // Use zero stats on error
			} else {
				periodStatsList[i] = stats
			}
		}
		// Ensure periodStatsList[numTrendPeriods-2] is previousStats if it was successfully fetched, or the one we just calculated
		// This handles the case where previousStats might have failed but the loop for trends succeeded for that slot.
		if previousStats != nil && err == nil { // if previousStats was fetched successfully
			periodStatsList[numTrendPeriods-2] = previousStats
		}


		// Define keys for trend data (must match frontend)
		trendStatKeys := []string{
			"merged_prs_count", "closed_prs_count", "commits_count", "rollbacks_count",
			"avg_lead_time_to_code_seconds", "avg_lead_time_to_review_seconds", "avg_lead_time_to_merge_seconds",
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
				case "avg_pr_size_lines":
					values[j] = pStats.AvgPRSizeLines
				case "change_failure_rate_percentage":
					values[j] = pStats.ChangeFailureRate
				case "avg_commits_per_merged_pr":
					values[j] = pStats.AvgCommitsPerMergedPR
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
		// Return zero stats for invalid ranges to avoid query errors and allow trend to show a zero point
		return &SinglePeriodStats{}, nil
	}

	commitParams := sqlc.CountTeamCommitsByDateRangeParams{
		TeamName:          teamName,
		Members:           selectedMembers,
		MergedAtStartDate: startDate,
		MergedAtEndDate:   endDate,
	}
	commitCount, err := store.CountTeamCommitsByDateRange(ctx, commitParams)
	if err != nil {
		// Log specific error for commit count
		slog.Error("fetchStatsForPeriod: failed to get commit count", "team", teamName, "start", startDate, "end", endDate, "members", selectedMembers, "error", err)
		// Decide if you want to return partial stats or fail entirely.
		// For trends, it might be better to return an error so the caller can decide to use zero values.
		return nil, fmt.Errorf("failed to get commit count for period %s to %s: %w", startDate, endDate, err)
	}

	prParams := sqlc.GetTeamPullRequestStatsByDateRangeParams{
		TeamName:  teamName,
		StartDate: startDate,
		EndDate:   endDate,
		Members:   selectedMembers,
	}
	prStats, err := store.GetTeamPullRequestStatsByDateRange(ctx, prParams)
	if err != nil {
		// Log specific error for PR stats
		slog.Error("fetchStatsForPeriod: failed to get PR stats", "team", teamName, "start", startDate, "end", endDate, "members", selectedMembers, "error", err)
		return nil, fmt.Errorf("failed to get PR stats for period %s to %s: %w", startDate, endDate, err)
	}

	stats := &SinglePeriodStats{
		DeploymentsCount:              int(prStats.MergedCount),
		CommitsCount:                  int(commitCount),
		MergedPRsCount:                int(prStats.MergedCount),
		ClosedPRsCount:                int(prStats.ClosedCount),
		RollbacksCount:                int(prStats.RollbacksCount),
		AvgLeadTimeToCodeSeconds:      prStats.AvgLeadTimeToCodeSeconds,
		CountPRsForAvgLeadTime:        int(prStats.CountPrsForAvgLeadTime),
		AvgLeadTimeToReviewSeconds:    prStats.AvgLeadTimeToReviewSeconds,
		AvgLeadTimeToMergeSeconds:     prStats.AvgLeadTimeToMergeSeconds,
		CountPRsForAvgLeadTimeToMerge: int(prStats.CountPrsForAvgLeadTimeToMerge),
		// Calculate AvgPRSizeLines
		AvgPRSizeLines: calculateAvgPRSize(prStats.TotalAdditions, prStats.TotalDeletions, prStats.MergedCount),
		// Calculate ChangeFailureRate
		ChangeFailureRate: calculateChangeFailureRate(prStats.RollbacksCount, prStats.MergedCount),
		// Calculate AvgCommitsPerMergedPR
		AvgCommitsPerMergedPR: calculateAvgCommitsPerMergedPR(int64(commitCount), prStats.MergedCount),
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
