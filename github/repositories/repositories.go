package repositories

import (
	"context"

	"github.com/akawula/DoraMatic/github/client"
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

func Get() ([]Repository, error) {
	orgs, err := organizations.Get()
	if err != nil {
		return nil, err
	}

	r := []Repository{}
	for _, org := range orgs {
		repos, err := getRepos(org)
		if err != nil {
			return nil, err
		}
		r = append(r, repos...)
	}

	return r, nil
}

func getRepos(org string) ([]Repository, error) {
	var q struct {
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

	client := client.Get()
	variables := map[string]interface{}{"organization": githubv4.String(org), "after": (*githubv4.String)(nil)}
	results := []Repository{}
	for {
		err := client.Query(context.Background(), &q, variables)
		if err != nil {
			return nil, err
		}
		results = append(results, q.Organization.Repositories.Nodes...)
		if !q.Organization.Repositories.PageInfo.HasNextPage {
			break
		}
		variables["after"] = githubv4.String(q.Organization.Repositories.PageInfo.EndCursor)
	}

	return results, nil
}
