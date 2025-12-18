-- Add github_team_slug column to teams table for mapping to CODEOWNERS team slugs
-- Example: team "Golden" maps to github_team_slug "wpengine/golden"

ALTER TABLE teams ADD COLUMN github_team_slug VARCHAR(255);

-- Create index for lookups by github_team_slug
CREATE INDEX IF NOT EXISTS idx_teams_github_team_slug ON teams (github_team_slug);
