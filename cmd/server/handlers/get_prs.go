package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os" // Import os package
	"strconv"
	"strings"
	"time"

	"github.com/akawula/DoraMatic/store"
	"github.com/akawula/DoraMatic/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// Define the response structure based on swagger.yaml
type PullRequestAPI struct {
	ID                      string     `json:"id"` // Changed from int to string based on DB schema
	RepoName                string     `json:"repo_name"`
	Title                   string     `json:"title"`
	Author                  string     `json:"author"`
	State                   string     `json:"state"`
	CreatedAt               time.Time  `json:"created_at"`
	MergedAt                *time.Time `json:"merged_at"` // Use pointer for nullable
	Additions               int32      `json:"additions"` // Add additions
	Deletions               int32      `json:"deletions"` // Add deletions
	URL                     string     `json:"url"`
	LeadTimeToCodeSeconds   *int64     `json:"lead_time_to_code_seconds"`   // Use pointer for nullable float64
	LeadTimeToReviewSeconds *int64     `json:"lead_time_to_review_seconds"` // Use pointer for nullable float64
	LeadTimeToMergeSeconds  *int64     `json:"lead_time_to_merge_seconds"`  // Use pointer for nullable float64
	PrReviewsRequestedCount *int32     `json:"pr_reviews_requested_count,omitempty"`
	JiraReferences          []string   `json:"jira_references,omitempty"`
	HasJiraReference        bool       `json:"has_jira_reference"`
}

type PullRequestListResponseAPI struct {
	PullRequests []PullRequestAPI `json:"pull_requests"`
	TotalCount   int32            `json:"total_count"`
	Page         int              `json:"page"`
	PageSize     int              `json:"page_size"`
}

func calculateOffset(page, pageSize int) int {
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize
	return offset
}

func GetPullRequests(logger *slog.Logger, db store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jiraURL := os.Getenv("JIRA_URL") // Get JIRA_URL
		ctx := r.Context()

		// --- Parameter Parsing & Validation ---
		startDateStr := r.URL.Query().Get("start_date")
		endDateStr := r.URL.Query().Get("end_date")
		search := r.URL.Query().Get("search")      // Optional
		teamName := r.URL.Query().Get("team")      // Optional team filter
		membersStr := r.URL.Query().Get("members") // New members query parameter
		pageStr := r.URL.Query().Get("page")
		pageSizeStr := r.URL.Query().Get("page_size")

		var selectedMembers []string
		if membersStr != "" {
			selectedMembers = strings.Split(membersStr, ",")
		}

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
		const maxInt32 = 2147483647
		const minInt32 = -2147483648

		// Ensure pageSize is within int32 range
		safePageSize := pageSize
		if safePageSize > maxInt32 {
			logger.Warn("pageSize exceeds int32 range, clamping to max int32",
				"original", pageSize,
				"clamped", maxInt32)
			safePageSize = maxInt32
		}

		// Ensure offset is within int32 range
		safeOffset := offset
		if safeOffset > maxInt32 {
			logger.Warn("offset exceeds int32 range, clamping to max int32",
				"original", offset,
				"clamped", maxInt32)
			safeOffset = maxInt32
		}

		listParams := sqlc.ListPullRequestsParams{
			StartDate:  startDate,
			EndDate:    endDate,
			SearchTerm: search,
			TeamName:   teamName,
			Members:    selectedMembers,     // Add members to params
			PageSize:   int32(safePageSize), // #nosec G115 - safe conversion, already checked bounds above
			OffsetVal:  int32(safeOffset),   // #nosec G115 - safe conversion, already checked bounds above
		}

		countParams := sqlc.CountPullRequestsParams{
			StartDate:  startDate,
			EndDate:    endDate,
			SearchTerm: search,
			TeamName:   teamName,
			Members:    selectedMembers, // Add members to params
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
			if dbPR.PrMergedAt.Valid {
				mergedAtPtr = &dbPR.PrMergedAt.Time
			}

			apiPR := PullRequestAPI{
				ID:        dbPR.ID,
				RepoName:  dbPR.RepositoryName.String,
				Title:     dbPR.Title.String,
				Author:    dbPR.Author.String,
				State:     dbPR.State.String,
				CreatedAt: dbPR.PrCreatedAt,
				MergedAt:  mergedAtPtr,
				Additions: dbPR.Additions.Int32,
				Deletions: dbPR.Deletions.Int32,
				URL:       dbPR.Url.String,
				// PrReviewsRequestedCount will be populated below
			}

			// Populate PrReviewsRequestedCount
			// Assuming dbPR.PrReviewsRequestedCount is pgtype.Int4 after sqlc generate
			if dbPR.PrReviewsRequestedCount.Valid {
				val := dbPR.PrReviewsRequestedCount.Int32
				apiPR.PrReviewsRequestedCount = &val
			}

			// Populate JIRA references
			var jiraRefs []string
			var hasJiraRef bool
			// Assuming dbPR.JiraReferences is the new field from sqlc generate (likely interface{} or []byte for REGEXP_MATCHES)
			if dbPR.JiraReferences != nil { // Check if the field exists and is not SQL NULL
				if refs, ok := dbPR.JiraReferences.([]interface{}); ok {
					for _, item := range refs {
						if refStr, ok := item.(string); ok {
							if jiraURL != "" {
								jiraRefs = append(jiraRefs, jiraURL+"/browse/"+refStr)
							} else {
								jiraRefs = append(jiraRefs, refStr)
							}
						}
					}
				} else if ref, ok := dbPR.JiraReferences.(string); ok && ref != "" {
					if jiraURL != "" {
						jiraRefs = append(jiraRefs, jiraURL+"/browse/"+ref)
					} else {
						jiraRefs = append(jiraRefs, ref)
					}
				} else if bytes, ok := dbPR.JiraReferences.([]byte); ok {
					var tempRefs []string
					err := json.Unmarshal(bytes, &tempRefs)
					if err == nil {
						// jiraRefs = tempRefs // This would add the whole array as one. Iterate instead.
						for _, refStr := range tempRefs {
							if jiraURL != "" {
								jiraRefs = append(jiraRefs, jiraURL+"/browse/"+refStr)
							} else {
								jiraRefs = append(jiraRefs, refStr)
							}
						}
					} else {
						// If unmarshal fails, it might be a plain string representation or PostgreSQL array literal string
						s := string(bytes)
						if s != "" && s != "null" && s != "[]" && !strings.HasPrefix(s, "{") { // Basic check to avoid adding "{}" etc.
							logger.Warn("JiraReferences from DB was []byte but not valid JSON array, treating as single string", "raw", s, "error", err)
							if jiraURL != "" {
								jiraRefs = append(jiraRefs, jiraURL+"/browse/"+s)
							} else {
								jiraRefs = append(jiraRefs, s)
							}
						} else if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && s != "{}" {
							// Handle PostgreSQL array literal string like {"ref1","ref2"}
							// This is a simplified parser, might need a more robust one for complex cases (escapes, quotes)
							trimmed := strings.Trim(s, "{}")
							if trimmed != "" {
								splitRefs := strings.Split(trimmed, ",")
								for _, r := range splitRefs { // Trim quotes if any, e.g. {"\"ref1\""}
									refStr := strings.Trim(r, "\"")
									if jiraURL != "" {
										jiraRefs = append(jiraRefs, jiraURL+"/browse/"+refStr)
									} else {
										jiraRefs = append(jiraRefs, refStr)
									}
								}
							}
						}
					}
				}
			}

			if len(jiraRefs) > 0 {
				hasJiraRef = true
			}
			apiPR.JiraReferences = jiraRefs
			apiPR.HasJiraReference = hasJiraRef

			if dbPR.LeadTimeToCodeSeconds != nil {
				if pgNum, ok := dbPR.LeadTimeToCodeSeconds.(pgtype.Numeric); ok {
					if pgNum.Valid {
						pgF8, err := pgNum.Float64Value()
						if err == nil && pgF8.Valid {
							valInt := int64(pgF8.Float64)
							apiPR.LeadTimeToCodeSeconds = &valInt
						}
					}
				}
			}

			if dbPR.LeadTimeToReviewSeconds != nil {
				if pgNum, ok := dbPR.LeadTimeToReviewSeconds.(pgtype.Numeric); ok {
					if pgNum.Valid {
						pgF8, err := pgNum.Float64Value()
						if err == nil && pgF8.Valid {
							valInt := int64(pgF8.Float64)
							apiPR.LeadTimeToReviewSeconds = &valInt
						}
					}
				}
			}

			if dbPR.LeadTimeToMergeSeconds != nil {
				if pgNum, ok := dbPR.LeadTimeToMergeSeconds.(pgtype.Numeric); ok {
					if pgNum.Valid {
						pgF8, err := pgNum.Float64Value()
						if err == nil && pgF8.Valid {
							valInt := int64(pgF8.Float64)
							apiPR.LeadTimeToMergeSeconds = &valInt
						}
					}
				}
			}

			if !dbPR.Additions.Valid {
				apiPR.Additions = 0 // Default to 0 if null
			}
			if !dbPR.Deletions.Valid {
				apiPR.Deletions = 0 // Default to 0 if null
			}
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
