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

-- name: GetTeamMembers :many
SELECT member, avatar_url
FROM teams
WHERE team = $1
ORDER BY member;


-- Team Statistics --

-- name: CountTeamCommitsByDateRange :one
SELECT COUNT(c.id)::int -- Cast to int for Go compatibility
FROM commits c
JOIN prs p ON c.pr_id = p.id
JOIN teams t ON p.author = t.member
WHERE t.team = sqlc.arg(team_name)
  AND (sqlc.arg(members)::text[] IS NULL OR p.author = ANY(sqlc.arg(members)::text[])) -- Filter by selected members
  AND p.state = 'MERGED'       -- Only consider PRs that are merged
  AND p.merged_at >= sqlc.arg(merged_at_start_date)::timestamptz      -- Filter by PR merge date
  AND p.merged_at <= sqlc.arg(merged_at_end_date)::timestamptz;     -- Filter by PR merge date

-- name: GetTeamPullRequestStatsByDateRange :one
WITH FirstCommitPerPR AS (
    SELECT
        pr_id,
        MIN(created_at) as first_commit_at
    FROM commits
    GROUP BY pr_id
)
SELECT
    COUNT(CASE WHEN p.state = 'MERGED' AND p.merged_at >= sqlc.arg(start_date)::timestamptz AND p.merged_at <= sqlc.arg(end_date)::timestamptz THEN 1 END)::int AS merged_count,
    COUNT(CASE WHEN p.state = 'CLOSED' AND p.created_at >= sqlc.arg(start_date)::timestamptz AND p.created_at <= sqlc.arg(end_date)::timestamptz THEN 1 END)::int AS closed_count,
    COUNT(CASE WHEN p.state = 'MERGED' AND p.merged_at >= sqlc.arg(start_date)::timestamptz AND p.merged_at <= sqlc.arg(end_date)::timestamptz AND p.title LIKE 'Revert %' THEN 1 END)::int AS rollbacks_count,
    -- Calculate average lead time to first review request in seconds
    COALESCE(AVG(
        CASE
            -- Only include PRs that have both timestamps and review requested after first commit
            WHEN p.review_requested_at IS NOT NULL AND fc.first_commit_at IS NOT NULL AND p.review_requested_at > fc.first_commit_at
            THEN EXTRACT(EPOCH FROM (p.review_requested_at - fc.first_commit_at))
            ELSE NULL
        END
    ), 0)::float AS avg_lead_time_to_code_seconds, -- Use COALESCE to return 0 if no valid PRs found, cast to float
    -- Count PRs contributing to the average lead time calculation
    COUNT(
        CASE
            WHEN p.review_requested_at IS NOT NULL AND fc.first_commit_at IS NOT NULL AND p.review_requested_at > fc.first_commit_at
            THEN 1 -- Count this PR
            ELSE NULL
        END
    )::int AS count_prs_for_avg_lead_time,
    -- Calculate average lead time from first review to merge in seconds
    COALESCE(AVG(
        CASE
            -- Only include PRs that are merged, have a first review, and the review happened before merge
            WHEN p.state = 'MERGED' AND p.merged_at IS NOT NULL AND p.review_requested_at IS NOT NULL AND p.review_requested_at < p.merged_at
            THEN EXTRACT(EPOCH FROM (p.merged_at - p.review_requested_at))
            ELSE NULL
        END
    ), 0)::float AS avg_lead_time_to_review_seconds,
    -- Calculate average lead time from first commit to merge in seconds
    COALESCE(AVG(
        CASE
            -- Only include PRs that are merged, have a first commit, and the commit happened before merge
            WHEN p.state = 'MERGED' AND p.merged_at IS NOT NULL AND fc.first_commit_at IS NOT NULL AND fc.first_commit_at < p.merged_at
            THEN EXTRACT(EPOCH FROM (p.merged_at - fc.first_commit_at))
            ELSE NULL
        END
    ), 0)::float AS avg_lead_time_to_merge_seconds,
    -- Count PRs contributing to the average lead time to merge calculation
    COUNT(
        CASE
            WHEN p.state = 'MERGED' AND p.merged_at IS NOT NULL AND fc.first_commit_at IS NOT NULL AND fc.first_commit_at < p.merged_at
            THEN 1 -- Count this PR
            ELSE NULL
        END
    )::int AS count_prs_for_avg_lead_time_to_merge
FROM prs p
JOIN teams t ON p.author = t.member
LEFT JOIN FirstCommitPerPR fc ON p.id = fc.pr_id
-- LEFT JOIN FirstReviewPerPR fr ON p.id = fr.pull_request_id -- Join with first review data -- No longer needed
WHERE t.team = sqlc.arg(team_name)
  AND (sqlc.arg(members)::text[] IS NULL OR p.author = ANY(sqlc.arg(members)::text[])) -- Filter by selected members
  AND (
       (p.state = 'MERGED' AND p.merged_at >= sqlc.arg(start_date)::timestamptz AND p.merged_at <= sqlc.arg(end_date)::timestamptz) OR
       (p.state = 'CLOSED' AND p.created_at >= sqlc.arg(start_date)::timestamptz AND p.created_at <= sqlc.arg(end_date)::timestamptz)
      );


-- List Pull Requests (Paginated & Searchable by Title/Author and optionally Team) --

-- name: ListPullRequests :many
WITH FirstCommitPerPR AS (
    SELECT
        pr_id,
        MIN(created_at) as first_commit_at
    FROM commits
    GROUP BY pr_id
),
PRsWithLeadTimes AS (
    SELECT
        p.id,
        p.repository_name,
        p.title,
        p.author,
        p.state,
        p.created_at,
        p.merged_at,
        p.additions,
        p.deletions,
        p.url,
        p.review_requested_at, -- Needed for calculation
        fc.first_commit_at,    -- From CTE
        CASE
            WHEN p.review_requested_at IS NOT NULL AND fc.first_commit_at IS NOT NULL AND p.review_requested_at > fc.first_commit_at
            THEN (EXTRACT(EPOCH FROM p.review_requested_at) - EXTRACT(EPOCH FROM fc.first_commit_at))
            ELSE 0.0 -- Changed from NULL
        END::DOUBLE PRECISION AS lead_time_to_code_seconds,
        CASE
            WHEN p.merged_at IS NOT NULL AND p.review_requested_at IS NOT NULL AND p.review_requested_at < p.merged_at
            THEN (EXTRACT(EPOCH FROM p.merged_at) - EXTRACT(EPOCH FROM p.review_requested_at))
            ELSE 0.0 -- Changed from NULL
        END::DOUBLE PRECISION AS lead_time_to_review_seconds,
        CASE
            WHEN p.merged_at IS NOT NULL AND fc.first_commit_at IS NOT NULL AND fc.first_commit_at < p.merged_at
            THEN EXTRACT(EPOCH FROM (p.merged_at - fc.first_commit_at))
            ELSE 0.0 -- Changed from NULL
        END::DOUBLE PRECISION AS lead_time_to_merge_seconds
    FROM prs p
    LEFT JOIN FirstCommitPerPR fc ON p.id = fc.pr_id
    -- LEFT JOIN FirstReviewPerPR fr ON p.id = fr.pull_request_id -- No longer needed
)
SELECT
    pr_lt.id,
    pr_lt.repository_name,
    pr_lt.title,
    pr_lt.author,
    pr_lt.state,
    pr_lt.created_at,
    pr_lt.merged_at,
    pr_lt.additions,
    pr_lt.deletions,
    pr_lt.url,
    pr_lt.lead_time_to_code_seconds,
    pr_lt.lead_time_to_review_seconds,
    pr_lt.lead_time_to_merge_seconds
FROM PRsWithLeadTimes pr_lt
LEFT JOIN teams t ON pr_lt.author = t.member -- Join with teams table for filtering
WHERE
    pr_lt.state = 'MERGED' -- Only show merged PRs when filtering by merged_at
    AND pr_lt.merged_at >= sqlc.arg(start_date)::timestamptz
    AND pr_lt.merged_at <= sqlc.arg(end_date)::timestamptz
    AND ( -- Filter by search term (title or author)
        sqlc.arg(search_term)::text = '' OR
        pr_lt.title ILIKE '%' || sqlc.arg(search_term)::text || '%' OR
        pr_lt.author ILIKE '%' || sqlc.arg(search_term)::text || '%'
    )
    AND ( -- Optionally filter by team name
        sqlc.arg(team_name)::text = '' OR
        t.team = sqlc.arg(team_name)::text
    )
    AND ( -- Optionally filter by state
        sqlc.arg(filter_state)::text = '' OR
        pr_lt.state ILIKE '%' || sqlc.arg(filter_state)::text || '%' -- Use ILIKE for state as well for consistency, though direct equals is fine if state is exact
    )
    AND ( -- Optionally filter by author (case-insensitive)
        sqlc.arg(filter_author)::text = '' OR
        pr_lt.author ILIKE '%' || sqlc.arg(filter_author)::text || '%'
    )
    AND (sqlc.arg(members)::text[] IS NULL OR pr_lt.author = ANY(sqlc.arg(members)::text[])) -- Filter by selected members
ORDER BY pr_lt.merged_at DESC -- Default sort by merged_at
LIMIT sqlc.arg(page_size)::int
OFFSET sqlc.arg(offset_val)::int;

-- name: CountPullRequests :one
SELECT COUNT(p.*)::int -- Count distinct PRs
FROM prs p
LEFT JOIN teams t ON p.author = t.member -- Join with teams table
WHERE
    p.state = 'MERGED' -- Only count merged PRs when filtering by merged_at
    AND p.merged_at >= sqlc.arg(start_date)::timestamptz
    AND p.merged_at <= sqlc.arg(end_date)::timestamptz
    AND ( -- Filter by search term (title or author)
        sqlc.arg(search_term)::text = '' OR
        p.title ILIKE '%' || sqlc.arg(search_term)::text || '%' OR
        p.author ILIKE '%' || sqlc.arg(search_term)::text || '%'
    )
    AND ( -- Optionally filter by team name
        sqlc.arg(team_name)::text = '' OR
        t.team = sqlc.arg(team_name)::text
    )
    AND ( -- Optionally filter by state
        sqlc.arg(filter_state)::text = '' OR
        p.state = sqlc.arg(filter_state)::text
    )
    AND ( -- Optionally filter by author (case-insensitive)
        sqlc.arg(filter_author)::text = '' OR
        p.author ILIKE '%' || sqlc.arg(filter_author)::text || '%'
    )
    AND (sqlc.arg(members)::text[] IS NULL OR p.author = ANY(sqlc.arg(members)::text[])); -- Filter by selected members

-- name: DiagnoseLeadTimes :many
WITH FirstCommitPerPR AS (
    SELECT
        pr_id,
        MIN(created_at) as first_commit_at
    FROM commits
    GROUP BY pr_id
)
-- FirstReviewPerPR AS ( -- No longer needed
--     SELECT
--         pull_request_id,
--         MIN(submitted_at) as first_review_at
--     FROM pull_request_reviews
--     WHERE state = 'APPROVED' OR state = 'CHANGES_REQUESTED'
--     GROUP BY pull_request_id
-- ) -- No longer needed
SELECT
    p.id AS pr_id,
    p.created_at AS pr_created_at,
    p.review_requested_at AS pr_review_requested_at, -- This is the field to use
    p.merged_at AS pr_merged_at,
    fc.first_commit_at,
    p.review_requested_at AS first_review_at -- Use p.review_requested_at directly
FROM prs p
LEFT JOIN FirstCommitPerPR fc ON p.id = fc.pr_id
-- LEFT JOIN FirstReviewPerPR fr ON p.id = fr.pull_request_id -- No longer needed
WHERE p.state = 'MERGED'
ORDER BY p.merged_at DESC
LIMIT 10;
