package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// rateLimitState tracks the current rate limit status
type rateLimitState struct {
	remaining int
	resetTime time.Time
	mu        sync.Mutex
}

// GitHubRESTClient defines the interface for GitHub REST API operations.
type GitHubRESTClient interface {
	GetPullRequestFiles(ctx context.Context, owner, repo string, prNumber int, logger *slog.Logger) ([]PullRequestFile, error)
}

// PullRequestFile represents a file changed in a pull request.
type PullRequestFile struct {
	SHA              string `json:"sha"`
	Filename         string `json:"filename"`
	Status           string `json:"status"` // added, removed, modified, renamed, copied, changed, unchanged
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
	PreviousFilename string `json:"previous_filename,omitempty"`
}

// RESTClient implements GitHubRESTClient using the GitHub REST API.
type RESTClient struct {
	httpClient   *http.Client
	token        string
	baseURL      string
	rateLimit    rateLimitState
	requestDelay time.Duration // Delay between requests to avoid rate limiting
}

// NewRESTClient creates a new GitHub REST API client.
func NewRESTClient() *RESTClient {
	// Default delay of 100ms between requests (allows ~600 requests/minute, well under the 5000/hour limit)
	delay := 100 * time.Millisecond
	if envDelay := os.Getenv("GITHUB_REST_DELAY_MS"); envDelay != "" {
		if d, err := strconv.Atoi(envDelay); err == nil && d >= 0 {
			delay = time.Duration(d) * time.Millisecond
		}
	}

	return &RESTClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token:        os.Getenv("GITHUB_TOKEN"),
		baseURL:      "https://api.github.com",
		requestDelay: delay,
	}
}

// GetPullRequestFiles fetches all files changed in a pull request with pagination.
// It handles rate limiting and retries with exponential backoff.
func (c *RESTClient) GetPullRequestFiles(ctx context.Context, owner, repo string, prNumber int, logger *slog.Logger) ([]PullRequestFile, error) {
	// Check if we need to wait for rate limit reset before starting
	c.waitForRateLimit(ctx, logger)

	var allFiles []PullRequestFile
	page := 1
	perPage := 100 // Maximum allowed by GitHub API
	maxRetries := 3

	for {
		url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files?page=%d&per_page=%d",
			c.baseURL, owner, repo, prNumber, page, perPage)

		files, hasMore, err := c.fetchFilesPage(ctx, url, maxRetries, logger)
		if err != nil {
			return allFiles, fmt.Errorf("fetching page %d: %w", page, err)
		}

		allFiles = append(allFiles, files...)

		if !hasMore || len(files) < perPage {
			break
		}

		page++

		// GitHub REST API has a hard limit of 3000 files per PR
		if len(allFiles) >= 3000 {
			logger.Warn("Reached GitHub API limit of 3000 files per PR",
				"owner", owner, "repo", repo, "pr", prNumber, "files_fetched", len(allFiles))
			break
		}

		// Add delay between pagination requests
		if c.requestDelay > 0 {
			select {
			case <-ctx.Done():
				return allFiles, ctx.Err()
			case <-time.After(c.requestDelay):
			}
		}
	}

	return allFiles, nil
}

// waitForRateLimit checks if we're rate limited and waits if necessary
func (c *RESTClient) waitForRateLimit(ctx context.Context, logger *slog.Logger) {
	c.rateLimit.mu.Lock()
	remaining := c.rateLimit.remaining
	resetTime := c.rateLimit.resetTime
	c.rateLimit.mu.Unlock()

	// If we have remaining requests or reset time is in the past, proceed
	if remaining > 10 || resetTime.Before(time.Now()) {
		return
	}

	// Wait until reset time
	waitDuration := time.Until(resetTime)
	if waitDuration <= 0 {
		return
	}

	// Cap maximum wait at 1 hour
	maxWait := 60 * time.Minute
	if waitDuration > maxWait {
		waitDuration = maxWait
	}

	logger.Warn("Rate limit low, waiting for reset",
		"remaining", remaining,
		"reset_in_seconds", waitDuration.Seconds())
	select {
	case <-ctx.Done():
		return
	case <-time.After(waitDuration + time.Second): // Add 1 second buffer
	}
}

// fetchFilesPage fetches a single page of files with retry logic.
func (c *RESTClient) fetchFilesPage(ctx context.Context, url string, maxRetries int, logger *slog.Logger) ([]PullRequestFile, bool, error) {
	var lastErr error
	backoff := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logger.Debug("Retrying request", "url", url, "attempt", attempt, "backoff", backoff)
			select {
			case <-ctx.Done():
				return nil, false, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, false, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Handle rate limiting
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			// Update rate limit state from headers even on error
			c.updateRateLimitFromHeaders(resp.Header, logger)

			resetTime := c.parseRateLimitReset(resp.Header.Get("X-RateLimit-Reset"))
			remaining := resp.Header.Get("X-RateLimit-Remaining")

			resp.Body.Close()

			// Calculate wait duration
			var waitDuration time.Duration
			if resetTime > 0 {
				waitDuration = time.Until(time.Unix(resetTime, 0))
			}

			// If we couldn't parse reset time or it's in the past, use a default wait
			if waitDuration <= 0 {
				waitDuration = 60 * time.Second // Default 1 minute wait
			}

			// Cap maximum wait at 1 hour to avoid infinite waits
			maxWait := 60 * time.Minute
			if waitDuration > maxWait {
				waitDuration = maxWait
			}

			logger.Warn("Rate limited, waiting for reset",
				"wait_seconds", waitDuration.Seconds(),
				"remaining", remaining,
				"reset_at", time.Unix(resetTime, 0).Format(time.RFC3339))

			select {
			case <-ctx.Done():
				return nil, false, ctx.Err()
			case <-time.After(waitDuration + time.Second): // Add 1 second buffer
			}

			// Reset retry counter after waiting for rate limit - don't count rate limit waits as retries
			attempt = -1 // Will become 0 after increment
			backoff = time.Second
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
			continue
		}

		// Update rate limit state from headers
		c.updateRateLimitFromHeaders(resp.Header, logger)

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response: %w", err)
			continue
		}

		var files []PullRequestFile
		if err := json.Unmarshal(body, &files); err != nil {
			return nil, false, fmt.Errorf("parsing response: %w", err)
		}

		// Check if there are more pages via Link header
		hasMore := c.hasNextPage(resp.Header.Get("Link"))

		return files, hasMore, nil
	}

	return nil, false, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// parseRateLimitReset parses the X-RateLimit-Reset header.
func (c *RESTClient) parseRateLimitReset(value string) int64 {
	if value == "" {
		return 0
	}
	resetTime, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return resetTime
}

// updateRateLimitFromHeaders updates the rate limit state from response headers
func (c *RESTClient) updateRateLimitFromHeaders(headers http.Header, logger *slog.Logger) {
	c.rateLimit.mu.Lock()
	defer c.rateLimit.mu.Unlock()

	if remaining := headers.Get("X-RateLimit-Remaining"); remaining != "" {
		if r, err := strconv.Atoi(remaining); err == nil {
			c.rateLimit.remaining = r
			// Log when rate limit is getting low
			if r < 100 && r%20 == 0 {
				logger.Warn("GitHub REST API rate limit getting low", "remaining", r)
			}
		}
	}

	if reset := headers.Get("X-RateLimit-Reset"); reset != "" {
		if resetUnix, err := strconv.ParseInt(reset, 10, 64); err == nil {
			c.rateLimit.resetTime = time.Unix(resetUnix, 0)
		}
	}
}

// hasNextPage checks if the Link header indicates more pages.
func (c *RESTClient) hasNextPage(linkHeader string) bool {
	// Simple check - if the header contains rel="next", there are more pages
	return linkHeader != "" && contains(linkHeader, `rel="next"`)
}

// contains is a simple string contains check.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
