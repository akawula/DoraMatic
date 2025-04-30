-- +migrate Down
-- Drop the trigger first if it exists
DROP TRIGGER IF EXISTS update_pull_request_reviews_updated_at ON pull_request_reviews;

-- Drop the function if it exists (optional, might be used by other tables)
-- Consider if update_updated_at_column is used elsewhere before uncommenting:
-- DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_pull_request_reviews_submitted_at;
DROP INDEX IF EXISTS idx_pull_request_reviews_author;
DROP INDEX IF EXISTS idx_pull_request_reviews_pr_id;

-- Drop the table
DROP TABLE IF EXISTS pull_request_reviews;
