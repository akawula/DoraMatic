package client

import (
	"context"
)

// GitHubV4Client defines the interface for the methods used from the githubv4 client.
// This allows for mocking the client in tests.
type GitHubV4Client interface {
	Query(ctx context.Context, q interface{}, variables map[string]interface{}) error
}
