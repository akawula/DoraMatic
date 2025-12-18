package codeowners

import (
	"bufio"
	"context"
	"log/slog"
	"strings"

	"github.com/akawula/DoraMatic/github/client"
	"github.com/shurcooL/githubv4"
)

// RepositoryOwnership represents the teams that own a repository based on CODEOWNERS
type RepositoryOwnership struct {
	Org      string
	RepoSlug string
	Teams    []string // GitHub team slugs, e.g., "wpengine/plutus"
}

// codeownersQuery is the GraphQL query to fetch CODEOWNERS file content
type codeownersQuery struct {
	Repository struct {
		Object struct {
			Blob struct {
				Text githubv4.String
			} `graphql:"... on Blob"`
		} `graphql:"object(expression: \"HEAD:.github/CODEOWNERS\")"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

// FetchCodeowners fetches the CODEOWNERS file from a repository and extracts team slugs
func FetchCodeowners(ghClient client.GitHubV4Client, org, repoSlug string, logger *slog.Logger) (*RepositoryOwnership, error) {
	var query codeownersQuery

	variables := map[string]interface{}{
		"owner": githubv4.String(org),
		"name":  githubv4.String(repoSlug),
	}

	err := ghClient.Query(context.Background(), &query, variables)
	if err != nil {
		logger.Debug("failed to fetch CODEOWNERS", "org", org, "repo", repoSlug, "error", err)
		return nil, err
	}

	content := string(query.Repository.Object.Blob.Text)
	if content == "" {
		logger.Debug("no CODEOWNERS file found", "org", org, "repo", repoSlug)
		return &RepositoryOwnership{
			Org:      org,
			RepoSlug: repoSlug,
			Teams:    []string{},
		}, nil
	}

	teams := parseCodeowners(content)
	logger.Debug("parsed CODEOWNERS", "org", org, "repo", repoSlug, "teams", teams)

	return &RepositoryOwnership{
		Org:      org,
		RepoSlug: repoSlug,
		Teams:    teams,
	}, nil
}

// parseCodeowners parses CODEOWNERS file content and extracts unique team slugs
// Team slugs are in the format @org/team-name (contains a /)
// Individual users are in the format @username (no /)
func parseCodeowners(content string) []string {
	teamSet := make(map[string]struct{})

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split line into parts (pattern + owners)
		parts := strings.Fields(line)
		if len(parts) < 2 {
			// Lines with no owners (like "docs/") - skip
			continue
		}

		// Extract owners (everything after the pattern)
		for _, owner := range parts[1:] {
			// Skip if not starting with @
			if !strings.HasPrefix(owner, "@") {
				continue
			}

			// Remove the @ prefix
			owner = strings.TrimPrefix(owner, "@")

			// Check if it's a team (contains /) vs individual user
			if strings.Contains(owner, "/") {
				// Store lowercase to match github_team_slug in teams table
				teamSet[strings.ToLower(owner)] = struct{}{}
			}
		}
	}

	// Convert set to slice
	teams := make([]string, 0, len(teamSet))
	for team := range teamSet {
		teams = append(teams, team)
	}

	return teams
}

// FetchAllCodeowners fetches CODEOWNERS for multiple repositories
func FetchAllCodeowners(ghClient client.GitHubV4Client, repos []struct{ Org, Slug string }, logger *slog.Logger) ([]RepositoryOwnership, error) {
	var results []RepositoryOwnership

	for _, repo := range repos {
		ownership, err := FetchCodeowners(ghClient, repo.Org, repo.Slug, logger)
		if err != nil {
			// Log error but continue with other repos
			logger.Warn("failed to fetch CODEOWNERS for repo", "org", repo.Org, "repo", repo.Slug, "error", err)
			continue
		}
		if len(ownership.Teams) > 0 {
			results = append(results, *ownership)
		}
	}

	return results, nil
}
