-- migrations/000010_add_pull_request_files.down.sql

-- Drop indexes first
DROP INDEX IF EXISTS idx_prs_generated_additions;
DROP INDEX IF EXISTS idx_prs_files_complete;
DROP INDEX IF EXISTS idx_pr_files_path;
DROP INDEX IF EXISTS idx_pr_files_generated;
DROP INDEX IF EXISTS idx_pr_files_pr_id;

-- Drop the files table
DROP TABLE IF EXISTS pull_request_files;

-- Remove columns from prs table
ALTER TABLE prs DROP COLUMN IF EXISTS files_complete;
ALTER TABLE prs DROP COLUMN IF EXISTS generated_deletions;
ALTER TABLE prs DROP COLUMN IF EXISTS generated_additions;
ALTER TABLE prs DROP COLUMN IF EXISTS changed_files;
