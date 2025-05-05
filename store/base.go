package store

import (
	"context" // Add context
	// "database/sql" // Remove unused import
	// "fmt" // Remove unused import
	"time"

	"github.com/akawula/DoraMatic/github/organizations" // Import organizations package
	"github.com/akawula/DoraMatic/github/pullrequests"
	"github.com/akawula/DoraMatic/github/repositories"
	"github.com/akawula/DoraMatic/store/sqlc" // Import sqlc package
	"github.com/jackc/pgx/v5/pgtype"          // Import pgtype for SecurityPR.MergedAt
)

// Need to keep SecurityPR definition consistent with its usage in postgres.go mapping
type SecurityPR struct {
	Id              string // Changed from ID to Id based on postgres.go usage
	Url             string // Changed from URL to Url
	Title           string
	RepositoryName  string `db:"repository_name"`
	RepositoryOwner string `db:"repository_owner"`
	Author          string
	Additions       int
	Deletions       int
	State           string
	CreatedAt       string         `db:"created_at"` // Keep as string
	MergedAt        pgtype.Timestamptz `db:"merged_at"` // Changed to pgtype.Timestamptz as used in postgres.go
}

type Store interface {
	Close()
	// Updated signatures to include context and use sqlc types
	GetRepos(ctx context.Context, page int, search string) ([]sqlc.Repository, int, error)
	SaveRepos(ctx context.Context, repos []repositories.Repository) error
	GetAllRepos(ctx context.Context) ([]sqlc.Repository, error) // Added for convenience if needed
	GetLastPRDate(ctx context.Context, org string, repo string) time.Time
	SavePullRequest(ctx context.Context, prs []pullrequests.PullRequest) error
	// Changed SaveTeams parameter type to include MemberInfo
	SaveTeams(ctx context.Context, teams map[string][]organizations.MemberInfo) error
	FetchSecurityPullRequests() ([]SecurityPR, error) // Keep this signature as required
	// Renamed and updated signature for prefix search
	SearchDistinctTeamNamesByPrefix(ctx context.Context, prefix string) ([]string, error)

	// New methods for team statistics
	CountTeamCommitsByDateRange(ctx context.Context, arg sqlc.CountTeamCommitsByDateRangeParams) (int32, error)
	GetTeamPullRequestStatsByDateRange(ctx context.Context, arg sqlc.GetTeamPullRequestStatsByDateRangeParams) (sqlc.GetTeamPullRequestStatsByDateRangeRow, error)

	// New methods for listing pull requests
	ListPullRequests(ctx context.Context, arg sqlc.ListPullRequestsParams) ([]sqlc.ListPullRequestsRow, error)
	CountPullRequests(ctx context.Context, arg sqlc.CountPullRequestsParams) (int32, error)
	// Removed duplicate Close()
}

// getQueryRepos is no longer used by postgres.go after sqlc refactor
// func getQueryRepos(search string) (string, string) {
// 	s := `SELECT org, slug, language `
// 	c := `SELECT count(*) as total `
// 	q := `FROM repositories`
// 	if len(search) > 0 {
// 		q = fmt.Sprintf(`FROM repositories WHERE slug LIKE '%%%s%%'`, search)
// 	}

// 	return s + q + " ORDER by slug, org", c + q
// }
// Keep only one valid calculateOffset function
func calculateOffset(page, limit int) (offset int) {
	offset = (page - 1) * limit

	if page == 1 {
		offset = 0
	}

	return
}
