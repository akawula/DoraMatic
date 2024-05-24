package organizations

import (
	"context"
	"os"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

var q struct {
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

func Get() ([]string, error) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)

	err := client.Query(context.Background(), &q, nil)
	if err != nil {
		return nil, err
	}

	results := []string{}
	for _, login := range q.Viewer.Organizations.Nodes {
		results = append(results, string(login.Login))
	}

	return results, nil
}
