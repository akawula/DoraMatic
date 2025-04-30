-- migrations/000001_initial_schema.up.sql

CREATE TABLE IF NOT EXISTS repositories (
    org VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    language VARCHAR(100),
    PRIMARY KEY (org, slug)
);

CREATE TABLE IF NOT EXISTS prs (
    id VARCHAR(255) PRIMARY KEY,
    url TEXT,
    title TEXT,
    state VARCHAR(50), -- e.g., OPEN, MERGED, CLOSED
    author VARCHAR(255),
    additions INTEGER,
    deletions INTEGER,
    merged_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    branch_name TEXT,
    repository_name VARCHAR(255),
    repository_owner VARCHAR(255),
    reviews_requested INTEGER,
    review_requested_at TIMESTAMPTZ NULL
);

-- Add indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_prs_repo_state_merged ON prs (repository_owner, repository_name, state, merged_at);
CREATE INDEX IF NOT EXISTS idx_prs_author ON prs (author);
CREATE INDEX IF NOT EXISTS idx_prs_created_at ON prs (created_at);
CREATE INDEX IF NOT EXISTS idx_prs_merged_at ON prs (merged_at);
CREATE INDEX IF NOT EXISTS idx_prs_state ON prs (state);


CREATE TABLE IF NOT EXISTS commits (
    id VARCHAR(255) PRIMARY KEY,
    pr_id VARCHAR(255) NOT NULL REFERENCES prs(id) ON DELETE CASCADE,
    message TEXT
);

-- Add index for joining with prs
CREATE INDEX IF NOT EXISTS idx_commits_pr_id ON commits (pr_id);


CREATE TABLE IF NOT EXISTS teams (
    team VARCHAR(255) NOT NULL,
    member VARCHAR(255) NOT NULL,
    PRIMARY KEY (team, member)
);

-- Add indexes for lookups
CREATE INDEX IF NOT EXISTS idx_teams_team ON teams (team);
CREATE INDEX IF NOT EXISTS idx_teams_member ON teams (member);
