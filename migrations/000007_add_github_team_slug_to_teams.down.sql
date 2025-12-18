-- Remove github_team_slug column from teams table

DROP INDEX IF EXISTS idx_teams_github_team_slug;
ALTER TABLE teams DROP COLUMN IF EXISTS github_team_slug;
