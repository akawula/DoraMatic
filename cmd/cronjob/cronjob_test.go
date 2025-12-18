package main

import (
	"context"
	"io" // For discarding logger output in tests
	"log/slog"
	"os"
	"testing"
	"time"

	// Import packages needed for mocks and App struct
	"github.com/akawula/DoraMatic/github/client"
	"github.com/akawula/DoraMatic/github/codeowners"    // Import codeowners for mock
	"github.com/akawula/DoraMatic/github/organizations" // Import organizations for MemberInfo
	"github.com/akawula/DoraMatic/github/pullrequests"
	"github.com/akawula/DoraMatic/github/repositories"
	"github.com/akawula/DoraMatic/store"
	"github.com/akawula/DoraMatic/store/sqlc" // For store method signatures
	ghTypes "github.com/shurcooL/githubv4"    // Alias to avoid conflict
)

// --- Mocks ---

// MockGitHubClient implements client.GitHubV4Client
type MockGitHubClient struct {
	QueryFunc func(ctx context.Context, q interface{}, variables map[string]interface{}) error
}

func (m *MockGitHubClient) Query(ctx context.Context, q interface{}, variables map[string]interface{}) error {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, q, variables)
	}
	// Default behavior: return nil, assuming success if QueryFunc is not set
	return nil
}

// MockStore implements store.Store for testing
type MockStore struct {
	// Updated SaveTeamsFunc signature
	SaveTeamsFunc                 func(ctx context.Context, teams map[string][]organizations.MemberInfo) error
	GetLastPRDateFunc             func(ctx context.Context, org string, repo string) time.Time
	SavePullRequestFunc           func(ctx context.Context, prs []pullrequests.PullRequest) error
	FetchSecurityPullRequestsFunc func() ([]store.SecurityPR, error)
	CloseFunc                     func()
	// New methods for team stats
	CountTeamCommitsByDateRangeFunc        func(ctx context.Context, arg sqlc.CountTeamCommitsByDateRangeParams) (int32, error)
	GetTeamPullRequestStatsByDateRangeFunc func(ctx context.Context, arg sqlc.GetTeamPullRequestStatsByDateRangeParams) (sqlc.GetTeamPullRequestStatsByDateRangeRow, error)
	// New methods for listing PRs
	ListPullRequestsFunc  func(ctx context.Context, arg sqlc.ListPullRequestsParams) ([]sqlc.ListPullRequestsRow, error)
	CountPullRequestsFunc func(ctx context.Context, arg sqlc.CountPullRequestsParams) (int32, error)
	// New method for getting team members
	GetTeamMembersFunc func(ctx context.Context, team string) ([]sqlc.GetTeamMembersRow, error)
	// New method for diagnosing lead times
	DiagnoseLeadTimesFunc func(ctx context.Context) ([]sqlc.DiagnoseLeadTimesRow, error)
	// New method for PR time data
	GetPullRequestTimeDataForStatsFunc func(ctx context.Context, arg sqlc.GetPullRequestTimeDataForStatsParams) ([]sqlc.GetPullRequestTimeDataForStatsRow, error)
	// New method for team member review stats
	GetTeamMemberReviewStatsByDateRangeFunc func(ctx context.Context, arg sqlc.GetTeamMemberReviewStatsByDateRangeParams) ([]sqlc.GetTeamMemberReviewStatsByDateRangeRow, error)
	// New method for repository owners (CODEOWNERS)
	SaveRepositoryOwnersFunc func(ctx context.Context, ownerships []codeowners.RepositoryOwnership) error

	// Add fields to track calls if needed
	SaveTeamsCalled                           bool
	GetLastPRDateCalledWith                   [][2]string // Store org/repo pairs
	SavePullRequestCalled                     bool
	FetchSecurityPullRequestsCalled           bool
	CloseCalled                               bool
	CountTeamCommitsByDateRangeCalled         bool
	GetTeamPullRequestStatsByDateRangeCalled  bool
	ListPullRequestsCalled                    bool
	CountPullRequestsCalled                   bool
	GetTeamMembersCalled                      bool
	DiagnoseLeadTimesCalled                   bool
	GetPullRequestTimeDataForStatsCalled      bool
	GetTeamMemberReviewStatsByDateRangeCalled bool
	SaveRepositoryOwnersCalled                bool
}

// Updated SaveTeams method signature
func (m *MockStore) SaveTeams(ctx context.Context, teams map[string][]organizations.MemberInfo) error {
	m.SaveTeamsCalled = true
	if m.SaveTeamsFunc != nil {
		return m.SaveTeamsFunc(ctx, teams)
	}
	return nil // Default mock behavior
}

func (m *MockStore) GetLastPRDate(ctx context.Context, org string, repo string) time.Time {
	m.GetLastPRDateCalledWith = append(m.GetLastPRDateCalledWith, [2]string{org, repo})
	if m.GetLastPRDateFunc != nil {
		return m.GetLastPRDateFunc(ctx, org, repo)
	}
	return time.Time{} // Default mock behavior (zero time)
}

func (m *MockStore) SavePullRequest(ctx context.Context, prs []pullrequests.PullRequest) error {
	m.SavePullRequestCalled = true
	if m.SavePullRequestFunc != nil {
		return m.SavePullRequestFunc(ctx, prs)
	}
	return nil // Default mock behavior
}

func (m *MockStore) FetchSecurityPullRequests() ([]store.SecurityPR, error) {
	m.FetchSecurityPullRequestsCalled = true
	if m.FetchSecurityPullRequestsFunc != nil {
		return m.FetchSecurityPullRequestsFunc()
	}
	return []store.SecurityPR{}, nil // Default mock behavior
}

func (m *MockStore) Close() {
	m.CloseCalled = true
	if m.CloseFunc != nil {
		m.CloseFunc()
	}
}

// Implement unused store.Store methods to satisfy the interface
func (m *MockStore) GetRepos(ctx context.Context, page int, search string) ([]sqlc.Repository, int, error) {
	return nil, 0, nil
}
func (m *MockStore) SaveRepos(ctx context.Context, repos []repositories.Repository) error { return nil }
func (m *MockStore) GetAllRepos(ctx context.Context) ([]sqlc.Repository, error)           { return nil, nil }
func (m *MockStore) SearchDistinctTeamNamesByPrefix(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

// Implement new store methods for team stats
func (m *MockStore) CountTeamCommitsByDateRange(ctx context.Context, arg sqlc.CountTeamCommitsByDateRangeParams) (int32, error) {
	m.CountTeamCommitsByDateRangeCalled = true // Keep tracking flag
	if m.CountTeamCommitsByDateRangeFunc != nil {
		return m.CountTeamCommitsByDateRangeFunc(ctx, arg)
	}
	return 0, nil // Default mock behavior
}

func (m *MockStore) GetTeamPullRequestStatsByDateRange(ctx context.Context, arg sqlc.GetTeamPullRequestStatsByDateRangeParams) (sqlc.GetTeamPullRequestStatsByDateRangeRow, error) {
	m.GetTeamPullRequestStatsByDateRangeCalled = true
	if m.GetTeamPullRequestStatsByDateRangeFunc != nil {
		return m.GetTeamPullRequestStatsByDateRangeFunc(ctx, arg)
	}
	// Default mock behavior returns zero values for the struct
	return sqlc.GetTeamPullRequestStatsByDateRangeRow{}, nil
}

// Implement new store methods for listing PRs
func (m *MockStore) ListPullRequests(ctx context.Context, arg sqlc.ListPullRequestsParams) ([]sqlc.ListPullRequestsRow, error) {
	m.ListPullRequestsCalled = true
	if m.ListPullRequestsFunc != nil {
		return m.ListPullRequestsFunc(ctx, arg)
	}
	return []sqlc.ListPullRequestsRow{}, nil // Default mock behavior
}

func (m *MockStore) CountPullRequests(ctx context.Context, arg sqlc.CountPullRequestsParams) (int32, error) {
	m.CountPullRequestsCalled = true
	if m.CountPullRequestsFunc != nil {
		return m.CountPullRequestsFunc(ctx, arg)
	}
	return 0, nil // Default mock behavior
}

// Implement new store method for getting team members
func (m *MockStore) GetTeamMembers(ctx context.Context, team string) ([]sqlc.GetTeamMembersRow, error) {
	m.GetTeamMembersCalled = true
	if m.GetTeamMembersFunc != nil {
		return m.GetTeamMembersFunc(ctx, team)
	}
	return []sqlc.GetTeamMembersRow{}, nil // Default mock behavior
}

// Implement DiagnoseLeadTimes for MockStore
func (m *MockStore) DiagnoseLeadTimes(ctx context.Context) ([]sqlc.DiagnoseLeadTimesRow, error) {
	m.DiagnoseLeadTimesCalled = true
	if m.DiagnoseLeadTimesFunc != nil {
		return m.DiagnoseLeadTimesFunc(ctx)
	}
	return []sqlc.DiagnoseLeadTimesRow{}, nil // Default mock behavior
}

// Implement GetPullRequestTimeDataForStats for MockStore
func (m *MockStore) GetPullRequestTimeDataForStats(ctx context.Context, arg sqlc.GetPullRequestTimeDataForStatsParams) ([]sqlc.GetPullRequestTimeDataForStatsRow, error) {
	m.GetPullRequestTimeDataForStatsCalled = true
	if m.GetPullRequestTimeDataForStatsFunc != nil {
		return m.GetPullRequestTimeDataForStatsFunc(ctx, arg)
	}
	return []sqlc.GetPullRequestTimeDataForStatsRow{}, nil // Default mock behavior
}

// Implement GetTeamMemberReviewStatsByDateRange for MockStore
func (m *MockStore) GetTeamMemberReviewStatsByDateRange(ctx context.Context, arg sqlc.GetTeamMemberReviewStatsByDateRangeParams) ([]sqlc.GetTeamMemberReviewStatsByDateRangeRow, error) {
	m.GetTeamMemberReviewStatsByDateRangeCalled = true
	if m.GetTeamMemberReviewStatsByDateRangeFunc != nil {
		return m.GetTeamMemberReviewStatsByDateRangeFunc(ctx, arg)
	}
	return []sqlc.GetTeamMemberReviewStatsByDateRangeRow{}, nil // Default mock behavior
}

// Implement Jira reference methods for MockStore
func (m *MockStore) CountPullRequestsWithJiraReferences(ctx context.Context, arg sqlc.CountPullRequestsWithJiraReferencesParams) (int64, error) {
	return 0, nil
}

func (m *MockStore) CountPullRequestsWithoutJiraReferences(ctx context.Context, arg sqlc.CountPullRequestsWithoutJiraReferencesParams) (int64, error) {
	return 0, nil
}

func (m *MockStore) ListPullRequestsWithJiraReferences(ctx context.Context, arg sqlc.ListPullRequestsWithJiraReferencesParams) ([]sqlc.ListPullRequestsWithJiraReferencesRow, error) {
	return []sqlc.ListPullRequestsWithJiraReferencesRow{}, nil
}

func (m *MockStore) ListPullRequestsWithoutJiraReferencesWithPagination(ctx context.Context, arg sqlc.ListPullRequestsWithoutJiraReferencesParamsWithPagination) ([]sqlc.ListPullRequestsWithoutJiraReferencesRow, error) {
	return []sqlc.ListPullRequestsWithoutJiraReferencesRow{}, nil
}

// Implement User methods for MockStore
func (m *MockStore) GetUserByUsername(ctx context.Context, username string) (sqlc.User, error) {
	return sqlc.User{}, nil
}

// Implement SonarQube methods for MockStore
func (m *MockStore) SaveSonarQubeProject(ctx context.Context, projectKey, projectName string) error {
	return nil
}

func (m *MockStore) SaveSonarQubeMetrics(ctx context.Context, projectKey string, metrics map[string]float64, recordedAt time.Time) error {
	return nil
}

// Implement SaveRepositoryOwners for MockStore
func (m *MockStore) SaveRepositoryOwners(ctx context.Context, ownerships []codeowners.RepositoryOwnership) error {
	m.SaveRepositoryOwnersCalled = true
	if m.SaveRepositoryOwnersFunc != nil {
		return m.SaveRepositoryOwnersFunc(ctx, ownerships)
	}
	return nil // Default mock behavior
}

// Mock function types for GitHub interactions
// Updated MockGetTeamsFunc signature
type MockGetTeamsFunc func(ghClient client.GitHubV4Client) (map[string][]organizations.MemberInfo, error)
type MockGetReposFunc func(ghClient client.GitHubV4Client) ([]repositories.Repository, error)

// Updated MockGetPullRequestsFunc signature to match cronjob.go
type MockGetPullRequestsFunc func(ghClient client.GitHubV4Client, org string, repo string, since time.Time, l *slog.Logger) ([]pullrequests.PullRequest, error)

// Mock function type for Slack
type MockSendMessageFunc func(prs []store.SecurityPR)

// Variables to hold the mock functions, allowing tests to swap them
// --- Test for App.Run ---

func TestAppRun_Success(t *testing.T) {
	// Setup: Create mocks
	mockDB := &MockStore{}
	mockGHClient := &MockGitHubClient{}                          // Use the new mock client
	testLogger := slog.New(slog.NewJSONHandler(io.Discard, nil)) // Discard logs during test

	// Mock data - Updated testTeams to use MemberInfo
	testTeams := map[string][]organizations.MemberInfo{
		"team-a": {{Login: "member1", AvatarUrl: "url1"}},
	}
	testRepo := repositories.Repository{
		Name:  "repo1",
		Owner: struct{ Login ghTypes.String }{Login: "org1"},
	}
	testRepos := []repositories.Repository{testRepo}
	testPRs := []pullrequests.PullRequest{{Id: "pr1"}} // Corrected: ID -> Id
	testSecPRs := []store.SecurityPR{{Id: "secpr1"}}
	var capturedSecPRs []store.SecurityPR // To capture args passed to SendMessage
	var sendMessageCalled bool            // Declare sendMessageCalled

	// Configure mock behaviors - Updated SaveTeamsFunc mock
	mockDB.SaveTeamsFunc = func(ctx context.Context, teams map[string][]organizations.MemberInfo) error {
		if len(teams) != 1 || teams["team-a"][0].Login != "member1" || teams["team-a"][0].AvatarUrl != "url1" {
			t.Errorf("SaveTeams called with unexpected teams: %+v", teams)
		}
		return nil
	}
	mockDB.GetLastPRDateFunc = func(ctx context.Context, org string, repo string) time.Time {
		if org != "org1" || repo != "repo1" {
			t.Errorf("GetLastPRDate called with unexpected org/repo: %s/%s", org, repo)
		}
		return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // Return a fixed time
	}
	mockDB.SavePullRequestFunc = func(ctx context.Context, prs []pullrequests.PullRequest) error {
		if len(prs) != 1 || prs[0].Id != "pr1" { // Corrected: ID -> Id
			t.Errorf("SavePullRequest called with unexpected PRs: %+v", prs)
		}
		return nil
	}
	mockDB.FetchSecurityPullRequestsFunc = func() ([]store.SecurityPR, error) {
		return testSecPRs, nil
	}

	// Updated mockGetTeamsImpl signature and return value
	mockGetTeamsImpl := func(ghClient client.GitHubV4Client) (map[string][]organizations.MemberInfo, error) {
		return testTeams, nil
	}
	// Updated mockGetReposImpl signature
	mockGetReposImpl := func(ghClient client.GitHubV4Client) ([]repositories.Repository, error) {
		return testRepos, nil
	}
	mockGetPullRequestsImpl := func(ghClient client.GitHubV4Client, org string, repo string, since time.Time, l *slog.Logger) ([]pullrequests.PullRequest, error) {
		if org != "org1" || repo != "repo1" {
			t.Errorf("GetPullRequests called with unexpected org/repo: %s/%s", org, repo)
		}
		// Check 'since' date if needed
		return testPRs, nil
	}
	mockSendMessageImpl := func(prs []store.SecurityPR) {
		sendMessageCalled = true // Mark as called
		capturedSecPRs = prs     // Capture arguments
	}
	mockGetCodeownersImpl := func(ghClient client.GitHubV4Client, org string, repo string, l *slog.Logger) (*codeowners.RepositoryOwnership, error) {
		// Return a mock ownership with one team
		return &codeowners.RepositoryOwnership{
			Org:      org,
			RepoSlug: repo,
			Teams:    []string{"test-org/test-team"},
		}, nil
	}

	// Create App instance with mocks
	app := NewApp(
		testLogger,
		mockDB,
		mockGHClient,
		mockGetTeamsImpl,        // Pass mock implementations directly
		mockGetReposImpl,        // Pass mock implementations directly
		mockGetPullRequestsImpl, // Pass mock implementations directly
		mockGetCodeownersImpl,   // Pass mock implementations directly
		mockSendMessageImpl,     // Pass mock implementations directly
	)

	// Execute: Run the app logic
	err := app.Run(context.Background())

	// Assert: Check results and mock calls
	if err != nil {
		t.Fatalf("App.Run() returned an unexpected error: %v", err)
	}
	if !mockDB.SaveTeamsCalled {
		t.Error("Expected db.SaveTeams to be called, but it wasn't.")
	}
	if len(mockDB.GetLastPRDateCalledWith) != 1 {
		t.Errorf("Expected db.GetLastPRDate to be called once, called %d times.", len(mockDB.GetLastPRDateCalledWith))
	}
	if !mockDB.SavePullRequestCalled {
		t.Error("Expected db.SavePullRequest to be called, but it wasn't.")
	}
	if !mockDB.FetchSecurityPullRequestsCalled {
		t.Error("Expected db.FetchSecurityPullRequests to be called, but it wasn't.")
	}
	if !sendMessageCalled {
		t.Error("Expected SendMessage to be called, but it wasn't.")
	}
	if len(capturedSecPRs) != 1 || capturedSecPRs[0].Id != "secpr1" {
		t.Errorf("SendMessage called with unexpected PRs: %+v", capturedSecPRs)
	}
	// Add more assertions as needed (e.g., checking mockGHClient calls if relevant)
}

// TODO: Add more test cases for error scenarios (e.g., GetTeams fails, SaveRepos fails, etc.)

// --- Original Tests ---

func TestDebug(t *testing.T) {
	// Test case 1: DEBUG environment variable is not set or empty
	os.Unsetenv("DEBUG") // Ensure the variable is not set
	expectedLevel := slog.LevelInfo
	actualLevel := debug()
	if actualLevel != expectedLevel {
		t.Errorf("Expected log level %v when DEBUG is not set, but got %v", expectedLevel, actualLevel)
	}

	// Test case 2: DEBUG environment variable is set to "1"
	os.Setenv("DEBUG", "1")
	expectedLevel = slog.LevelDebug
	actualLevel = debug()
	if actualLevel != expectedLevel {
		t.Errorf("Expected log level %v when DEBUG=1, but got %v", expectedLevel, actualLevel)
	}

	// Test case 3: DEBUG environment variable is set to something else (e.g., "0")
	os.Setenv("DEBUG", "0")
	expectedLevel = slog.LevelInfo // Should default to Info
	actualLevel = debug()
	if actualLevel != expectedLevel {
		t.Errorf("Expected log level %v when DEBUG is not '1', but got %v", expectedLevel, actualLevel)
	}

	// Clean up the environment variable after the test
	os.Unsetenv("DEBUG")
}

func TestLogger(t *testing.T) {
	// Call the logger function
	l := logger()

	// Basic check: ensure the logger is not nil
	if l == nil {
		t.Fatal("logger() returned nil, expected a valid *slog.Logger instance")
	}

	// Optional: You could potentially try to capture stdout and verify JSON format,
	// but that adds complexity. For now, ensuring it doesn't panic and returns
	// a non-nil logger is a reasonable basic test.
	// We already test the log level logic via TestDebug.
}
