package pullrequests

import (
	"context"
	"log/slog"
	"time"

	"github.com/akawula/DoraMatic/github/client" // Use the client package for the interface
	"github.com/akawula/DoraMatic/github/repositories"
	"github.com/shurcooL/githubv4"
)

type Commit struct {
	Id     githubv4.String
	Commit struct {
		Message githubv4.String
	}
}

// Review holds information about a single pull request review.
type Review struct {
	Id          githubv4.String
	State       githubv4.String // e.g., APPROVED, CHANGES_REQUESTED, COMMENTED
	SubmittedAt githubv4.String
	Author      struct {
		Login githubv4.String
	}
	Body githubv4.String
	Url  githubv4.String
}

type PullRequest struct {
	Id          githubv4.String
	Title       githubv4.String
	State       githubv4.String
	Url         githubv4.String
	MergedAt    githubv4.String
	CreatedAt   githubv4.String
	Additions   githubv4.Int
	Deletions   githubv4.Int
	HeadRefName githubv4.String
	Author      struct {
		AvatarUrl githubv4.String
		Login     githubv4.String
	}
	Repository repositories.Repository
	Commits    struct {
		Nodes      []Commit
		TotalCount githubv4.Int
	} `graphql:"commits(first: 50)"`
	TimelineItems struct {
		Nodes []struct {
			ReviewRequestedEventFragment struct {
				CreatedAt githubv4.String
			} `graphql:"... on ReviewRequestedEvent"`
		}
		TotalCount githubv4.Int
	} `graphql:"timelineItems(itemTypes: REVIEW_REQUESTED_EVENT, first: 1)"`
	Reviews struct {
		Nodes    []Review
		PageInfo struct {
			HasNextPage githubv4.Boolean
			EndCursor   githubv4.String
		}
		TotalCount githubv4.Int
	} `graphql:"reviews(first: 50)"` // Fetch first 50 reviews
}

// Get fetches pull requests for a repository, using the provided GitHubV4Client.
func Get(ghClient client.GitHubV4Client, org string, repo string, lastDBDate time.Time, logger *slog.Logger) ([]PullRequest, error) {
	var q struct {
		Repository struct {
			PullRequests struct {
				Nodes    []PullRequest // Review data is now fetched within PullRequest struct
				PageInfo struct {
					HasNextPage githubv4.Boolean
					EndCursor   githubv4.String
				}
			} `graphql:"pullRequests(first:30, orderBy: {field: CREATED_AT, direction: DESC}, states: [MERGED, OPEN], after: $after)"`
		} `graphql:"repository(name: $name, owner: $login)"`
	}

	// ghClient := client.Get() // Removed: Use the passed-in ghClient
	// Add reviewsAfter variable, initially nil. Note: This simple implementation doesn't handle review pagination within a single PR if > 50 reviews.
	variables := map[string]interface{}{
		"login":        githubv4.String(org),
		"name":  githubv4.String(repo),
		"after": (*githubv4.String)(nil), // For PR pagination
	}
	logger.Debug("Will do the pull request query with params", "variables", variables)
	results := []PullRequest{}
	retries := 3
	for {
		// Use the passed-in ghClient
		err := ghClient.Query(context.Background(), &q, variables)
		if err != nil {
			if retries > 0 {
				logger.Debug("Retrying fetching pull requests", "org", org, "repo", repo, "retries", retries, "error", err)
				retries--
				continue
			}
			return nil, err
		}
		results = append(results, q.Repository.PullRequests.Nodes...)
		if older := len(results) > 0 && checkDates(lastDBDate, results[len(results)-1].CreatedAt); older || !bool(q.Repository.PullRequests.PageInfo.HasNextPage) {
			break
		}
		variables["after"] = githubv4.String(q.Repository.PullRequests.PageInfo.EndCursor)
	}

	return results, nil
}

func checkDates(lastDbDate time.Time, ghDate githubv4.String) bool {
	r, err := time.Parse(time.RFC3339, string(ghDate))
	if err != nil {
		slog.Error("Parse error", "error", err)
	}

	return r.Before(lastDbDate)
}
