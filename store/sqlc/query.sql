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
