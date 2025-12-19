-- Remove closed_at index and column
DROP INDEX IF EXISTS idx_prs_closed_at;
ALTER TABLE prs DROP COLUMN IF EXISTS closed_at;
