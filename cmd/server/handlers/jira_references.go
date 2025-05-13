package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os" // Added for environment variable access
	"strconv"
	"time"

	"github.com/akawula/DoraMatic/store"
	"github.com/akawula/DoraMatic/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// JiraReferenceStatus represents whether a PR has JIRA references
type JiraReferenceStatus string

const (
// WithJira means PR has JIRA references
WithJira JiraReferenceStatus = "with_jira"
// WithoutJira means PR doesn't have JIRA references
WithoutJira JiraReferenceStatus = "without_jira"
// All means return all PRs regardless of JIRA status
All JiraReferenceStatus = "all"
)

// PullRequestJiraAPI extends PullRequestAPI with JIRA reference data
type PullRequestJiraAPI struct {
PullRequestAPI
JiraReferences   []string `json:"jira_references,omitempty"` // Array of JIRA references found
HasJiraReference bool     `json:"has_jira_reference"`        // True if PR has at least one JIRA reference
}

// PullRequestJiraResponseAPI is the response structure for the Jira references endpoint
type PullRequestJiraResponseAPI struct {
PullRequests         []PullRequestJiraAPI `json:"pull_requests"`
TotalCount           int32                `json:"total_count"`
Page                 int                  `json:"page"`
PageSize             int                  `json:"page_size"`
WithJiraCount        int64                `json:"with_jira_count"`
WithoutJiraCount     int64                `json:"without_jira_count"`
JiraReferencePercent float64              `json:"jira_reference_percent"`
}

// GetPullRequestsJiraReferences returns an HTTP handler for getting PRs with their JIRA reference status
func GetPullRequestsJiraReferences(logger *slog.Logger, db store.Store) http.HandlerFunc {
	jiraURL := os.Getenv("JIRA_URL") // Retrieve JIRA_URL once
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

// --- Parameter Parsing & Validation ---
startDateStr := r.URL.Query().Get("start_date")
endDateStr := r.URL.Query().Get("end_date")
search := r.URL.Query().Get("search") // Optional search term
pageStr := r.URL.Query().Get("page")
pageSizeStr := r.URL.Query().Get("page_size")
jiraStatusStr := r.URL.Query().Get("jira_status") // with_jira, without_jira, all

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
if err != nil || pageSize < 1 || pageSize > 100 {
http.Error(w, "Invalid page_size. Must be an integer between 1 and 100.", http.StatusBadRequest)
return
}
}

jiraStatus := All
if jiraStatusStr != "" {
switch JiraReferenceStatus(jiraStatusStr) {
case WithJira, WithoutJira, All:
jiraStatus = JiraReferenceStatus(jiraStatusStr)
default:
http.Error(w, "Invalid jira_status. Must be 'with_jira', 'without_jira', or 'all'.", http.StatusBadRequest)
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

var pullRequests []PullRequestJiraAPI
var totalCount int32

// Get counts for both types
withJiraCount, err := db.CountPullRequestsWithJiraReferences(ctx, sqlc.CountPullRequestsWithJiraReferencesParams{
StartDate:      startDate,
EndDate:        endDate,
TextSearchTerm: search,
TeamName:       "",
Members:        nil,
})
if err != nil {
logger.Error("Failed to count PRs with JIRA references", "error", err)
http.Error(w, "Internal server error", http.StatusInternalServerError)
return
}

withoutJiraCount, err := db.CountPullRequestsWithoutJiraReferences(ctx, sqlc.CountPullRequestsWithoutJiraReferencesParams{
StartDate:      startDate,
EndDate:        endDate,
TextSearchTerm: search,
TeamName:       "",
Members:        nil,
})
if err != nil {
logger.Error("Failed to count PRs without JIRA references", "error", err)
http.Error(w, "Internal server error", http.StatusInternalServerError)
return
}

totalPRs := withJiraCount + withoutJiraCount
jiraPercent := 0.0
if totalPRs > 0 {
jiraPercent = float64(withJiraCount) / float64(totalPRs) * 100.0
}

// Fetch the appropriate list based on the jira_status parameter
switch jiraStatus {
case WithJira:
prsWithJira, err := db.ListPullRequestsWithJiraReferences(ctx, sqlc.ListPullRequestsWithJiraReferencesParams{
StartDate:      startDate,
EndDate:        endDate,
TextSearchTerm: search,
TeamName:       "",
Members:        nil,
PageSize:       int32(safePageSize), // #nosec G115
OffsetVal:      int32(safeOffset),   // #nosec G115
})
if err != nil {
logger.Error("Failed to list PRs with JIRA references", "error", err)
http.Error(w, "Internal server error", http.StatusInternalServerError)
return
}

for _, pr := range prsWithJira {
jiraRefs := make([]string, 0)
// Handle the JiraReferences which is an interface{} type
if pr.JiraReferences != nil {
		if refs, ok := pr.JiraReferences.([]interface{}); ok {
			for _, item := range refs {
				if refStr, ok := item.(string); ok {
					if jiraURL != "" {
						jiraLink := jiraURL + "/browse/" + refStr
						jiraRefs = append(jiraRefs, jiraLink)
					} else {
						jiraRefs = append(jiraRefs, refStr) // Fallback if JIRA_URL is not set
					}
				}
			}
		} else if ref, ok := pr.JiraReferences.(string); ok && ref != "" {
			if jiraURL != "" {
				jiraLink := jiraURL + "/browse/" + ref
				jiraRefs = append(jiraRefs, jiraLink)
			} else {
				jiraRefs = append(jiraRefs, ref) // Fallback if JIRA_URL is not set
			}
		}
	}

var mergedAtPtr *time.Time
if pr.MergedAt.Valid {
mergedAtPtr = &pr.MergedAt.Time
}

pullRequests = append(pullRequests, PullRequestJiraAPI{
PullRequestAPI: PullRequestAPI{
ID:        pr.ID,
RepoName:  pr.RepositoryName.String,
Title:     pr.Title.String,
Author:    pr.Author.String,
State:     pr.State.String,
CreatedAt: pr.CreatedAt,
MergedAt:  mergedAtPtr,
URL:       pr.Url.String,
},
JiraReferences:   jiraRefs,
HasJiraReference: true,
})
}
totalCount = int32(withJiraCount)

case WithoutJira:
prsWithoutJira, err := db.ListPullRequestsWithoutJiraReferencesWithPagination(ctx, sqlc.ListPullRequestsWithoutJiraReferencesParamsWithPagination{
StartDate:      startDate,
EndDate:        endDate,
TextSearchTerm: search,
TeamName:       "",
Members:        nil,
PageSize:       int32(safePageSize), // #nosec G115
OffsetVal:      int32(safeOffset),   // #nosec G115
})
if err != nil {
logger.Error("Failed to list PRs without JIRA references", "error", err)
http.Error(w, "Internal server error", http.StatusInternalServerError)
return
}

for _, pr := range prsWithoutJira {
var mergedAtPtr *time.Time
if pr.MergedAt.Valid {
mergedAtPtr = &pr.MergedAt.Time
}

pullRequests = append(pullRequests, PullRequestJiraAPI{
PullRequestAPI: PullRequestAPI{
ID:        pr.ID,
RepoName:  pr.RepositoryName.String,
Title:     pr.Title.String,
Author:    pr.Author.String,
State:     pr.State.String,
CreatedAt: pr.CreatedAt,
MergedAt:  mergedAtPtr,
URL:       pr.Url.String,
},
JiraReferences:   []string{},
HasJiraReference: false,
})
}
totalCount = int32(withoutJiraCount)

default: // All
    totalCount = int32(withJiraCount + withoutJiraCount)

    listParams := sqlc.ListPullRequestsParams{
        StartDate:  startDate,
        EndDate:    endDate,
        SearchTerm: search,
        TeamName:   "",  // Not a filter in this specific endpoint variant
        Members:    nil, // Not a filter in this specific endpoint variant
        PageSize:   int32(safePageSize),
        OffsetVal:  int32(safeOffset),
    }
    logger.Debug("Fetching all pull requests for JIRA references 'all' mode", "params", listParams)

    allPrsFromDB, err := db.ListPullRequests(ctx, listParams)
    if err != nil {
        logger.Error("Failed to list all PRs for JIRA references 'all' mode", "error", err)
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }

    for _, prFromDB := range allPrsFromDB { // prFromDB is sqlc.ListPullRequestsRow
        var mergedAtPtr *time.Time
        if prFromDB.PrMergedAt.Valid {
            mergedAtPtr = &prFromDB.PrMergedAt.Time
        }

        var currentJiraRefs []string // No JIRA info from ListPullRequests
        var hasJiraRef bool = false    // No JIRA info from ListPullRequests

        apiPR := PullRequestAPI{
            ID:        prFromDB.ID,
            RepoName:  prFromDB.RepositoryName.String,
            Title:     prFromDB.Title.String,
            Author:    prFromDB.Author.String,
            State:     prFromDB.State.String,
            CreatedAt: prFromDB.PrCreatedAt,
            MergedAt:  mergedAtPtr,
            URL:       prFromDB.Url.String,
        }

        if prFromDB.Additions.Valid {
            apiPR.Additions = prFromDB.Additions.Int32
        }
        if prFromDB.Deletions.Valid {
            apiPR.Deletions = prFromDB.Deletions.Int32
        }

        if prFromDB.LeadTimeToCodeSeconds != nil {
            if pgNum, ok := prFromDB.LeadTimeToCodeSeconds.(pgtype.Numeric); ok {
                if pgNum.Valid {
                    pgF8, errConv := pgNum.Float64Value()
                    if errConv == nil && pgF8.Valid {
                        valInt := int64(pgF8.Float64)
                        apiPR.LeadTimeToCodeSeconds = &valInt
                    } else if errConv != nil {
                        logger.Warn("Error converting LeadTimeToCodeSeconds value", "original_value", prFromDB.LeadTimeToCodeSeconds, "conversion_error", errConv)
                    }
                }
            } else {
                logger.Warn("LeadTimeToCodeSeconds is not pgtype.Numeric as expected", "value", prFromDB.LeadTimeToCodeSeconds)
            }
        }
        if prFromDB.LeadTimeToReviewSeconds != nil {
            if pgNum, ok := prFromDB.LeadTimeToReviewSeconds.(pgtype.Numeric); ok {
                if pgNum.Valid {
                    pgF8, errConv := pgNum.Float64Value()
                    if errConv == nil && pgF8.Valid {
                        valInt := int64(pgF8.Float64)
                        apiPR.LeadTimeToReviewSeconds = &valInt
                    } else if errConv != nil {
                        logger.Warn("Error converting LeadTimeToReviewSeconds value", "original_value", prFromDB.LeadTimeToReviewSeconds, "conversion_error", errConv)
                    }
                }
            } else {
                logger.Warn("LeadTimeToReviewSeconds is not pgtype.Numeric as expected", "value", prFromDB.LeadTimeToReviewSeconds)
            }
        }
        if prFromDB.LeadTimeToMergeSeconds != nil {
            if pgNum, ok := prFromDB.LeadTimeToMergeSeconds.(pgtype.Numeric); ok {
                if pgNum.Valid {
                    pgF8, errConv := pgNum.Float64Value()
                    if errConv == nil && pgF8.Valid {
                        valInt := int64(pgF8.Float64)
                        apiPR.LeadTimeToMergeSeconds = &valInt
                    } else if errConv != nil {
                        logger.Warn("Error converting LeadTimeToMergeSeconds value", "original_value", prFromDB.LeadTimeToMergeSeconds, "conversion_error", errConv)
                    }
                }
            } else {
                logger.Warn("LeadTimeToMergeSeconds is not pgtype.Numeric as expected", "value", prFromDB.LeadTimeToMergeSeconds)
            }
        }

        if prFromDB.PrReviewsRequestedCount.Valid {
            val := prFromDB.PrReviewsRequestedCount.Int32
            apiPR.PrReviewsRequestedCount = &val
        }

        pullRequests = append(pullRequests, PullRequestJiraAPI{
            PullRequestAPI:   apiPR,
            JiraReferences:   currentJiraRefs,
            HasJiraReference: hasJiraRef,
        })
    }
}

response := PullRequestJiraResponseAPI{
PullRequests:         pullRequests,
TotalCount:           totalCount,
Page:                 page,
PageSize:             pageSize,
WithJiraCount:        withJiraCount,
WithoutJiraCount:     withoutJiraCount,
JiraReferencePercent: jiraPercent,
}

w.Header().Set("Content-Type", "application/json")
if err := json.NewEncoder(w).Encode(response); err != nil {
logger.Error("Failed to encode response", "error", err)
// Response already started, can't send http.Error
}
}
}
