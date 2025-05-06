-- Add created_at column to commits table
ALTER TABLE commits ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ;
