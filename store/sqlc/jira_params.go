// Package sqlc contains generated code and additions for database interactions
package sqlc

import (
	"context"
	"time"
)

// CountPullRequestsWithoutJiraReferencesParams matches the parameters in the query.sql file
type CountPullRequestsWithoutJiraReferencesParams struct {
	StartDate      time.Time `db:"start_date"`
	EndDate        time.Time `db:"end_date"`
	TextSearchTerm string    `db:"text_search_term"`
	TeamName       string    `db:"team_name"`
	Members        []string  `db:"members"`
}

// ListPullRequestsWithoutJiraReferencesParamsWithPagination extends the original params with pagination
type ListPullRequestsWithoutJiraReferencesParamsWithPagination struct {
	StartDate      time.Time `db:"start_date"`
	EndDate        time.Time `db:"end_date"`
	TextSearchTerm string    `db:"text_search_term"`
	TeamName       string    `db:"team_name"`
	Members        []string  `db:"members"`
	PageSize       int32     `db:"page_size"`
	OffsetVal      int32     `db:"offset_val"`
}

// CountPullRequestsWithoutJiraReferences counts pull requests without JIRA references
func (q *Queries) CountPullRequestsWithoutJiraReferences(ctx context.Context, arg CountPullRequestsWithoutJiraReferencesParams) (int64, error) {
	const query = `SELECT COUNT(DISTINCT p.id)
	FROM prs p
	LEFT JOIN teams t ON p.author = t.member
	WHERE
		(COALESCE(p.title, '') !~ '[A-Z]+-[0-9]+' AND COALESCE(p.branch_name, '') !~ '[A-Z]+-[0-9]+')
		AND p.created_at >= $1::timestamptz
		AND p.created_at <= $2::timestamptz
		AND ($3::text = '' OR
			 p.title ILIKE '%' || $3::text || '%' OR
			 p.branch_name ILIKE '%' || $3::text || '%' OR
			 p.author ILIKE '%' || $3::text || '%')
		AND ($4::text = '' OR t.team = $4::text)
		AND ($5::text[] IS NULL OR p.author = ANY($5::text[]))`

	row := q.db.QueryRow(ctx, query,
		arg.StartDate,
		arg.EndDate,
		arg.TextSearchTerm,
		arg.TeamName,
		arg.Members,
	)
	var count int64
	err := row.Scan(&count)
	return count, err
}

// ListPullRequestsWithoutJiraReferencesWithPagination lists pull requests without JIRA references with pagination
func (q *Queries) ListPullRequestsWithoutJiraReferencesWithPagination(ctx context.Context, arg ListPullRequestsWithoutJiraReferencesParamsWithPagination) ([]ListPullRequestsWithoutJiraReferencesRow, error) {
	const query = `SELECT
		p.id,
		p.title,
		p.branch_name,
		p.url,
		p.author,
		p.state,
		p.created_at,
		p.merged_at,
		p.repository_name,
		p.repository_owner
	FROM prs p
	LEFT JOIN teams t ON p.author = t.member
	WHERE
		(COALESCE(p.title, '') !~ '[A-Z]+-[0-9]+' AND COALESCE(p.branch_name, '') !~ '[A-Z]+-[0-9]+')
		AND p.created_at >= $1::timestamptz
		AND p.created_at <= $2::timestamptz
		AND ($3::text = '' OR
			 p.title ILIKE '%' || $3::text || '%' OR
			 p.branch_name ILIKE '%' || $3::text || '%' OR
			 p.author ILIKE '%' || $3::text || '%')
		AND ($4::text = '' OR t.team = $4::text)
		AND ($5::text[] IS NULL OR p.author = ANY($5::text[]))
	ORDER BY p.created_at DESC
	LIMIT $6::int OFFSET $7::int`

	rows, err := q.db.Query(ctx, query,
		arg.StartDate,
		arg.EndDate,
		arg.TextSearchTerm,
		arg.TeamName,
		arg.Members,
		arg.PageSize,
		arg.OffsetVal,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListPullRequestsWithoutJiraReferencesRow{}
	for rows.Next() {
		var i ListPullRequestsWithoutJiraReferencesRow
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.BranchName,
			&i.Url,
			&i.Author,
			&i.State,
			&i.CreatedAt,
			&i.MergedAt,
			&i.RepositoryName,
			&i.RepositoryOwner,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
