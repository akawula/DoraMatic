package organizations

import (
	"context"
	"maps"
	"os"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

var query struct {
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
					} `graphql:"members(first:100, after: $membersAfter)"`
				}
				PageInfo struct {
					HasNextPage githubv4.Boolean
					EndCursor   githubv4.String
				}
			} `graphql:"teams(first: 1, after: $teamsAfter)"`
		} `graphql:"organization(login: $organization)"`
	}
}

func GetTeams() (map[string][]string, error) {
	orgs, err := Get()
	if err != nil {
		return nil, err
	}

	results := map[string][]string{}
	for _, org := range orgs {
		team, err := getTeam(org)
		if err != nil {
			return nil, err
		}
		maps.Copy(results, team)
	}

	return results, nil
}

func getTeam(org string) (map[string][]string, error) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)
	variables := map[string]interface{}{"organization": githubv4.String(org), "teamsAfter": (*githubv4.String)(nil), "membersAfter": (*githubv4.String)(nil)}
	retries := 3
	results := make(map[string][]string)

	for {
		err := client.Query(context.Background(), &query, variables)
		if err != nil {
			if retries > 0 {
				retries--
				continue
			}
			return nil, err
		}

		if len(query.Viewer.Organization.Teams.Nodes) == 0 {
			break
		}

		teamName := query.Viewer.Organization.Teams.Nodes[0].Name
		for _, m := range query.Viewer.Organization.Teams.Nodes[0].Members.Nodes {
			results[string(teamName)] = append(results[string(teamName)], string(m.Login))
		}

		if query.Viewer.Organization.Teams.Nodes[0].Members.PageInfo.HasNextPage {
			variables["membersAfter"] = query.Viewer.Organization.Teams.Nodes[0].Members.PageInfo.EndCursor
		} else if query.Viewer.Organization.Teams.PageInfo.HasNextPage {
			variables["membersAfter"] = (*githubv4.String)(nil)
			variables["teamsAfter"] = query.Viewer.Organization.Teams.PageInfo.EndCursor
		} else {
			break
		}
	}

	return results, nil
}
