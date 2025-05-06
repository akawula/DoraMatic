-- +migrate Up
CREATE TABLE IF NOT EXISTS pull_request_reviews (
    id TEXT PRIMARY KEY, -- GitHub's node ID for the review
    pull_request_id TEXT NOT NULL REFERENCES prs(id) ON DELETE CASCADE,
    author_login TEXT,
    state VARCHAR(50), -- e.g., APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED, PENDING
    body TEXT,
    url TEXT,
    submitted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pull_request_reviews_pr_id ON pull_request_reviews(pull_request_id);
CREATE INDEX IF NOT EXISTS idx_pull_request_reviews_author ON pull_request_reviews(author_login);
CREATE INDEX IF NOT EXISTS idx_pull_request_reviews_submitted_at ON pull_request_reviews(submitted_at);

-- Optional: Add a function and trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS update_pull_request_reviews_updated_at ON pull_request_reviews; -- Add this line
CREATE TRIGGER update_pull_request_reviews_updated_at
BEFORE UPDATE ON pull_request_reviews
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();
