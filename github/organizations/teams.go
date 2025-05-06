package organizations

import (
	"context"

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
							Login     githubv4.String
							AvatarUrl githubv4.String // Added AvatarUrl field
						}
						PageInfo struct {
							HasNextPage githubv4.Boolean
							EndCursor   githubv4.String
						}
						// Corrected GraphQL query tag - fields are inferred from struct
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

// MemberInfo holds login and avatar URL.
type MemberInfo struct {
	Login     string
	AvatarUrl string
}

// GetTeams fetches teams and members (including avatar URL) for all organizations.
// Return type changed to map[string][]MemberInfo.
func GetTeams(ghClient client.GitHubV4Client) (map[string][]MemberInfo, error) {
	// Pass ghClient to organizations.Get
	orgs, err := Get(ghClient)
	if err != nil {
		return nil, err
	}

	results := make(map[string][]MemberInfo)
	for _, org := range orgs {
		// Pass ghClient to getTeam
		teamsInOrg, err := getTeam(ghClient, org) // getTeam returns map[string][]MemberInfo for a single org
		if err != nil {
			// TODO: Consider logging and continuing, or returning partial results.
			// For now, maintaining fail-fast behavior from original logic.
			return nil, err
		}
		// Use a qualified team name (org/team) to prevent collisions between orgs
		for teamName, members := range teamsInOrg {
			qualifiedTeamName := org + "/" + teamName
			results[qualifiedTeamName] = members
		}
	}

	return results, nil
}

// getTeam fetches teams and members (including avatar URL) for a specific organization.
// Return type changed to map[string][]MemberInfo.
func getTeam(ghClient client.GitHubV4Client, org string) (map[string][]MemberInfo, error) {
	// Removed internal client creation, use ghClient
	variables := map[string]interface{}{"organization": githubv4.String(org), "teamsAfter": (*githubv4.String)(nil), "membersAfter": (*githubv4.String)(nil)}
	retries := 3
	results := make(map[string][]MemberInfo) // Changed value type

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

		teamName := string(teamQuery.Viewer.Organization.Teams.Nodes[0].Name) // Cast to string once
		for _, m := range teamQuery.Viewer.Organization.Teams.Nodes[0].Members.Nodes {
			// Create MemberInfo struct and append
			memberInfo := MemberInfo{
				Login:     string(m.Login),
				AvatarUrl: string(m.AvatarUrl), // Add avatar url
			}
			results[teamName] = append(results[teamName], memberInfo)
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
