package store

import (
	"context"
	"database/sql" // Re-add for sql.NullString
	"fmt"
	"log/slog"
	"os"

	// "slices" // No longer needed directly in this file after refactor
	"time"

	"github.com/akawula/DoraMatic/github/organizations" // Import organizations package
	"github.com/akawula/DoraMatic/github/pullrequests"
	"github.com/akawula/DoraMatic/github/repositories"
	"github.com/akawula/DoraMatic/store/sqlc" // Import the generated package
	"github.com/jackc/pgx/v5"                 // Needed for pgx.ErrNoRows
	"github.com/jackc/pgx/v5/pgtype"          // Import pgtype for timestamptz handling
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBRepository type is replaced by sqlc.Repository
// type DBRepository struct {
// 	Org      string
// 	Slug     string
// 	Language string
// }

// Count struct is replaced by sqlc's direct return value (int64) for CountRepositories
// type Count struct {
// 	Total int
// }

// SecurityPR struct definition removed, assume it's in base.go

type Postgres struct {
	connPool *pgxpool.Pool // Use pgx connection pool
	queries  *sqlc.Queries // Use generated queries
	Logger   *slog.Logger
}

// NewPostgres creates a new Postgres store instance using pgx and sqlc.
// NOTE: Changed signature to accept context.Context for pgxpool.New
func NewPostgres(ctx context.Context, logger *slog.Logger) Store {
	connectionString := fmt.Sprintf("user=%s dbname=%s sslmode=disable password=%s host=%s port=%s", os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_DB"), os.Getenv("POSTGRES_PASSWORD"), os.Getenv("POSTGRES_SERVICE_HOST"), os.Getenv("POSTGRES_SERVICE_PORT"))

	// Use pgxpool for connection pooling
	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		logger.Error("Unable to connect to database", "error", err)
		// In a real app, you might want to handle this more gracefully, maybe os.Exit(1) or return error
		panic(err) // Or return an error and handle it in the caller
	}

	// Create sqlc Queries instance
	queries := sqlc.New(pool)

	return &Postgres{connPool: pool, queries: queries, Logger: logger}
}

// Close closes the database connection pool.
func (p *Postgres) Close() {
	p.connPool.Close()
}

// getTotal is no longer needed as CountRepositories query handles it.
// func (p *Postgres) getTotal(q string) int { ... }

// GetRepos retrieves repositories using generated sqlc code.
// NOTE: Return type uses the generated sqlc.Repository struct.
func (p *Postgres) GetRepos(ctx context.Context, page int, search string) ([]sqlc.Repository, int, error) {
	limit := int32(20)                                 // sqlc uses specific int types
	offset := int32(calculateOffset(page, int(limit))) // Assumes calculateOffset is accessible

	p.Logger.Debug("Fetching repositories", "search", search, "limit", limit, "offset", offset)

	// Get total count using sqlc query
	total, err := p.queries.CountRepositories(ctx, search) // Pass search term directly
	if err != nil {
		p.Logger.Error("can't count repositories", "error", err)
		return nil, 0, err
	}

	// Fetch the repositories for the current page using sqlc query
	// ListRepositoriesParams expects Column1 (search) as string
	repos, err := p.queries.ListRepositories(ctx, sqlc.ListRepositoriesParams{
		Column1: search,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		p.Logger.Error("can't fetch repositories", "error", err)
		return nil, 0, err
	}

	// Note: The return type is now []sqlc.ListRepositoriesRow.
	// Adjust downstream code or map the results if necessary.
	return repos, int(total), nil
}

// SaveRepos saves repositories using generated sqlc code within a transaction.
// NOTE: Changed signature to accept context.Context.
func (p *Postgres) SaveRepos(ctx context.Context, repos []repositories.Repository) error {
	tx, err := p.connPool.Begin(ctx)
	if err != nil {
		p.Logger.Error("Failed to begin transaction for saving repos", "error", err)
		return err
	}
	defer tx.Rollback(ctx) // Rollback if anything goes wrong

	qtx := p.queries.WithTx(tx) // Use queries within the transaction

	// Truncate the table first
	err = qtx.TruncateRepositories(ctx)
	if err != nil {
		p.Logger.Error("Failed to truncate repositories", "error", err)
		return err
	}

	// Insert new repositories
	for _, repo := range repos {
		// Safely handle potential empty string for language
		var lang pgtype.Text // CreateRepositoryParams expects pgtype.Text
		// Correct check for struct field's value
		if repo.PrimaryLanguage.Name != "" {
			lang = pgtype.Text{String: string(repo.PrimaryLanguage.Name), Valid: true}
		}
		err = qtx.CreateRepository(ctx, sqlc.CreateRepositoryParams{
			Org:      string(repo.Owner.Login), // Cast githubv4.String
			Slug:     string(repo.Name),        // Cast githubv4.String
			Language: lang,                     // Pass pgtype.Text
		})
		if err != nil {
			p.Logger.Error("Failed to insert repository", "repo", repo.Name, "error", err)
			return err // Rollback happens automatically due to defer tx.Rollback
		} // Closing brace for the if err != nil block
	} // Closing brace for the loop

	// Commit the transaction if all inserts were successful
	return tx.Commit(ctx)
}

// GetLastPRDate retrieves the last merged date for a PR using generated sqlc code.
// NOTE: Changed signature to accept context.Context. Return type is time.Time as before.
func (p *Postgres) GetLastPRDate(ctx context.Context, org string, repo string) time.Time {
	defaultTime := time.Now().AddDate(-2, 0, 0) // Default to -2 years

	// Use sqlc query. Parameters seem to expect strings based on query.sql ($1, $2, $3)
	// Result type is pgtype.Timestamptz. Params struct expects pgtype.Text.
	mergedAtResult, err := p.queries.GetLastPullRequestMergedDate(ctx, sqlc.GetLastPullRequestMergedDateParams{
		State:           pgtype.Text{String: "MERGED", Valid: true},
		RepositoryOwner: pgtype.Text{String: org, Valid: true},
		RepositoryName:  pgtype.Text{String: repo, Valid: true},
	})

	if err != nil {
		// Log the error but return the default time as per original logic
		if err == pgx.ErrNoRows {
			p.Logger.Info("No merged PRs found for repository, using default date", "repo", repo, "org", org)
		} else {
			p.Logger.Error("Can't fetch last date of pr", "error", err, "repo", repo, "org", org)
		}
		return defaultTime
	}

	// Check if the pgtype.Timestamptz result is valid
	if mergedAtResult.Valid {
		return mergedAtResult.Time
	}

	// If merged_at is NULL in the database for the latest merged PR (unlikely but possible)
	p.Logger.Warn("Latest merged PR has NULL merged_at date", "repo", repo, "org", org)
	return defaultTime
}

// SavePullRequest saves pull requests and their associated commits/reviews using sqlc within a transaction.
// NOTE: Changed signature to accept context.Context.
func (p *Postgres) SavePullRequest(ctx context.Context, prs []pullrequests.PullRequest) (err error) {
	if len(prs) == 0 {
		p.Logger.Info("Pull Requests slice is empty, nothing to save.")
		return nil
	}

	tx, err := p.connPool.Begin(ctx)
	if err != nil {
		p.Logger.Error("Failed to begin transaction for saving PRs", "error", err)
		return err
	}
	defer tx.Rollback(ctx) // Ensure rollback on error

	qtx := p.queries.WithTx(tx)

	for _, pr := range prs {
		// Prepare parameters for sqlc UpsertPullRequest
		var reviewAt pgtype.Timestamptz
		if len(pr.TimelineItems.Nodes) > 0 && pr.TimelineItems.Nodes[0].ReviewRequestedEventFragment.CreatedAt != "" {
			t, parseErr := time.Parse(time.RFC3339, string(pr.TimelineItems.Nodes[0].ReviewRequestedEventFragment.CreatedAt))
			if parseErr == nil {
				reviewAt = pgtype.Timestamptz{Time: t, Valid: true}
			} else {
				p.Logger.Warn("Failed to parse review_requested_at", "value", pr.TimelineItems.Nodes[0].ReviewRequestedEventFragment.CreatedAt, "error", parseErr)
			}
		}

		var mergedAt pgtype.Timestamptz
		if pr.MergedAt != "" {
			t, parseErr := time.Parse(time.RFC3339, string(pr.MergedAt))
			if parseErr == nil {
				mergedAt = pgtype.Timestamptz{Time: t, Valid: true}
			} else {
				p.Logger.Warn("Failed to parse merged_at", "value", pr.MergedAt, "error", parseErr)
			}
		}

		createdAt, parseErr := time.Parse(time.RFC3339, string(pr.CreatedAt)) // Cast githubv4.String
		if parseErr != nil {
			p.Logger.Error("Failed to parse created_at, skipping PR", "pr_id", string(pr.Id), "value", string(pr.CreatedAt), "error", parseErr) // Cast Id and CreatedAt for logging
			continue                                                                                                                            // Skip this PR if created_at is invalid
		}
		pgCreatedAt := pgtype.Timestamptz{Time: createdAt, Valid: true} // Assuming CreatedAt is non-nullable

		// Revert Timestamptz vars to sql.NullTime if that's what sqlc generates for nullable timestamps
		// Or keep pgtype if that's accurate after regeneration check (let's keep pgtype for now)
		// Match the exact types from the generated UpsertPullRequestParams struct
		params := sqlc.UpsertPullRequestParams{
			ID:                string(pr.Id),                                                                                  // Cast githubv4.String
			Title:             sql.NullString{String: string(pr.Title), Valid: pr.Title != ""},                                // sql.NullString
			State:             pgtype.Text{String: string(pr.State), Valid: pr.State != ""},                                   // pgtype.Text
			Url:               sql.NullString{String: string(pr.Url), Valid: pr.Url != ""},                                    // sql.NullString
			MergedAt:          mergedAt,                                                                                       // pgtype.Timestamptz
			CreatedAt:         pgCreatedAt.Time,                                                                               // time.Time
			Additions:         pgtype.Int4{Int32: int32(pr.Additions), Valid: true},                                           // pgtype.Int4
			Deletions:         pgtype.Int4{Int32: int32(pr.Deletions), Valid: true},                                           // pgtype.Int4
			BranchName:        sql.NullString{String: string(pr.HeadRefName), Valid: pr.HeadRefName != ""},                    // sql.NullString
			Author:            pgtype.Text{String: string(pr.Author.Login), Valid: pr.Author.Login != ""},                     // pgtype.Text
			RepositoryName:    pgtype.Text{String: string(pr.Repository.Name), Valid: pr.Repository.Name != ""},               // pgtype.Text
			RepositoryOwner:   pgtype.Text{String: string(pr.Repository.Owner.Login), Valid: pr.Repository.Owner.Login != ""}, // pgtype.Text
			ReviewsRequested:  pgtype.Int4{Int32: int32(pr.TimelineItems.TotalCount), Valid: true},                            // pgtype.Int4
			ReviewRequestedAt: reviewAt,                                                                                       // pgtype.Timestamptz
		}

		err = qtx.UpsertPullRequest(ctx, params)
		if err != nil {
			p.Logger.Error("Failed to upsert pull request", "pr_id", string(pr.Id), "error", err) // Cast pr.Id
			return err                                                                            // Rollback triggered by defer
		}

		// Save commits within the same transaction
		if len(pr.Commits.Nodes) > 0 {
			commitErr := p.saveCommitsInTx(ctx, qtx, string(pr.Id), pr.Commits.Nodes) // Cast pr.Id
			if commitErr != nil {
				// Error already logged in helper
				return commitErr // Rollback
			}
		}

		// Save reviews within the same transaction
		if len(pr.Reviews.Nodes) > 0 {
			reviewErr := p.savePullRequestReviewsInTx(ctx, qtx, string(pr.Id), pr.Reviews.Nodes) // Cast pr.Id
			if reviewErr != nil {
				// Error already logged in helper
				return reviewErr // Rollback
			}
		}
	}

	return tx.Commit(ctx) // Commit transaction if all steps succeeded
}

// savePullRequestReviewsInTx saves reviews within an existing transaction using sqlc.
// Note: This removes the duplicate SavePullRequest function above.
func (p *Postgres) savePullRequestReviewsInTx(ctx context.Context, qtx *sqlc.Queries, pullRequestID string, reviews []pullrequests.Review) error {
	for _, review := range reviews {
		var submittedAt pgtype.Timestamptz
		if review.SubmittedAt != "" {
			t, parseErr := time.Parse(time.RFC3339, string(review.SubmittedAt))
			if parseErr == nil {
				submittedAt = pgtype.Timestamptz{Time: t, Valid: true}
			} else {
				p.Logger.Warn("Failed to parse review submitted_at", "review_id", review.Id, "value", review.SubmittedAt, "error", parseErr)
			}
		}

		// Match the exact types from the generated UpsertPullRequestReviewParams struct
		params := sqlc.UpsertPullRequestReviewParams{
			ID:            string(review.Id), // Cast githubv4.String
			PullRequestID: pullRequestID,
			AuthorLogin:   sql.NullString{String: string(review.Author.Login), Valid: review.Author.Login != ""}, // sql.NullString
			State:         pgtype.Text{String: string(review.State), Valid: review.State != ""},                  // pgtype.Text
			Body:          sql.NullString{String: string(review.Body), Valid: review.Body != ""},                 // sql.NullString
			Url:           sql.NullString{String: string(review.Url), Valid: review.Url != ""},                   // sql.NullString
			SubmittedAt:   submittedAt,                                                                           // pgtype.Timestamptz
		}
		err := qtx.UpsertPullRequestReview(ctx, params)
		if err != nil {
			p.Logger.Error("Failed to upsert pull request review", "review_id", string(review.Id), "pr_id", pullRequestID, "error", err) // Cast review.Id
			return err                                                                                                                   // Propagate error to trigger rollback
		}
	}
	return nil
}

// saveCommitsInTx saves commits within an existing transaction using sqlc.
func (p *Postgres) saveCommitsInTx(ctx context.Context, qtx *sqlc.Queries, prID string, commits []pullrequests.Commit) error {
	for _, commit := range commits {
		// Parse the commit date
		var createdAt pgtype.Timestamptz
		if commit.Commit.CommittedDate != "" {
			t, parseErr := time.Parse(time.RFC3339, string(commit.Commit.CommittedDate))
			if parseErr == nil {
				createdAt = pgtype.Timestamptz{Time: t, Valid: true}
			} else {
				p.Logger.Warn("Failed to parse commit CommittedDate", "commit_id", string(commit.Id), "value", commit.Commit.CommittedDate, "error", parseErr)
				// Decide if we should skip or insert with NULL date? Let's insert with NULL for now.
				createdAt = pgtype.Timestamptz{Valid: false}
			}
		} else {
			createdAt = pgtype.Timestamptz{Valid: false} // Set as invalid if the string is empty
		}

		// Match the exact types from the generated InsertCommitParams struct
		params := sqlc.InsertCommitParams{
			ID:        string(commit.Id), // Cast githubv4.String
			PrID:      prID,
			Message:   sql.NullString{String: string(commit.Commit.Message), Valid: commit.Commit.Message != ""}, // sql.NullString
			CreatedAt: createdAt,                                                                                 // Add the parsed created_at timestamp
		}
		err := qtx.InsertCommit(ctx, params)
		if err != nil {
			p.Logger.Error("Failed to insert commit", "commit_id", string(commit.Id), "pr_id", prID, "error", err) // Cast commit.Id
			return err                                                                                             // Propagate error to trigger rollback
		}
	}
	return nil
}

// GetAllRepos retrieves all repositories using generated sqlc code.
// NOTE: Return type uses the generated sqlc.Repository struct.
func (p *Postgres) GetAllRepos(ctx context.Context) ([]sqlc.Repository, error) {
	repos, err := p.queries.GetAllRepositories(ctx)
	if err != nil {
		p.Logger.Error("can't fetch all repositories", "error", err)
		return nil, err
	}
	return repos, nil
}

// SaveTeams saves team memberships (including avatar URL) using generated sqlc code within a transaction.
// NOTE: Updated signature to accept map[string][]organizations.MemberInfo.
func (p *Postgres) SaveTeams(ctx context.Context, teams map[string][]organizations.MemberInfo) error {
	tx, err := p.connPool.Begin(ctx)
	if err != nil {
		p.Logger.Error("Failed to begin transaction for saving teams", "error", err)
		return err
	}
	defer tx.Rollback(ctx)

	qtx := p.queries.WithTx(tx)

	err = qtx.TruncateTeams(ctx)
	if err != nil {
		p.Logger.Error("Failed to truncate teams", "error", err)
		return err
	}

	for teamName, members := range teams {
		for _, memberInfo := range members { // Iterate over MemberInfo struct
			// Prepare params for CreateTeamMember, including avatar_url
			// Assuming avatar_url is nullable TEXT in DB, use sql.NullString
			avatarURL := sql.NullString{String: memberInfo.AvatarUrl, Valid: memberInfo.AvatarUrl != ""}

			err = qtx.CreateTeamMember(ctx, sqlc.CreateTeamMemberParams{
				Team:      teamName,
				Member:    memberInfo.Login, // Use Login from MemberInfo
				AvatarUrl: avatarURL,        // Pass the avatar URL
			})
			if err != nil {
				p.Logger.Error("Failed to insert team member", "team", teamName, "member", memberInfo.Login, "error", err)
				return err // Rollback triggered by defer
			}
		}
	}

	return tx.Commit(ctx)
}

// FetchSecurityPullRequests retrieves specific PRs using generated sqlc code
// and maps them to the SecurityPR struct to satisfy the Store interface.
// NOTE: Signature matches Store interface now (no context, returns []SecurityPR).
func (p *Postgres) FetchSecurityPullRequests() ([]SecurityPR, error) {
	// Create a background context as the interface doesn't provide one
	ctx := context.Background()

	sqlcPRs, err := p.queries.FetchSecurityPullRequests(ctx)
	if err != nil {
		p.Logger.Error("can't fetch security pull requests using sqlc", "error", err)
		return nil, err
	}
	// Removed duplicate "return nil, err"

	// Map sqlc results to the SecurityPR struct (defined in base.go)
	prs := make([]SecurityPR, 0, len(sqlcPRs))
	for _, sp := range sqlcPRs {
		// Map types correctly, assuming SecurityPR in base.go expects these types
		// Use correct field names (lowercase id, url based on errors)
		// MergedAt is pgtype.Timestamptz in both source and target, assign directly

		secPR := SecurityPR{
			Id:              sp.ID,                             // string
			Url:             sp.Url.String,                     // sql.NullString -> string
			Title:           sp.Title.String,                   // sql.NullString -> string
			RepositoryName:  sp.RepositoryName.String,          // pgtype.Text -> string
			RepositoryOwner: sp.RepositoryOwner.String,         // pgtype.Text -> string
			Author:          sp.Author.String,                  // pgtype.Text -> string
			Additions:       int(sp.Additions.Int32),           // pgtype.Int4 -> int
			Deletions:       int(sp.Deletions.Int32),           // pgtype.Int4 -> int
			State:           sp.State.String,                   // pgtype.Text -> string
			CreatedAt:       sp.CreatedAt.Format(time.RFC3339), // time.Time -> string
			MergedAt:        sp.MergedAt,                       // pgtype.Timestamptz (assign directly)
		}
		// Add Valid checks from source if target fields are non-nullable ints/strings
		// if sp.Additions.Valid { secPR.Additions = int(sp.Additions.Int32) } else { secPR.Additions = 0 } // Example

		prs = append(prs, secPR)
	}
	// Removed duplicated block again (take 2)

	return prs, nil
}

// SearchDistinctTeamNamesByPrefix retrieves a sorted list of unique team names matching the prefix.
func (p *Postgres) SearchDistinctTeamNamesByPrefix(ctx context.Context, prefix string) ([]string, error) {
	// Wrap the prefix in sql.NullString as expected by the generated SQLC function
	sqlPrefix := sql.NullString{String: prefix, Valid: prefix != ""}
	teamNames, err := p.queries.SearchDistinctTeamNamesByPrefix(ctx, sqlPrefix)
	if err != nil {
		p.Logger.Error("Failed to search distinct team names by prefix", "prefix", prefix, "error", err)
		return nil, err
	}
	// The sqlc query returns []string directly.
	return teamNames, nil
}

// CountTeamCommitsByDateRange implements the Store interface method by calling the generated sqlc query.
func (p *Postgres) CountTeamCommitsByDateRange(ctx context.Context, arg sqlc.CountTeamCommitsByDateRangeParams) (int32, error) {
	count, err := p.queries.CountTeamCommitsByDateRange(ctx, arg)
	if err != nil {
		p.Logger.Error("Failed to count team commits by date range", "team", arg.TeamName, "error", err) // Changed arg.Team to arg.TeamName
		return 0, err
	}
	return count, nil
}

// GetTeamMemberReviewStatsByDateRange implements the Store interface method by calling the generated sqlc query.
func (p *Postgres) GetTeamMemberReviewStatsByDateRange(ctx context.Context, arg sqlc.GetTeamMemberReviewStatsByDateRangeParams) ([]sqlc.GetTeamMemberReviewStatsByDateRangeRow, error) {
	rows, err := p.queries.GetTeamMemberReviewStatsByDateRange(ctx, arg)
	if err != nil {
		if err == pgx.ErrNoRows {
			p.Logger.Info("No team member review stats found for date range", "params", arg)
			return []sqlc.GetTeamMemberReviewStatsByDateRangeRow{}, nil // Return empty slice if no rows is not an error
		}
		p.Logger.Error("Failed to get team member review stats by date range", "params", arg, "error", err)
		return nil, err
	}
	return rows, nil
}

// GetPullRequestTimeDataForStats implements the Store interface method by calling the generated sqlc query.
func (p *Postgres) GetPullRequestTimeDataForStats(ctx context.Context, arg sqlc.GetPullRequestTimeDataForStatsParams) ([]sqlc.GetPullRequestTimeDataForStatsRow, error) {
	rows, err := p.queries.GetPullRequestTimeDataForStats(ctx, arg)
	if err != nil {
		if err == pgx.ErrNoRows {
			p.Logger.Info("No PR time data found for stats", "params", arg)
			return []sqlc.GetPullRequestTimeDataForStatsRow{}, nil
		}
		p.Logger.Error("Failed to get PR time data for stats", "params", arg, "error", err)
		return nil, err
	}
	return rows, nil
}

// DiagnoseLeadTimes implements the Store interface method by calling the generated sqlc query.
func (p *Postgres) DiagnoseLeadTimes(ctx context.Context) ([]sqlc.DiagnoseLeadTimesRow, error) {
	results, err := p.queries.DiagnoseLeadTimes(ctx)
	if err != nil {
		if err == pgx.ErrNoRows {
			p.Logger.Info("No data found for lead time diagnosis")
			return []sqlc.DiagnoseLeadTimesRow{}, nil
		}
		p.Logger.Error("Failed to diagnose lead times", "error", err)
		return nil, err
	}
	return results, nil
}

// GetTeamMembers implements the Store interface method by calling the generated sqlc query.
func (p *Postgres) GetTeamMembers(ctx context.Context, team string) ([]sqlc.GetTeamMembersRow, error) {
	members, err := p.queries.GetTeamMembers(ctx, team)
	if err != nil {
		// Handle pgx.ErrNoRows specifically if needed, otherwise log generic error
		if err == pgx.ErrNoRows {
			p.Logger.Info("No members found for team", "team", team)
			// Return empty slice instead of error if no rows is not considered an error state
			return []sqlc.GetTeamMembersRow{}, nil
		}
		p.Logger.Error("Failed to get team members", "team", team, "error", err)
		return nil, err
	}
	return members, nil
}

// GetTeamPullRequestStatsByDateRange implements the Store interface method by calling the generated sqlc query.
func (p *Postgres) GetTeamPullRequestStatsByDateRange(ctx context.Context, arg sqlc.GetTeamPullRequestStatsByDateRangeParams) (sqlc.GetTeamPullRequestStatsByDateRangeRow, error) {
	stats, err := p.queries.GetTeamPullRequestStatsByDateRange(ctx, arg)
	if err != nil {
		// Handle pgx.ErrNoRows specifically if needed, otherwise log generic error
		if err == pgx.ErrNoRows {
			p.Logger.Info("No pull request stats found for team in date range", "team", arg.TeamName) // Changed arg.Team to arg.TeamName
			// Return zero stats instead of error if no rows is not considered an error state
			return sqlc.GetTeamPullRequestStatsByDateRangeRow{}, nil
		}
		p.Logger.Error("Failed to get team pull request stats by date range", "team", arg.TeamName, "error", err) // Changed arg.Team to arg.TeamName
		return sqlc.GetTeamPullRequestStatsByDateRangeRow{}, err
	}
	return stats, nil
}

// ListPullRequests implements the Store interface method by calling the generated sqlc query.
func (p *Postgres) ListPullRequests(ctx context.Context, arg sqlc.ListPullRequestsParams) ([]sqlc.ListPullRequestsRow, error) {
	prs, err := p.queries.ListPullRequests(ctx, arg)
	if err != nil {
		// Handle pgx.ErrNoRows specifically if needed, otherwise log generic error
		if err == pgx.ErrNoRows {
			p.Logger.Info("No pull requests found matching criteria", "params", arg)
			// Return empty slice instead of error if no rows is not considered an error state
			return []sqlc.ListPullRequestsRow{}, nil
		}
		p.Logger.Error("Failed to list pull requests", "params", arg, "error", err)
		return nil, err
	}
	return prs, nil
}

// CountPullRequests implements the Store interface method by calling the generated sqlc query.
func (p *Postgres) CountPullRequests(ctx context.Context, arg sqlc.CountPullRequestsParams) (int32, error) {
	count, err := p.queries.CountPullRequests(ctx, arg)
	if err != nil {
		p.Logger.Error("Failed to count pull requests", "params", arg, "error", err)
		return 0, err
	}
	return count, nil
}
