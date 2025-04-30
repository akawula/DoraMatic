-- migrations/000001_initial_schema.down.sql

DROP INDEX IF EXISTS idx_teams_member;
DROP INDEX IF EXISTS idx_teams_team;
DROP TABLE IF EXISTS teams;

DROP INDEX IF EXISTS idx_commits_pr_id;
DROP TABLE IF EXISTS commits;

DROP INDEX IF EXISTS idx_prs_state;
DROP INDEX IF EXISTS idx_prs_merged_at;
DROP INDEX IF EXISTS idx_prs_created_at;
DROP INDEX IF EXISTS idx_prs_author;
DROP INDEX IF EXISTS idx_prs_repo_state_merged;
DROP TABLE IF EXISTS prs;

DROP TABLE IF EXISTS repositories;
