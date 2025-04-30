package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/akawula/DoraMatic/github/client" // Import client package for mock
	// Mocking organizations.Get requires a way to intercept or replace it,
	// or restructure Get to accept orgs []string, or use a more complex mock setup.
	// For simplicity, we'll focus on testing getRepos directly first.
	// We can add tests for Get later if needed, potentially by making organizations.Get injectable too.
	"github.com/shurcooL/githubv4"
)

// --- Mock Structures ---

// Mock structure for the getRepos query result
type mockRepoQueryResult struct {
	Organization struct {
		Repositories struct {
			Nodes    []Repository
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		} `graphql:"repositories(first: 100, isArchived: false, after: $after)"`
	} `graphql:"organization(login: $organization)"`
}

// Mock structure for the organizations.Get query result (needed for testing Get)
type mockOrgQueryResult struct {
	Viewer struct {
		Organizations struct {
			Nodes []struct {
				Login githubv4.String
			}
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		}
	}
}

// --- Tests for getRepos ---

func TestGetRepos_Success_SinglePage(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	org := "test-org-repo"

	// Mock response
	mockResponseData := mockRepoQueryResult{}
	mockResponseData.Organization.Repositories.Nodes = []Repository{
		{Name: "repo1", Owner: struct{ Login githubv4.String }{Login: githubv4.String(org)}},
		{Name: "repo2", Owner: struct{ Login githubv4.String }{Login: githubv4.String(org)}},
	}
	mockResponseData.Organization.Repositories.PageInfo.HasNextPage = false

	mockClient.SetResponse(mockResponseData)
	mockClient.ExpectedVariables = map[string]interface{}{
		"organization": githubv4.String(org),
		"after":        (*githubv4.String)(nil),
	}

	// Call getRepos
	repos, err := getRepos(mockClient, org)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if mockClient.QueryCallCount != 1 {
		t.Errorf("Expected Query to be called 1 time, got %d", mockClient.QueryCallCount)
	}
	if len(repos) != 2 {
		t.Errorf("Expected 2 repos, got %d", len(repos))
	}
	// Add more specific checks if needed
	if repos[0].Name != "repo1" || repos[1].Name != "repo2" {
		t.Errorf("Unexpected repo names: %v", repos)
	}
}

func TestGetRepos_Success_Paginated(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	org := "test-org-paginate"

	// Mock Page 1 Response
	mockResponse1 := mockRepoQueryResult{}
	mockResponse1.Organization.Repositories.Nodes = []Repository{
		{Name: "repo-p1", Owner: struct{ Login githubv4.String }{Login: githubv4.String(org)}},
	}
	mockResponse1.Organization.Repositories.PageInfo.HasNextPage = true
	mockResponse1.Organization.Repositories.PageInfo.EndCursor = "REPO_CURSOR1"

	// Mock Page 2 Response
	mockResponse2 := mockRepoQueryResult{}
	mockResponse2.Organization.Repositories.Nodes = []Repository{
		{Name: "repo-p2", Owner: struct{ Login githubv4.String }{Login: githubv4.String(org)}},
	}
	mockResponse2.Organization.Repositories.PageInfo.HasNextPage = false // Last page

	// Use QueryFunc for pagination
	callCount := 0
	mockClient.QueryFunc = func(ctx context.Context, q interface{}, variables map[string]interface{}) error {
		callCount++
		var responseToUse interface{}

		if callCount == 1 { // First call
			responseToUse = mockResponse1
		} else if callCount == 2 { // Second call
			responseToUse = mockResponse2
		} else {
			return fmt.Errorf("unexpected query call number %d", callCount)
		}

		// No cursor check inside mock - rely on final assertion

		respBytes, _ := json.Marshal(responseToUse)
		err := json.Unmarshal(respBytes, q)
		if err != nil {
			t.Fatalf("Failed to unmarshal mock response in call %d: %v", callCount, err)
		}
		return nil
	}

	// Call getRepos
	repos, err := getRepos(mockClient, org)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected Query to be called 2 times for pagination, got %d", callCount)
	}
	if len(repos) != 2 {
		t.Errorf("Expected 2 repos after pagination, got %d", len(repos))
	}
	if repos[0].Name != "repo-p1" || repos[1].Name != "repo-p2" {
		t.Errorf("Unexpected repo names after pagination: %v", repos)
	}
}

func TestGetRepos_Error(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	org := "test-org-repo-err"
	expectedError := fmt.Errorf("getRepos API error")

	mockClient.SetError(expectedError)
	mockClient.ExpectedVariables = map[string]interface{}{
		"organization": githubv4.String(org),
		"after":        (*githubv4.String)(nil),
	}

	// Call getRepos
	repos, err := getRepos(mockClient, org)

	// Assertions
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
	if repos != nil {
		t.Errorf("Expected nil repos on error, got %v", repos)
	}
	if mockClient.QueryCallCount != 1 { // No retries implemented in getRepos
		t.Errorf("Expected Query to be called 1 time, got %d", mockClient.QueryCallCount)
	}
}

// --- Tests for Get (integration of organizations.Get and getRepos) ---

func TestGetRepositories_Success(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}

	// --- Mock Response for organizations.Get ---
	mockOrgResponse := mockOrgQueryResult{}
	mockOrgResponse.Viewer.Organizations.Nodes = []struct{ Login githubv4.String }{{Login: "org1"}, {Login: "org2"}}

	// --- Mock Response for getRepos("org1") ---
	mockRepoResponse1 := mockRepoQueryResult{}
	mockRepoResponse1.Organization.Repositories.Nodes = []Repository{{Name: "repo1a"}, {Name: "repo1b"}}
	mockRepoResponse1.Organization.Repositories.PageInfo.HasNextPage = false

	// --- Mock Response for getRepos("org2") ---
	mockRepoResponse2 := mockRepoQueryResult{}
	mockRepoResponse2.Organization.Repositories.Nodes = []Repository{{Name: "repo2a"}}
	mockRepoResponse2.Organization.Repositories.PageInfo.HasNextPage = false

	// Use QueryFunc to differentiate calls
	callCount := 0
	mockClient.QueryFunc = func(ctx context.Context, q interface{}, variables map[string]interface{}) error {
		callCount++
		var responseToUse interface{}

		// Differentiate calls based on callCount only, removing type switch
		if callCount == 1 { // organizations.Get call
			responseToUse = mockOrgResponse
		} else if callCount == 2 { // getRepos("org1") call
			if variables["organization"] != githubv4.String("org1") {
				t.Errorf("Call 2: Expected org 'org1', got %v", variables["organization"])
			}
			responseToUse = mockRepoResponse1
		} else if callCount == 3 { // getRepos("org2") call
			if variables["organization"] != githubv4.String("org2") {
				t.Errorf("Call 3: Expected org 'org2', got %v", variables["organization"])
			}
			responseToUse = mockRepoResponse2
		} else {
			return fmt.Errorf("unexpected query call number %d", callCount)
		}

		respBytes, _ := json.Marshal(responseToUse)
		err := json.Unmarshal(respBytes, q)
		if err != nil {
			t.Fatalf("Failed to unmarshal mock response in call %d: %v", callCount, err)
		}
		return nil
	}

	// Call Get (the main function integrating the others)
	allRepos, err := Get(mockClient)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if callCount != 3 { // 1 org + 2 repo calls
		t.Errorf("Expected Query to be called 3 times, got %d", callCount)
	}
	if len(allRepos) != 3 { // 2 from org1 + 1 from org2
		t.Errorf("Expected 3 repos in total, got %d", len(allRepos))
	}
	// Basic check for content
	if allRepos[0].Name != "repo1a" || allRepos[1].Name != "repo1b" || allRepos[2].Name != "repo2a" {
		t.Errorf("Unexpected repo names: %v", allRepos)
	}

}

func TestGetRepositories_OrgGetError(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	expectedError := fmt.Errorf("Get orgs failed")

	// Set mock to fail on the first call (organizations.Get)
	mockClient.SetError(expectedError)

	// Call Get
	allRepos, err := Get(mockClient)

	// Assertions
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
	if allRepos != nil {
		t.Errorf("Expected nil repos on error, got %v", allRepos)
	}
	if mockClient.QueryCallCount != 1 {
		t.Errorf("Expected Query to be called 1 time, got %d", mockClient.QueryCallCount)
	}
}

func TestGetRepositories_GetReposError(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	expectedRepoError := fmt.Errorf("getRepos failed")

	// --- Mock Response for organizations.Get (Success) ---
	mockOrgResponse := mockOrgQueryResult{}
	mockOrgResponse.Viewer.Organizations.Nodes = []struct{ Login githubv4.String }{{Login: "org1-repo-fail"}}

	// Use QueryFunc
	callCount := 0
	mockClient.QueryFunc = func(ctx context.Context, q interface{}, variables map[string]interface{}) error {
		callCount++
		switch callCount {
		case 1: // organizations.Get - Succeeds
			respBytes, _ := json.Marshal(mockOrgResponse)
			json.Unmarshal(respBytes, q)
			return nil
		case 2: // getRepos("org1-repo-fail") - Fails
			return expectedRepoError
		default:
			return fmt.Errorf("unexpected query call number %d", callCount)
		}
	}

	// Call Get
	allRepos, err := Get(mockClient)

	// Assertions
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != expectedRepoError.Error() { // Error should propagate from getRepos
		t.Errorf("Expected error '%v', got '%v'", expectedRepoError, err)
	}
	if allRepos != nil {
		t.Errorf("Expected nil repos on error, got %v", allRepos)
	}
	if callCount != 2 { // 1 org + 1 failed repo call
		t.Errorf("Expected Query to be called 2 times, got %d", callCount)
	}
}
