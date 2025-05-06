-- Add avatar_url column to teams table
ALTER TABLE teams ADD COLUMN IF NOT EXISTS avatar_url TEXT;
