package organizations

import (
	"fmt"
	"reflect" // Needed for DeepEqual
	"testing"

	"github.com/akawula/DoraMatic/github/client" // Import client package for mock
	"github.com/shurcooL/githubv4"
)

// --- Tests ---

func TestGetOrganizations_Success(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}

	// Prepare mock response
	mockResponseData := struct {
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
	}{}
	mockResponseData.Viewer.Organizations.Nodes = []struct {
		Login githubv4.String
	}{
		{Login: "org1"},
		{Login: "org2"},
	}
	// Assuming single page for simplicity here
	mockResponseData.Viewer.Organizations.PageInfo.HasNextPage = false

	mockClient.SetResponse(mockResponseData)

	// Call the function
	orgs, err := Get(mockClient)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if mockClient.QueryCallCount != 1 {
		t.Errorf("Expected Query to be called 1 time, got %d", mockClient.QueryCallCount)
	}
	expectedOrgs := []string{"org1", "org2"}
	if !reflect.DeepEqual(orgs, expectedOrgs) {
		t.Errorf("Expected orgs %v, got %v", expectedOrgs, orgs)
	}
}

func TestGetOrganizations_Error(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	expectedError := fmt.Errorf("GitHub API error for orgs")

	mockClient.SetError(expectedError)

	// Call the function
	orgs, err := Get(mockClient)

	// Assertions
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
	if orgs != nil {
		t.Errorf("Expected nil orgs on error, got %v", orgs)
	}
	if mockClient.QueryCallCount != 1 {
		t.Errorf("Expected Query to be called 1 time, got %d", mockClient.QueryCallCount)
	}
}

// Note: Pagination tests for Get could be added similarly to pull requests if needed,
// but the current implementation fetches all orgs in one go (up to 100).
