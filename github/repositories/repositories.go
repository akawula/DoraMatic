package repositories

import (
	"context"

	"github.com/akawula/DoraMatic/github/client" // Import client package
	"github.com/akawula/DoraMatic/github/organizations"
	"github.com/shurcooL/githubv4"
)

type Repository struct {
	Name            githubv4.String
	PrimaryLanguage struct {
		Name githubv4.String
	}
	Owner struct {
		Login githubv4.String
	}
}

// Get fetches repositories for all organizations using the provided GitHubV4Client.
func Get(ghClient client.GitHubV4Client) ([]Repository, error) {
	// Pass ghClient to organizations.Get
	orgs, err := organizations.Get(ghClient)
	if err != nil {
		return nil, err
	}

	r := []Repository{}
	for _, org := range orgs {
		// Pass ghClient to getRepos
		repos, err := getRepos(ghClient, org)
		if err != nil {
			return nil, err
		}
		r = append(r, repos...)
	}

	return r, nil
}

// getRepos fetches repositories for a specific organization using the provided GitHubV4Client.
func getRepos(ghClient client.GitHubV4Client, org string) ([]Repository, error) {
	var repoQuery struct { // Renamed query variable
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

	// Removed internal client creation, use ghClient
	variables := map[string]interface{}{"organization": githubv4.String(org), "after": (*githubv4.String)(nil)}
	results := []Repository{}
	for {
		// Use ghClient and the renamed repoQuery
		err := ghClient.Query(context.Background(), &repoQuery, variables)
		if err != nil {
			return nil, err
		}
		results = append(results, repoQuery.Organization.Repositories.Nodes...)
		if !repoQuery.Organization.Repositories.PageInfo.HasNextPage {
			break
		}
		variables["after"] = repoQuery.Organization.Repositories.PageInfo.EndCursor
	}

	return results, nil
}
