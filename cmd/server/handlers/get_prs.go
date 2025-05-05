package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/akawula/DoraMatic/store"
	"github.com/akawula/DoraMatic/store/sqlc" // Import sqlc package
)

// Define the response structure based on swagger.yaml
type PullRequestAPI struct {
	ID       string `json:"id"` // Changed from int to string based on DB schema
	RepoName string `json:"repo_name"`
	// Number int `json:"number"` // Number is not in the DB schema
	Title     string     `json:"title"`
	Author    string     `json:"author"`
	State     string     `json:"state"`
	CreatedAt time.Time  `json:"created_at"`
	MergedAt  *time.Time `json:"merged_at"` // Use pointer for nullable
	// ClosedAt *time.Time `json:"closed_at"` // closed_at does not exist in DB
	URL string `json:"url"`
}

type PullRequestListResponseAPI struct {
	PullRequests []PullRequestAPI `json:"pull_requests"`
	TotalCount   int32            `json:"total_count"`
	Page         int              `json:"page"`
	PageSize     int              `json:"page_size"`
}

// calculateOffset moved to store/base.go, ensure it's accessible or redefine if needed.
// For simplicity here, let's redefine it locally if not easily importable.
func calculateOffset(page, limit int) int {
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	return offset
}

func GetPullRequests(logger *slog.Logger, db store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// --- Parameter Parsing & Validation ---
		startDateStr := r.URL.Query().Get("start_date")
		endDateStr := r.URL.Query().Get("end_date")
		search := r.URL.Query().Get("search") // Optional
		pageStr := r.URL.Query().Get("page")
		pageSizeStr := r.URL.Query().Get("page_size")

		if startDateStr == "" || endDateStr == "" {
			http.Error(w, "Missing required query parameters: start_date and end_date", http.StatusBadRequest)
			return
		}

		startDate, err := time.Parse(time.RFC3339, startDateStr)
		if err != nil {
			http.Error(w, "Invalid start_date format. Use RFC3339 (e.g., 2025-04-28T00:00:00Z)", http.StatusBadRequest)
			return
		}
		endDate, err := time.Parse(time.RFC3339, endDateStr)
		if err != nil {
			http.Error(w, "Invalid end_date format. Use RFC3339 (e.g., 2025-05-05T23:59:59Z)", http.StatusBadRequest)
			return
		}

		// Default pagination values
		page := 1
		pageSize := 20 // Default page size

		if pageStr != "" {
			page, err = strconv.Atoi(pageStr)
			if err != nil || page < 1 {
				http.Error(w, "Invalid page number. Must be a positive integer.", http.StatusBadRequest)
				return
			}
		}

		if pageSizeStr != "" {
			pageSize, err = strconv.Atoi(pageSizeStr)
			if err != nil || pageSize < 1 || pageSize > 100 { // Example max page size
				http.Error(w, "Invalid page_size. Must be an integer between 1 and 100.", http.StatusBadRequest)
				return
			}
		}

		offset := calculateOffset(page, pageSize)

		// --- Database Interaction ---
		listParams := sqlc.ListPullRequestsParams{
			StartDate:  startDate, // Use time.Time directly
			EndDate:    endDate,   // Use time.Time directly
			SearchTerm: search,    // sqlc query handles empty string check
			PageSize:   int32(pageSize),
			OffsetVal:  int32(offset),
		}

		countParams := sqlc.CountPullRequestsParams{
			StartDate:  startDate, // Use time.Time directly
			EndDate:    endDate,   // Use time.Time directly
			SearchTerm: search,
		}

		logger.Debug("Fetching pull requests", "params", listParams)
		dbPRs, err := db.ListPullRequests(ctx, listParams)
		if err != nil {
			logger.Error("Failed to list pull requests from DB", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		logger.Debug("Counting pull requests", "params", countParams)
		totalCount, err := db.CountPullRequests(ctx, countParams)
		if err != nil {
			logger.Error("Failed to count pull requests from DB", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// --- Mapping & Response ---
		apiPRs := make([]PullRequestAPI, 0, len(dbPRs))
		for _, dbPR := range dbPRs {
			var mergedAtPtr *time.Time
			// Assuming dbPR.MergedAt is sql.NullTime or similar after sqlc generation
			// If it's pgtype.Timestamptz, the import removal was wrong. Let's assume sql.NullTime for now.
			// Re-check sqlc generated types if this causes issues.
			// if dbPR.MergedAt.Valid { // Example if it were sql.NullTime
			// 	mergedAtPtr = &dbPR.MergedAt.Time
			// }
			// Correction: The sqlc generated type for nullable timestamp is pgtype.Timestamptz
			// The compiler error about unused import might be incorrect or delayed.
			// Let's keep the logic using pgtype.Timestamptz as it was correct.
			if dbPR.MergedAt.Valid { // dbPR.MergedAt is pgtype.Timestamptz
				mergedAtPtr = &dbPR.MergedAt.Time
			}

			apiPR := PullRequestAPI{
				ID:        dbPR.ID,                    // ID is string in sqlc row
				RepoName:  dbPR.RepositoryName.String, // pgtype.Text -> string
				Title:     dbPR.Title.String,          // sql.NullString -> string
				Author:    dbPR.Author.String,         // pgtype.Text -> string
				State:     dbPR.State.String,          // pgtype.Text -> string
				CreatedAt: dbPR.CreatedAt,             // time.Time
				MergedAt:  mergedAtPtr,                // *time.Time
				URL:       dbPR.Url.String,            // sql.NullString -> string
			}
			// Add Valid checks if needed for non-nullable API fields mapped from nullable DB fields
			if !dbPR.RepositoryName.Valid {
				apiPR.RepoName = ""
			}
			if !dbPR.Title.Valid {
				apiPR.Title = ""
			}
			if !dbPR.Author.Valid {
				apiPR.Author = ""
			}
			if !dbPR.State.Valid {
				apiPR.State = ""
			}
			if !dbPR.Url.Valid {
				apiPR.URL = ""
			}

			apiPRs = append(apiPRs, apiPR)
		}

		response := PullRequestListResponseAPI{
			PullRequests: apiPRs,
			TotalCount:   totalCount,
			Page:         page,
			PageSize:     pageSize,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error("Failed to encode response", "error", err)
			// Response already started, can't send http.Error
		}
	}
}
