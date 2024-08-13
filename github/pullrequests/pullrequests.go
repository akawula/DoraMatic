package pullrequests

import (
	"context"
	"log/slog"
	"time"

	"github.com/akawula/DoraMatic/github/client"
	"github.com/akawula/DoraMatic/github/repositories"
	"github.com/shurcooL/githubv4"
)

type Commit struct {
	Id     githubv4.String
	Commit struct {
		Message githubv4.String
	}
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
}

func Get(org string, repo string, lastDBDate time.Time, logger *slog.Logger) ([]PullRequest, error) {
	var q struct {
		Repository struct {
			PullRequests struct {
				Nodes    []PullRequest
				PageInfo struct {
					HasNextPage githubv4.Boolean
					EndCursor   githubv4.String
				}
			} `graphql:"pullRequests(first:30, orderBy: {field: CREATED_AT, direction: DESC}, states: [MERGED], after: $after)"`
		} `graphql:"repository(name: $name, owner: $login)"`
	}

	client := client.Get()
	variables := map[string]interface{}{"login": githubv4.String(org), "name": githubv4.String(repo), "after": (*githubv4.String)(nil)}
	logger.Debug("Will do the pull reuqest query with params", "variables", variables)
	results := []PullRequest{}
	retries := 3
	for {
		err := client.Query(context.Background(), &q, variables)
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
