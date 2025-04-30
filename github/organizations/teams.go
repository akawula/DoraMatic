package organizations

import (
	"context"
	"maps"

	"github.com/akawula/DoraMatic/github/client" // Import client package
	"github.com/shurcooL/githubv4"
)

var teamQuery struct { // Renamed query variable to avoid conflict
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

// GetTeams fetches teams for all organizations using the provided GitHubV4Client.
func GetTeams(ghClient client.GitHubV4Client) (map[string][]string, error) {
	// Pass ghClient to organizations.Get
	orgs, err := Get(ghClient)
	if err != nil {
		return nil, err
	}

	results := map[string][]string{}
	for _, org := range orgs {
		// Pass ghClient to getTeam
		team, err := getTeam(ghClient, org)
		if err != nil {
			return nil, err
		}
		maps.Copy(results, team)
	}

	return results, nil
}

// getTeam fetches teams for a specific organization using the provided GitHubV4Client.
func getTeam(ghClient client.GitHubV4Client, org string) (map[string][]string, error) {
	// Removed internal client creation, use ghClient
	variables := map[string]interface{}{"organization": githubv4.String(org), "teamsAfter": (*githubv4.String)(nil), "membersAfter": (*githubv4.String)(nil)}
	retries := 3
	results := make(map[string][]string)

	for {
		// Use ghClient and the renamed teamQuery
		err := ghClient.Query(context.Background(), &teamQuery, variables)
		if err != nil {
			if retries > 0 {
				retries-- // TODO: Add exponential backoff or delay
				continue
			}
			return nil, err
		}

		// Use renamed teamQuery
		if len(teamQuery.Viewer.Organization.Teams.Nodes) == 0 {
			break
		}

		teamName := teamQuery.Viewer.Organization.Teams.Nodes[0].Name
		for _, m := range teamQuery.Viewer.Organization.Teams.Nodes[0].Members.Nodes {
			results[string(teamName)] = append(results[string(teamName)], string(m.Login))
		}

		if teamQuery.Viewer.Organization.Teams.Nodes[0].Members.PageInfo.HasNextPage {
			variables["membersAfter"] = teamQuery.Viewer.Organization.Teams.Nodes[0].Members.PageInfo.EndCursor
		} else if teamQuery.Viewer.Organization.Teams.PageInfo.HasNextPage {
			variables["membersAfter"] = (*githubv4.String)(nil)
			variables["teamsAfter"] = teamQuery.Viewer.Organization.Teams.PageInfo.EndCursor
		} else {
			break
		}
	}

	return results, nil
}
