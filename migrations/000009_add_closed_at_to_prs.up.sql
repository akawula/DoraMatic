-- Add closed_at column to track when PRs were closed (without merging)
ALTER TABLE prs ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ NULL;

-- Add index for closed_at queries
CREATE INDEX IF NOT EXISTS idx_prs_closed_at ON prs (closed_at);
