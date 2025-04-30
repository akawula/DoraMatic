package organizations

import (
	"context"

	"github.com/akawula/DoraMatic/github/client" // Import client package
	"github.com/shurcooL/githubv4"
)

var orgQuery struct { // Renamed query variable to avoid conflict with other files
	Viewer struct {
		Organizations struct {
			Nodes []struct {
				Login githubv4.String
			}
			PageInfo struct {
				HasNextPage githubv4.Boolean
				EndCursor   githubv4.String
			}
		} `graphql:"organizations(first:100)"`
	}
}

// Get fetches organization logins using the provided GitHubV4Client.
func Get(ghClient client.GitHubV4Client) ([]string, error) {
	// client := githubv4.NewClient(httpClient) // Removed: Use passed-in ghClient

	// Use the renamed query variable 'orgQuery'
	err := ghClient.Query(context.Background(), &orgQuery, nil)
	if err != nil {
		return nil, err
	}

	results := []string{}
	for _, login := range orgQuery.Viewer.Organizations.Nodes {
		results = append(results, string(login.Login))
	}

	return results, nil
}
