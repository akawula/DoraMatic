-- Create repository_owners table for storing CODEOWNERS team ownership
-- Maps repositories to the GitHub teams that own them (from CODEOWNERS file)

CREATE TABLE IF NOT EXISTS repository_owners (
    org VARCHAR(255) NOT NULL,
    repo_slug VARCHAR(255) NOT NULL,
    team_slug VARCHAR(255) NOT NULL,  -- GitHub team slug, e.g., "wpengine/plutus"
    PRIMARY KEY (org, repo_slug, team_slug),
    FOREIGN KEY (org, repo_slug) REFERENCES repositories(org, slug) ON DELETE CASCADE
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_repository_owners_team_slug ON repository_owners (team_slug);
CREATE INDEX IF NOT EXISTS idx_repository_owners_repo ON repository_owners (org, repo_slug);
