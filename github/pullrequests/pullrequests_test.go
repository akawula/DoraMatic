package pullrequests

import (
	"context"       // Keep one context
	"encoding/json" // Keep one encoding/json
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/akawula/DoraMatic/github/client" // Import client package for mock
	"github.com/shurcooL/githubv4"
)

// --- Mocks and Test Setup ---

// Mock structure for the pull request query result
type mockPullRequestQueryResult struct {
	Repository struct {
		PullRequests struct {
			Nodes    []PullRequest
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		} `graphql:"pullRequests(first:30, orderBy: {field: CREATED_AT, direction: DESC}, states: [MERGED, OPEN], after: $after)"`
	} `graphql:"repository(name: $name, owner: $login)"`
}

// --- Tests ---

func TestGetPullRequests_Success_SinglePage_StopEarly(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})) // Keep logs quiet unless error
	org := "test-org"
	repo := "test-repo"
	lastDBDate := time.Now().AddDate(0, -1, 0) // 1 month ago

	// Prepare mock response
	mockPRs := []PullRequest{
		{Id: "PR1", CreatedAt: githubv4.String(time.Now().AddDate(0, 0, -1).Format(time.RFC3339))}, // Yesterday
		{Id: "PR2", CreatedAt: githubv4.String(time.Now().AddDate(0, -2, 0).Format(time.RFC3339))}, // 2 months ago (older than lastDBDate)
	}
	mockResponse := mockPullRequestQueryResult{}
	mockResponse.Repository.PullRequests.Nodes = mockPRs
	mockResponse.Repository.PullRequests.PageInfo.HasNextPage = false // No next page

	mockClient.SetResponse(mockResponse)
	mockClient.ExpectedVariables = map[string]interface{}{"login": githubv4.String(org), "name": githubv4.String(repo), "after": (*githubv4.String)(nil)}

	// Call the function
	prs, err := Get(mockClient, org, repo, lastDBDate, logger)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if mockClient.QueryCallCount != 1 {
		t.Errorf("Expected Query to be called 1 time, got %d", mockClient.QueryCallCount)
	}
	// The loop fetches all nodes from the page, then checks the date of the *last* one (PR2).
	// Since PR2 is older than lastDBDate, the loop breaks.
	// It should return *all* PRs fetched in that query (PR1, PR2).
	if len(prs) != 2 {
		t.Errorf("Expected 2 PRs from the single page fetch, got %d", len(prs))
	}
}

func TestGetPullRequests_Success_Paginated(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	org := "test-org-p"
	repo := "test-repo-p"
	lastDBDate := time.Now().AddDate(0, -3, 0) // 3 months ago

	// --- Mock Response Page 1 ---
	mockPRs1 := []PullRequest{
		{Id: "PR10", CreatedAt: githubv4.String(time.Now().AddDate(0, 0, -5).Format(time.RFC3339))},   // 5 days ago
		{Id: "PR11", CreatedAt: githubv4.String(time.Now().AddDate(0, -1, -10).Format(time.RFC3339))}, // ~1.3 months ago
	}
	mockResponse1 := mockPullRequestQueryResult{}
	mockResponse1.Repository.PullRequests.Nodes = mockPRs1
	mockResponse1.Repository.PullRequests.PageInfo.HasNextPage = true
	mockResponse1.Repository.PullRequests.PageInfo.EndCursor = "CURSOR1"

	// --- Mock Response Page 2 ---
	mockPRs2 := []PullRequest{
		{Id: "PR12", CreatedAt: githubv4.String(time.Now().AddDate(0, -2, -15).Format(time.RFC3339))}, // ~2.5 months ago
		{Id: "PR13", CreatedAt: githubv4.String(time.Now().AddDate(0, -4, 0).Format(time.RFC3339))},   // 4 months ago (older than lastDBDate)
	}
	mockResponse2 := mockPullRequestQueryResult{}
	mockResponse2.Repository.PullRequests.Nodes = mockPRs2
	mockResponse2.Repository.PullRequests.PageInfo.HasNextPage = false // Last page

	// Use QueryFunc to handle multiple calls with different responses/checks
	callCount := 0
	mockClient.QueryFunc = func(ctx context.Context, q interface{}, variables map[string]interface{}) error {
		callCount++
		// Check variables based on call count
		var responseToUse interface{}

		if callCount == 1 {
			responseToUse = mockResponse1
		} else if callCount == 2 {
			responseToUse = mockResponse2
		} else {
			return fmt.Errorf("unexpected query call number %d", callCount)
		}

		// No cursor check inside mock - rely on final assertion

		// Simulate response population
		respBytes, _ := json.Marshal(responseToUse)
		err := json.Unmarshal(respBytes, q)
		if err != nil {
			t.Fatalf("Failed to unmarshal mock response in call %d: %v", callCount, err)
		}
		return nil
	}

	prs, err := Get(mockClient, org, repo, lastDBDate, logger)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if callCount != 2 { // Check if Query was called twice
		t.Errorf("Expected Query to be called 2 times for pagination, got %d", callCount)
	}

	// The loop should stop after page 2 because the last PR (PR13) is older than lastDBDate.
	// It should return all PRs collected so far (PR10, PR11, PR12, PR13).
	if len(prs) != 4 {
		t.Errorf("Expected 4 PRs after pagination, got %d", len(prs))
	}
}

func TestGetPullRequests_Error_WithRetries(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	org := "test-org-err"
	repo := "test-repo-err"
	lastDBDate := time.Now()
	expectedError := fmt.Errorf("GitHub API error")

	// Set expectation for the first call to return an error
	mockClient.ExpectedVariables = map[string]interface{}{"login": githubv4.String(org), "name": githubv4.String(repo), "after": (*githubv4.String)(nil)}
	mockClient.SetError(expectedError) // Error will be returned on every call

	// Call the function
	prs, err := Get(mockClient, org, repo, lastDBDate, logger)

	// Assertions
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	// The specific error might be wrapped, check underlying cause if needed, but checking message often suffices
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
	if prs != nil {
		t.Errorf("Expected nil PRs on error, got %v", prs)
	}
	// Query should be called multiple times due to retries (default 3 retries + 1 initial = 4 calls)
	if mockClient.QueryCallCount != 4 {
		t.Errorf("Expected Query to be called 4 times (1 initial + 3 retries), got %d", mockClient.QueryCallCount)
	}
}

func TestCheckDates(t *testing.T) {
	lastDbDate := time.Date(2024, 5, 15, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name     string
		ghDate   githubv4.String
		expected bool
	}{
		{
			name:     "GitHub date is before last DB date",
			ghDate:   githubv4.String(time.Date(2024, 5, 10, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)),
			expected: true,
		},
		{
			name:     "GitHub date is after last DB date",
			ghDate:   githubv4.String(time.Date(2024, 5, 20, 14, 0, 0, 0, time.UTC).Format(time.RFC3339)),
			expected: false,
		},
		{
			name:     "GitHub date is the same as last DB date",
			ghDate:   githubv4.String(lastDbDate.Format(time.RFC3339)),
			expected: false, // Before() returns false for equal times
		},
		{
			name:     "GitHub date is slightly before last DB date",
			ghDate:   githubv4.String(lastDbDate.Add(-time.Second).Format(time.RFC3339)),
			expected: true,
		},
		{
			name:   "Invalid GitHub date format",
			ghDate: githubv4.String("invalid-date-string"),
			// Expecting true because parsing fails, resulting in the zero time.Time{},
			// which is before the lastDbDate (year 2024).
			// Note: The function logs an error in this case but doesn't return it.
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := checkDates(lastDbDate, tc.ghDate)
			if actual != tc.expected {
				t.Errorf("Expected checkDates(%v, %q) to be %v, but got %v", lastDbDate, tc.ghDate, tc.expected, actual)
			}
		})
	}
}
