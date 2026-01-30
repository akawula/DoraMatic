-- migrations/000010_add_pull_request_files.up.sql

-- Add new columns to prs table for tracking generated code and file count
ALTER TABLE prs ADD COLUMN IF NOT EXISTS changed_files INTEGER DEFAULT 0;
ALTER TABLE prs ADD COLUMN IF NOT EXISTS generated_additions INTEGER DEFAULT 0;
ALTER TABLE prs ADD COLUMN IF NOT EXISTS generated_deletions INTEGER DEFAULT 0;
ALTER TABLE prs ADD COLUMN IF NOT EXISTS files_complete BOOLEAN DEFAULT true;

-- Create table to store individual file changes per PR
CREATE TABLE IF NOT EXISTS pull_request_files (
    id SERIAL PRIMARY KEY,
    pull_request_id VARCHAR(255) NOT NULL REFERENCES prs(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    additions INTEGER NOT NULL DEFAULT 0,
    deletions INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20),  -- added, modified, removed, renamed, copied
    is_generated BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(pull_request_id, file_path)
);

-- Add indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_pr_files_pr_id ON pull_request_files(pull_request_id);
CREATE INDEX IF NOT EXISTS idx_pr_files_generated ON pull_request_files(is_generated);
CREATE INDEX IF NOT EXISTS idx_pr_files_path ON pull_request_files(file_path);

-- Add indexes on prs table for generated code queries
CREATE INDEX IF NOT EXISTS idx_prs_files_complete ON prs(files_complete);
CREATE INDEX IF NOT EXISTS idx_prs_generated_additions ON prs(generated_additions);
