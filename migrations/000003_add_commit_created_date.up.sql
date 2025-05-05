-- Add created_at column to commits table
ALTER TABLE commits ADD COLUMN created_at TIMESTAMPTZ;
