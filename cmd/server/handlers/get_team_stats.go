package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"log/slog"

	"github.com/akawula/DoraMatic/store"
	"github.com/akawula/DoraMatic/store/sqlc" // Import the sqlc package
	"github.com/jackc/pgx/v5/pgtype"
)

// TeamStatsResponse defines the structure for the team statistics response.
type TeamStatsResponse struct {
	DeploymentsCount int `json:"deployments_count"`
	CommitsCount     int `json:"commits_count"`
	OpenPRsCount     int `json:"open_prs_count"`
	MergedPRsCount   int `json:"merged_prs_count"`
	ClosedPRsCount   int `json:"closed_prs_count"`
	RollbacksCount   int `json:"rollbacks_count"` // Add rollbacks count
}

// GetTeamStatsHandler handles requests for team statistics.
func GetTeamStatsHandler(store store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamName := r.PathValue("teamName") // Assuming router supports path parameters like this
		if teamName == "" {
			http.Error(w, "Team name is required", http.StatusBadRequest)
			return
		}

		// Parse start and end date query parameters
		startDateStr := r.URL.Query().Get("start_date")
		endDateStr := r.URL.Query().Get("end_date")

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

		// Note: The sqlc generated functions expect different time types.
		// CountTeamCommitsByDateRange expects pgtype.Timestamptz
		// GetTeamPullRequestStatsByDateRange expects time.Time

		pgStartDate := pgtype.Timestamptz{Time: startDate, Valid: true}
		pgEndDate := pgtype.Timestamptz{Time: endDate, Valid: true}

		// Fetch commit count
		commitParams := sqlc.CountTeamCommitsByDateRangeParams{ // Corrected prefix
			Team:        teamName,
			CreatedAt:   pgStartDate, // Use pgtype.Timestamptz
			CreatedAt_2: pgEndDate,   // Use pgtype.Timestamptz
		}
		commitCount, err := store.CountTeamCommitsByDateRange(r.Context(), commitParams)
		if err != nil {
			slog.Error("Failed to get team commit count", "team", teamName, "error", err)
			http.Error(w, "Failed to retrieve commit statistics", http.StatusInternalServerError)
			return
		}

		// Fetch PR stats
		prParams := sqlc.GetTeamPullRequestStatsByDateRangeParams{ // Corrected prefix
			Team:        teamName,
			CreatedAt:   startDate, // Use time.Time
			CreatedAt_2: endDate,   // Use time.Time
		}
		prStats, err := store.GetTeamPullRequestStatsByDateRange(r.Context(), prParams)
		if err != nil {
			slog.Error("Failed to get team PR stats", "team", teamName, "error", err)
			http.Error(w, "Failed to retrieve pull request statistics", http.StatusInternalServerError)
			return
		}

		// Populate response
		// Remember "DeploymentsCount" actually means Merged PRs count based on user feedback
		stats := TeamStatsResponse{
			DeploymentsCount: int(prStats.MergedCount), // Mapped from MergedCount
			CommitsCount:     int(commitCount),
			OpenPRsCount:     int(prStats.OpenCount),
			MergedPRsCount:   int(prStats.MergedCount),
			ClosedPRsCount:   int(prStats.ClosedCount),
			RollbacksCount:   int(prStats.RollbacksCount), // Map the new count
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			slog.Error("Failed to encode team stats response", "error", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}
