package organizations

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect" // Re-add reflect
	"testing"

	"github.com/akawula/DoraMatic/github/client" // Import client package for mock
	"github.com/shurcooL/githubv4"
)

// --- Mock Structures ---

// Mock response structure for the Get function (needed by GetTeams)
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

// Mock response structure for the getTeam function's query
type mockTeamQueryResult struct {
	Viewer struct {
		Organization struct {
			Teams struct {
				Nodes []struct {
					Name    githubv4.String
					Members struct {
						Nodes []struct {
							Login githubv4.String
						}
						PageInfo struct {
							HasNextPage githubv4.Boolean
							EndCursor   githubv4.String
						}
					}
				}
				PageInfo struct {
					HasNextPage githubv4.Boolean
					EndCursor   githubv4.String
				}
			}
		}
	}
}

// --- Tests ---

func TestGetTeam_Success_SinglePage(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	org := "test-single-org"

	// Mock response for the single team and its members
	mockTeamResponse := mockTeamQueryResult{}
	mockTeamResponse.Viewer.Organization.Teams.Nodes = []struct {
		Name    githubv4.String
		Members struct {
			Nodes []struct {
				Login githubv4.String
			}
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		}
	}{
		{
			Name: "team-alpha",
			Members: struct {
				Nodes []struct {
					Login githubv4.String
				}
				PageInfo struct {
					HasNextPage githubv4.Boolean
					EndCursor   githubv4.String
				}
			}{
				Nodes: []struct{ Login githubv4.String }{
					{Login: "user1"},
					{Login: "user2"},
				},
				PageInfo: struct {
					HasNextPage githubv4.Boolean
					EndCursor   githubv4.String
				}{HasNextPage: false}, // No member pagination
			},
		},
	}
	mockTeamResponse.Viewer.Organization.Teams.PageInfo.HasNextPage = false // No team pagination

	mockClient.SetResponse(mockTeamResponse)
	mockClient.ExpectedVariables = map[string]interface{}{
		"organization": githubv4.String(org),
		"teamsAfter":   (*githubv4.String)(nil),
		"membersAfter": (*githubv4.String)(nil),
	}

	// Call getTeam
	teamMap, err := getTeam(mockClient, org)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if mockClient.QueryCallCount != 1 {
		t.Errorf("Expected Query to be called 1 time, got %d", mockClient.QueryCallCount)
	}

	expectedMap := map[string][]string{
		"team-alpha": {"user1", "user2"},
	}
	if !reflect.DeepEqual(teamMap, expectedMap) {
		t.Errorf("Expected team map %v, got %v", expectedMap, teamMap)
	}
}

func TestGetTeam_Success_PaginatedMembers(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	org := "test-paginate-members"

	// --- Mock Response Page 1 (Members) ---
	mockTeamResponse1 := mockTeamQueryResult{}
	mockTeamResponse1.Viewer.Organization.Teams.Nodes = []struct {
		Name    githubv4.String
		Members struct {
			Nodes    []struct{ Login githubv4.String }
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		}
	}{
		{Name: "team-beta", Members: struct {
			Nodes    []struct{ Login githubv4.String }
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		}{
			Nodes: []struct{ Login githubv4.String }{{Login: "user-m1"}},
			PageInfo: struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}{HasNextPage: true, EndCursor: "MEMCURSOR1"},
		}},
	}
	mockTeamResponse1.Viewer.Organization.Teams.PageInfo.HasNextPage = false // Only one team

	// --- Mock Response Page 2 (Members) ---
	mockTeamResponse2 := mockTeamQueryResult{}
	// Need to return the same team structure, just different members
	mockTeamResponse2.Viewer.Organization.Teams.Nodes = []struct {
		Name    githubv4.String
		Members struct {
			Nodes    []struct{ Login githubv4.String }
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		}
	}{
		{Name: "team-beta", Members: struct {
			Nodes    []struct{ Login githubv4.String }
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		}{
			Nodes: []struct{ Login githubv4.String }{{Login: "user-m2"}},
			PageInfo: struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}{HasNextPage: false}, // Last member page
		}},
	}
	mockTeamResponse2.Viewer.Organization.Teams.PageInfo.HasNextPage = false

	// Use QueryFunc for pagination logic
	callCount := 0
	mockClient.QueryFunc = func(ctx context.Context, q interface{}, variables map[string]interface{}) error {
		callCount++
		var responseToUse interface{}

		if callCount == 1 { // First call, no cursors
			responseToUse = mockTeamResponse1
		} else if callCount == 2 { // Second call, expect member cursor
			responseToUse = mockTeamResponse2
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

	// Call getTeam
	teamMap, err := getTeam(mockClient, org)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected Query to be called 2 times for member pagination, got %d", callCount)
	}
	expectedMap := map[string][]string{
		"team-beta": {"user-m1", "user-m2"},
	}
	// Check map content carefully as order might vary in mock setup vs result concatenation
	if len(teamMap) != 1 || len(teamMap["team-beta"]) != 2 || !reflect.DeepEqual(teamMap, expectedMap) {
		// Basic check first
		if !maps.EqualFunc(teamMap, expectedMap, func(s1, s2 []string) bool { return reflect.DeepEqual(s1, s2) }) {
			t.Errorf("Expected team map %v, got %v", expectedMap, teamMap)
		}
	}
}

// TODO: Add test for paginated teams (similar logic, adjusting mock responses and cursors)
// TODO: Add test for combined team and member pagination.

func TestGetTeam_Error(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	org := "test-org-error"
	expectedError := fmt.Errorf("getTeam API error")

	mockClient.SetError(expectedError) // Error returned on first query

	// Call getTeam
	teamMap, err := getTeam(mockClient, org)

	// Assertions
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	// The function has retries, so check the count
	// Retries = 3, Initial = 1 -> 4 calls total
	if mockClient.QueryCallCount != 4 {
		t.Errorf("Expected Query to be called 4 times on error with retries, got %d", mockClient.QueryCallCount)
	}
	if teamMap != nil {
		t.Errorf("Expected nil map on error, got %v", teamMap)
	}
	// Check if the final error matches
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

func TestGetTeams_Success(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}

	// --- Mock Response for organizations.Get ---
	mockOrgResponse := mockOrgQueryResult{}
	mockOrgResponse.Viewer.Organizations.Nodes = []struct{ Login githubv4.String }{{Login: "org1"}}

	// --- Mock Response for getTeam("org1") ---
	mockTeamResponse := mockTeamQueryResult{}
	mockTeamResponse.Viewer.Organization.Teams.Nodes = []struct {
		Name    githubv4.String
		Members struct {
			Nodes    []struct{ Login githubv4.String }
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		}
	}{
		{Name: "team-final", Members: struct {
			Nodes    []struct{ Login githubv4.String }
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		}{
			Nodes: []struct{ Login githubv4.String }{{Login: "user-final"}},
			PageInfo: struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}{HasNextPage: false},
		}},
	}
	mockTeamResponse.Viewer.Organization.Teams.PageInfo.HasNextPage = false

	// Use QueryFunc to differentiate between org query and team query
	callCount := 0
	mockClient.QueryFunc = func(ctx context.Context, q interface{}, variables map[string]interface{}) error {
		callCount++
		var responseToUse interface{}

		// Differentiate calls based on callCount only
		if callCount == 1 { // organizations.Get call
			responseToUse = mockOrgResponse
		} else if callCount == 2 { // getTeam("org1") call
			if variables["organization"] != githubv4.String("org1") {
				t.Errorf("Expected org 'org1', got %v", variables["organization"])
			}
			responseToUse = mockTeamResponse
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

	// Call GetTeams
	teamsMap, err := GetTeams(mockClient)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if callCount != 2 { // 1 for orgs, 1 for teams in org1
		t.Errorf("Expected Query to be called 2 times, got %d", callCount)
	}
	expectedMap := map[string][]string{
		"team-final": {"user-final"},
	}
	if !maps.EqualFunc(teamsMap, expectedMap, func(s1, s2 []string) bool {
		// Simple comparison assuming order doesn't matter for this small example
		if len(s1) != len(s2) {
			return false
		}
		if len(s1) == 0 {
			return true
		} // Both empty
		return s1[0] == s2[0] // Adjust if more complex comparison needed
	}) {
		t.Errorf("Expected final teams map %v, got %v", expectedMap, teamsMap)
	}
}

func TestGetTeams_OrgError(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	expectedError := fmt.Errorf("Get orgs failed")

	// Set mock to fail on the first call (organizations.Get)
	mockClient.SetError(expectedError)

	// Call GetTeams
	teamsMap, err := GetTeams(mockClient)

	// Assertions
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != expectedError.Error() { // Error should propagate from organizations.Get
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
	if teamsMap != nil {
		t.Errorf("Expected nil map on error, got %v", teamsMap)
	}
	if mockClient.QueryCallCount != 1 {
		t.Errorf("Expected Query to be called 1 time, got %d", mockClient.QueryCallCount)
	}
}

func TestGetTeams_GetTeamError(t *testing.T) {
	mockClient := &client.MockGitHubV4Client{T: t}
	expectedTeamError := fmt.Errorf("getTeam failed")

	// --- Mock Response for organizations.Get (Success) ---
	mockOrgResponse := mockOrgQueryResult{}
	mockOrgResponse.Viewer.Organizations.Nodes = []struct{ Login githubv4.String }{{Login: "org1-fail"}}

	// Use QueryFunc to differentiate calls
	callCount := 0
	mockClient.QueryFunc = func(ctx context.Context, q interface{}, variables map[string]interface{}) error {
		callCount++
		// Differentiate calls based on callCount only
		if callCount == 1 { // organizations.Get call - Succeeds
			respBytes, _ := json.Marshal(mockOrgResponse)
			err := json.Unmarshal(respBytes, q)
			if err != nil {
				t.Fatalf("Unmarshal failed for org query: %v", err)
			}
			return nil
		} else if callCount >= 2 && callCount <= 5 { // getTeam calls - Fail (1 initial + 3 retries)
			return expectedTeamError
		} else {
			return fmt.Errorf("unexpected query call number %d", callCount)
		}

	}

	// Call GetTeams
	teamsMap, err := GetTeams(mockClient)

	// Assertions
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != expectedTeamError.Error() { // Error should propagate from getTeam
		t.Errorf("Expected error '%v', got '%v'", expectedTeamError, err)
	}
	if teamsMap != nil {
		t.Errorf("Expected nil map on error, got %v", teamsMap)
	}
	if callCount != 5 { // 1 org + 4 team attempts (1 initial + 3 retries)
		t.Errorf("Expected Query to be called 5 times, got %d", callCount)
	}
}
