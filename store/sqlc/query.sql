-- Repositories --

-- name: CountRepositories :one
SELECT count(*) FROM repositories
WHERE ($1::text = '' OR slug ILIKE '%' || $1 || '%'); -- Use ILIKE for case-insensitive search, handle empty search string

-- name: ListRepositories :many
SELECT org, slug, language FROM repositories
WHERE ($1::text = '' OR slug ILIKE '%' || $1 || '%')
LIMIT $2 OFFSET $3;

-- name: TruncateRepositories :exec
TRUNCATE TABLE repositories;

-- name: CreateRepository :exec
INSERT INTO repositories (org, slug, language)
VALUES ($1, $2, $3);

-- name: GetAllRepositories :many
SELECT org, slug, language FROM repositories;


-- Pull Requests (prs) --

-- name: GetLastPullRequestMergedDate :one
SELECT merged_at FROM prs
WHERE state = $1 AND repository_owner = $2 AND repository_name = $3
ORDER BY merged_at DESC
LIMIT 1;

-- name: UpsertPullRequest :exec
INSERT INTO prs (
    id, title, state, url, merged_at, created_at, additions, deletions,
    branch_name, author, repository_name, repository_owner,
    review_requested_at, reviews_requested
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
) ON CONFLICT (id)
DO UPDATE SET
    title = EXCLUDED.title,
    state = EXCLUDED.state,
    merged_at = EXCLUDED.merged_at,
    additions = EXCLUDED.additions,
    deletions = EXCLUDED.deletions,
    review_requested_at = EXCLUDED.review_requested_at,
    reviews_requested = EXCLUDED.reviews_requested;


-- Commits --

-- name: InsertCommit :exec
INSERT INTO commits (id, pr_id, message, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO NOTHING;


-- Pull Request Reviews --

-- name: UpsertPullRequestReview :exec
INSERT INTO pull_request_reviews (
    id, pull_request_id, author_login, state, body, url, submitted_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) ON CONFLICT (id)
DO UPDATE SET
    state = EXCLUDED.state,
    body = EXCLUDED.body,
    submitted_at = EXCLUDED.submitted_at,
    updated_at = CURRENT_TIMESTAMP;


-- Teams --

-- name: TruncateTeams :exec
TRUNCATE TABLE teams;

-- name: CreateTeamMember :exec
INSERT INTO teams (team, member, avatar_url)
VALUES ($1, $2, $3);

-- name: FetchSecurityPullRequests :many
SELECT
    p.id, p.url, p.title, p.repository_name, p.repository_owner, p.author,
    p.additions, p.deletions, p.state, p.created_at, p.merged_at
FROM teams t
INNER JOIN prs p ON p.author = t.member
WHERE
    (p.created_at >= date_trunc('day', current_timestamp) - interval '1 day' AND p.state = 'OPEN')
    OR
    (p.merged_at >= date_trunc('day', current_timestamp) - interval '1 day' AND p.state = 'MERGED')
AND t.team IN (
    'pe-customer-journey', 'PE Platform Insights', 'Webstack', 'Omnibus', 'CSI',
    'pe-platform-fleet', 'ie-deploy', 'P&E - Team Domino', 'Ares', 'RD-Edge',
    'Golden', 'RD - Production Engineering', 'Security Engineering'
)
GROUP BY p.id
ORDER BY p.additions + p.deletions DESC;

-- name: SearchDistinctTeamNamesByPrefix :many
SELECT DISTINCT team
FROM teams
WHERE team ILIKE $1 || '%' -- Case-insensitive prefix search
ORDER BY team;


-- Team Statistics --

-- name: CountTeamCommitsByDateRange :one
SELECT COUNT(c.id)::int -- Cast to int for Go compatibility
FROM commits c
JOIN prs p ON c.pr_id = p.id
JOIN teams t ON p.author = t.member
WHERE t.team = $1
  AND c.created_at >= $2 -- pgtype.Timestamptz
  AND c.created_at <= $3; -- pgtype.Timestamptz

-- name: GetTeamPullRequestStatsByDateRange :one
SELECT
    COUNT(CASE WHEN p.state = 'OPEN' AND p.created_at >= $2 AND p.created_at <= $3 THEN 1 END)::int AS open_count,
    COUNT(CASE WHEN p.state = 'MERGED' AND p.merged_at >= $2 AND p.merged_at <= $3 THEN 1 END)::int AS merged_count,
    -- Assuming 'CLOSED' is a state and filtering by created_at. Adjust if logic differs.
    COUNT(CASE WHEN p.state = 'CLOSED' AND p.created_at >= $2 AND p.created_at <= $3 THEN 1 END)::int AS closed_count,
    -- Count rollbacks: Merged PRs within the date range whose title starts with 'Revert '
    COUNT(CASE WHEN p.state = 'MERGED' AND p.merged_at >= $2 AND p.merged_at <= $3 AND p.title LIKE 'Revert %' THEN 1 END)::int AS rollbacks_count
FROM prs p
JOIN teams t ON p.author = t.member
WHERE t.team = $1
  AND (
       (p.state = 'OPEN' AND p.created_at >= $2 AND p.created_at <= $3) OR
       (p.state = 'MERGED' AND p.merged_at >= $2 AND p.merged_at <= $3) OR
       (p.state = 'CLOSED' AND p.created_at >= $2 AND p.created_at <= $3)
      );


-- List Pull Requests (Paginated & Searchable) --

-- name: ListPullRequests :many
SELECT
    id,
    repository_name,
    title,
    author,
    state,
    created_at,
    merged_at,
    -- closed_at column does not exist, state indicates closure
    url
FROM prs
WHERE
    created_at >= sqlc.arg(start_date)::timestamptz
    AND created_at <= sqlc.arg(end_date)::timestamptz
    AND (
        sqlc.arg(search_term)::text = '' OR
        title ILIKE '%' || sqlc.arg(search_term)::text || '%' OR
        author ILIKE '%' || sqlc.arg(search_term)::text || '%'
    )
ORDER BY created_at DESC -- Or another relevant field like id
LIMIT sqlc.arg(page_size)::int
OFFSET sqlc.arg(offset_val)::int;

-- name: CountPullRequests :one
SELECT COUNT(*)::int
FROM prs
WHERE
    created_at >= sqlc.arg(start_date)::timestamptz
    AND created_at <= sqlc.arg(end_date)::timestamptz
    AND (
        sqlc.arg(search_term)::text = '' OR
        title ILIKE '%' || sqlc.arg(search_term)::text || '%' OR
        author ILIKE '%' || sqlc.arg(search_term)::text || '%'
    );
