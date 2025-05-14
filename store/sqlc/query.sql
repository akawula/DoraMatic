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
WHERE team ILIKE '%' || $1 || '%' -- Case-insensitive prefix search
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
),
FirstActualReviewPerPR AS (
    SELECT
        pull_request_id,
        MIN(submitted_at) as first_actual_review_at
    FROM pull_request_reviews
    WHERE state = 'APPROVED' OR state = 'CHANGES_REQUESTED' -- Consider only substantive reviews
    GROUP BY pull_request_id
),
TeamMemberReviewStats AS (
    SELECT
        COUNT(prr.id) AS total_team_reviews_submitted_val,
        COUNT(DISTINCT prr.author_login) AS distinct_team_reviewers_count_val
    FROM pull_request_reviews prr
    INNER JOIN teams t_rev ON prr.author_login = t_rev.member
    WHERE t_rev.team = sqlc.arg(team_name)
      AND (sqlc.arg(members)::text[] IS NULL OR prr.author_login = ANY(sqlc.arg(members)::text[]))
      AND prr.submitted_at >= sqlc.arg(start_date)::timestamptz
      AND prr.submitted_at <= sqlc.arg(end_date)::timestamptz
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
    )::int AS count_prs_for_avg_lead_time_to_merge,
    -- Calculate average time to first actual review in seconds
    COALESCE(AVG(
        CASE
            -- Only include PRs that have a first commit and a first actual review, and review happened after commit
            WHEN far.first_actual_review_at IS NOT NULL AND fc.first_commit_at IS NOT NULL AND far.first_actual_review_at > fc.first_commit_at
            THEN EXTRACT(EPOCH FROM (far.first_actual_review_at - fc.first_commit_at))
            ELSE NULL
        END
    ), 0)::float AS avg_time_to_first_actual_review_seconds,
    -- Count PRs contributing to the average time to first actual review
    COUNT(
        CASE
            WHEN far.first_actual_review_at IS NOT NULL AND fc.first_commit_at IS NOT NULL AND far.first_actual_review_at > fc.first_commit_at
            THEN 1
            ELSE NULL
        END
    )::int AS count_prs_for_avg_time_to_first_actual_review,
    COALESCE(SUM(CASE WHEN p.state = 'MERGED' AND p.merged_at >= sqlc.arg(start_date)::timestamptz AND p.merged_at <= sqlc.arg(end_date)::timestamptz THEN p.additions ELSE 0 END), 0)::bigint AS total_additions,
    COALESCE(SUM(CASE WHEN p.state = 'MERGED' AND p.merged_at >= sqlc.arg(start_date)::timestamptz AND p.merged_at <= sqlc.arg(end_date)::timestamptz THEN p.deletions ELSE 0 END), 0)::bigint AS total_deletions,
    COALESCE(MAX(tmrs.total_team_reviews_submitted_val), 0)::bigint AS total_team_reviews_submitted,
    COALESCE(MAX(tmrs.distinct_team_reviewers_count_val), 0)::int AS distinct_team_reviewers_count
FROM prs p
JOIN teams t ON p.author = t.member
LEFT JOIN FirstCommitPerPR fc ON p.id = fc.pr_id
LEFT JOIN FirstActualReviewPerPR far ON p.id = far.pull_request_id -- Join with first actual review data
CROSS JOIN TeamMemberReviewStats tmrs
WHERE t.team = sqlc.arg(team_name)
  AND (sqlc.arg(members)::text[] IS NULL OR p.author = ANY(sqlc.arg(members)::text[])) -- Filter by selected members
  AND (
       (p.state = 'MERGED' AND p.merged_at >= sqlc.arg(start_date)::timestamptz AND p.merged_at <= sqlc.arg(end_date)::timestamptz) OR
       (p.state = 'CLOSED' AND p.created_at >= sqlc.arg(start_date)::timestamptz AND p.created_at <= sqlc.arg(end_date)::timestamptz)
      );

-- name: GetTeamMemberReviewStatsByDateRange :many
SELECT
    prr.author_login,
    COUNT(prr.id) AS total_reviews_submitted,
    SUM(CASE WHEN prr.state = 'APPROVED' THEN 1 ELSE 0 END) AS approved_reviews,
    SUM(CASE WHEN prr.state = 'CHANGES_REQUESTED' THEN 1 ELSE 0 END) AS changes_requested_reviews,
    SUM(CASE WHEN prr.state = 'COMMENTED' THEN 1 ELSE 0 END) AS commented_reviews
FROM
    pull_request_reviews prr
INNER JOIN teams t ON prr.author_login = t.member -- Ensure reviewer is in the specified team
WHERE t.team = sqlc.arg(team_name)
  AND (sqlc.arg(members)::text[] IS NULL OR prr.author_login = ANY(sqlc.arg(members)::text[])) -- Further filter by selected members if provided
  AND prr.submitted_at >= sqlc.arg(start_date)::timestamptz
  AND prr.submitted_at <= sqlc.arg(end_date)::timestamptz
GROUP BY
    prr.author_login
ORDER BY
    total_reviews_submitted DESC;

-- name: GetPullRequestTimeDataForStats :many
WITH FirstCommitPerPR AS (
    SELECT
        pr_id,
        MIN(created_at) as first_commit_at
    FROM commits
    GROUP BY pr_id
),
FirstActualReviewPerPR AS (
    SELECT
        pull_request_id,
        MIN(submitted_at) as first_actual_review_at
    FROM pull_request_reviews
    WHERE state = 'APPROVED' OR state = 'CHANGES_REQUESTED'
    GROUP BY pull_request_id
)
SELECT
    p.id AS pr_id,
    p.created_at AS pr_created_at,
    p.state AS pr_state,
    p.merged_at AS pr_merged_at,
    p.review_requested_at AS pr_review_requested_at,
    p.reviews_requested AS pr_reviews_requested, -- Added for avg reviews requested count
    fc.first_commit_at,
    far.first_actual_review_at
FROM prs p
LEFT JOIN teams t ON p.author = t.member -- Join with teams to filter by team_name
LEFT JOIN FirstCommitPerPR fc ON p.id = fc.pr_id
LEFT JOIN FirstActualReviewPerPR far ON p.id = far.pull_request_id
WHERE
    t.team = sqlc.arg(team_name)
    AND (sqlc.arg(members)::text[] IS NULL OR p.author = ANY(sqlc.arg(members)::text[]))
    AND (
        -- Include PRs relevant for any lead time calculation or stats within the period
        -- Merged PRs within the period
        (p.state = 'MERGED' AND p.merged_at >= sqlc.arg(start_date)::timestamptz AND p.merged_at <= sqlc.arg(end_date)::timestamptz) OR
        -- PRs created within the period (relevant for 'time to code', 'time to first review' even if not merged in period)
        (p.created_at >= sqlc.arg(start_date)::timestamptz AND p.created_at <= sqlc.arg(end_date)::timestamptz) OR
        -- PRs that had review requested within the period
        (p.review_requested_at >= sqlc.arg(start_date)::timestamptz AND p.review_requested_at <= sqlc.arg(end_date)::timestamptz)
        -- Note: This might fetch more PRs than strictly needed for *averages* if averages are only for merged PRs.
        -- The Go code will need to filter appropriately for each specific metric.
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
FirstActualReviewPerPR AS ( -- Added for consistency, though not directly used for display yet
    SELECT
        pull_request_id,
        MIN(submitted_at) as first_actual_review_at
    FROM pull_request_reviews
    WHERE state = 'APPROVED' OR state = 'CHANGES_REQUESTED'
    GROUP BY pull_request_id
)
SELECT DISTINCT -- Added DISTINCT
    p.id,
    p.repository_name,
    p.title,
    p.author,
    p.state,
    p.created_at AS pr_created_at, -- Renamed for clarity
    p.merged_at AS pr_merged_at,   -- Renamed for clarity
    p.additions,
    p.deletions,
    p.url,
    p.review_requested_at AS pr_review_requested_at, -- Renamed for clarity
    p.reviews_requested AS pr_reviews_requested_count, -- Added for PR list display
    fc.first_commit_at,
    far.first_actual_review_at, -- Added for potential future use or if Go calculates all lead times
    REGEXP_MATCHES(
        COALESCE(p.title, '') || ' ' || COALESCE(p.branch_name, ''),
        '([A-Z]+-[0-9]+)'
    ) AS jira_references, -- Derived JIRA references
    CASE
        WHEN p.review_requested_at IS NOT NULL AND fc.first_commit_at IS NOT NULL AND p.review_requested_at > fc.first_commit_at
        THEN EXTRACT(EPOCH FROM (p.review_requested_at - fc.first_commit_at))
        ELSE NULL
    END AS lead_time_to_code_seconds,
    CASE
        WHEN p.state = 'MERGED' AND p.merged_at IS NOT NULL AND p.review_requested_at IS NOT NULL AND p.review_requested_at < p.merged_at
        THEN EXTRACT(EPOCH FROM (p.merged_at - p.review_requested_at))
        ELSE NULL
    END AS lead_time_to_review_seconds,
    CASE
        WHEN p.state = 'MERGED' AND p.merged_at IS NOT NULL AND fc.first_commit_at IS NOT NULL AND fc.first_commit_at < p.merged_at
        THEN EXTRACT(EPOCH FROM (p.merged_at - fc.first_commit_at))
        ELSE NULL
    END AS lead_time_to_merge_seconds
FROM prs p
LEFT JOIN FirstCommitPerPR fc ON p.id = fc.pr_id
LEFT JOIN FirstActualReviewPerPR far ON p.id = far.pull_request_id -- Joined
LEFT JOIN teams t ON p.author = t.member -- Join with teams table for filtering
WHERE
    -- Date filtering should apply to the primary event defining the list, e.g., merged_at or created_at
    -- For this example, let's assume filtering by merged_at for a list of merged PRs.
    -- Adjust as needed if the list criteria change (e.g., all open PRs, PRs created in range).
    p.merged_at >= sqlc.arg(start_date)::timestamptz
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
        p.state ILIKE '%' || sqlc.arg(filter_state)::text || '%'
    )
    AND ( -- Optionally filter by author (case-insensitive)
        sqlc.arg(filter_author)::text = '' OR
        p.author ILIKE '%' || sqlc.arg(filter_author)::text || '%'
    )
    AND (sqlc.arg(members)::text[] IS NULL OR p.author = ANY(sqlc.arg(members)::text[])) -- Filter by selected members
ORDER BY p.merged_at DESC, p.id ASC -- Default sort by merged_at, then by ID for stable pagination
LIMIT sqlc.arg(page_size)::int
OFFSET sqlc.arg(offset_val)::int;

-- name: CountPullRequests :one
SELECT COUNT(DISTINCT p.id)::int -- Count distinct PR IDs
FROM prs p
LEFT JOIN teams t ON p.author = t.member -- Join with teams table
WHERE
    p.merged_at >= sqlc.arg(start_date)::timestamptz -- Ensure this matches ListPullRequests criteria
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
        p.state ILIKE '%' || sqlc.arg(filter_state)::text || '%'
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

-- Pull Request JIRA References --

-- name: ListPullRequestsWithJiraReferences :many
SELECT
    p.id,
    p.title,
    p.branch_name,
    p.url,
    p.author,
    p.state,
    p.created_at,
    p.merged_at,
    p.repository_name,
    p.repository_owner,
    REGEXP_MATCHES(
        COALESCE(p.title, '') || ' ' || COALESCE(p.branch_name, ''),
        '([A-Z]+-[0-9]+)'
    ) AS jira_references
FROM prs p
LEFT JOIN teams t ON p.author = t.member -- Join with teams table
WHERE
    (COALESCE(p.title, '') ~ '[A-Z]+-[0-9]+' OR COALESCE(p.branch_name, '') ~ '[A-Z]+-[0-9]+') -- Condition for having Jira refs
    AND p.created_at >= sqlc.arg(start_date)::timestamptz
    AND p.created_at <= sqlc.arg(end_date)::timestamptz
    AND (sqlc.arg(text_search_term)::text = '' OR
         p.title ILIKE '%' || sqlc.arg(text_search_term)::text || '%' OR
         p.branch_name ILIKE '%' || sqlc.arg(text_search_term)::text || '%' OR
         p.author ILIKE '%' || sqlc.arg(text_search_term)::text || '%')
    AND (sqlc.arg(team_name)::text = '' OR t.team = sqlc.arg(team_name)::text)
    AND (sqlc.arg(members)::text[] IS NULL OR p.author = ANY(sqlc.arg(members)::text[]))
ORDER BY p.created_at DESC
LIMIT sqlc.arg(page_size)::int OFFSET sqlc.arg(offset_val)::int;

-- name: CountPullRequestsWithJiraReferences :one
SELECT COUNT(DISTINCT p.id) -- Ensure distinct PRs are counted
FROM prs p
LEFT JOIN teams t ON p.author = t.member -- Join with teams table
WHERE
    (COALESCE(p.title, '') ~ '[A-Z]+-[0-9]+' OR COALESCE(p.branch_name, '') ~ '[A-Z]+-[0-9]+')
    AND p.created_at >= sqlc.arg(start_date)::timestamptz
    AND p.created_at <= sqlc.arg(end_date)::timestamptz
    AND (sqlc.arg(text_search_term)::text = '' OR
         p.title ILIKE '%' || sqlc.arg(text_search_term)::text || '%' OR
         p.branch_name ILIKE '%' || sqlc.arg(text_search_term)::text || '%' OR
         p.author ILIKE '%' || sqlc.arg(text_search_term)::text || '%')
    AND (sqlc.arg(team_name)::text = '' OR t.team = sqlc.arg(team_name)::text)
    AND (sqlc.arg(members)::text[] IS NULL OR p.author = ANY(sqlc.arg(members)::text[]));

-- Users --

-- name: CreateUser :one
INSERT INTO users (
    username,
    hashed_password
) VALUES (
    $1, $2
)
RETURNING *;

-- name: GetUserByUsername :one
SELECT id, username, hashed_password, created_at, updated_at
FROM users
WHERE username = $1;

-- name: ListPullRequestsWithoutJiraReferences :many
SELECT
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
LEFT JOIN teams t ON p.author = t.member -- Join with teams table
WHERE
    (COALESCE(p.title, '') !~ '[A-Z]+-[0-9]+' AND COALESCE(p.branch_name, '') !~ '[A-Z]+-[0-9]+') -- Condition for NOT having Jira refs
    AND p.created_at >= sqlc.arg(start_date)::timestamptz
    AND p.created_at <= sqlc.arg(end_date)::timestamptz
    AND (sqlc.arg(text_search_term)::text = '' OR
         p.title ILIKE '%' || sqlc.arg(text_search_term)::text || '%' OR
         p.branch_name ILIKE '%' || sqlc.arg(text_search_term)::text || '%' OR
         p.author ILIKE '%' || sqlc.arg(text_search_term)::text || '%')
    AND (sqlc.arg(team_name)::text = '' OR t.team = sqlc.arg(team_name)::text)
    AND (sqlc.arg(members)::text[] IS NULL OR p.author = ANY(sqlc.arg(members)::text[]));
