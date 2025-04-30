package client

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// MockGitHubV4Client is a mock implementation of the GitHubV4Client interface for testing.
type MockGitHubV4Client struct {
	// ExpectedQuery is the GraphQL query structure the mock expects to receive.
	ExpectedQuery interface{}
	// ExpectedVariables is the map of variables the mock expects to receive.
	ExpectedVariables map[string]interface{}
	// ResponseToReturn is the data structure to be marshalled into the query result.
	ResponseToReturn interface{}
	// ErrorToReturn is the error to return when Query is called.
	ErrorToReturn error
	// QueryCallCount tracks how many times Query was called.
	QueryCallCount int
	// T is a testing object for reporting errors (*testing.T or *testing.B)
	T testingT
	// QueryFunc allows overriding Query behavior for complex mocks like pagination.
	QueryFunc func(ctx context.Context, q interface{}, variables map[string]interface{}) error
}

// testingT is an interface wrapper around *testing.T
type testingT interface {
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// Query mocks the Query method of the GitHubV4Client interface.
// It checks if the provided query structure and variables match expectations,
// calls QueryFunc if set, otherwise returns the pre-configured response or error.
func (m *MockGitHubV4Client) Query(ctx context.Context, q interface{}, variables map[string]interface{}) error {
	m.QueryCallCount++

	// Allow override func for complex scenarios
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, q, variables)
	}

	if m.ErrorToReturn != nil {
		return m.ErrorToReturn
	}

	// Optional: Check if variables match expectations (use reflect.DeepEqual for comparison)
	if m.ExpectedVariables != nil && !reflect.DeepEqual(m.ExpectedVariables, variables) {
		err := fmt.Errorf("mock Query: variables mismatch. Expected %v, Got %v", m.ExpectedVariables, variables)
		if m.T != nil {
			m.T.Errorf(err.Error()) // Report error via testing object if provided
		}
		return err // Return error on mismatch
	}

	// Optional: You could add more complex checks for the query 'q' structure if needed,
	// but often just returning the correct response structure is sufficient.

	if m.ResponseToReturn != nil {
		// Marshal the response data and unmarshal it into the query structure 'q'.
		// This simulates how the actual client populates the result.
		respBytes, err := json.Marshal(m.ResponseToReturn)
		if err != nil {
			if m.T != nil {
				m.T.Fatalf("mock Query: failed to marshal mock response: %v", err)
			}
			return fmt.Errorf("mock Query: failed to marshal mock response: %w", err)
		}

		// 'q' must be a pointer for Unmarshal to work.
		if reflect.ValueOf(q).Kind() != reflect.Ptr {
			err := fmt.Errorf("mock Query: 'q' must be a pointer, got %T", q)
			if m.T != nil {
				m.T.Errorf(err.Error())
			}
			return err
		}

		err = json.Unmarshal(respBytes, q)
		if err != nil {
			if m.T != nil {
				m.T.Fatalf("mock Query: failed to unmarshal mock response into query struct: %v", err)
			}
			return fmt.Errorf("mock Query: failed to unmarshal mock response into query struct: %w", err)
		}
	}

	return nil // No error specified, and response (if any) was set.
}

// --- Helper methods for setting up expectations (optional but convenient) ---

func (m *MockGitHubV4Client) SetResponse(resp interface{}) {
	m.ResponseToReturn = resp
	m.ErrorToReturn = nil
}

func (m *MockGitHubV4Client) SetError(err error) {
	m.ErrorToReturn = err
	m.ResponseToReturn = nil
}

func (m *MockGitHubV4Client) SetExpectations(q interface{}, vars map[string]interface{}) {
	m.ExpectedQuery = q
	m.ExpectedVariables = vars
}

func (m *MockGitHubV4Client) Reset() {
	m.ExpectedQuery = nil
	m.ExpectedVariables = nil
	m.ResponseToReturn = nil
	m.ErrorToReturn = nil
	m.QueryCallCount = 0
	// m.T = nil // Keep testing object if needed across calls in a single test
}
