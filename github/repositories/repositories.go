package repositories

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/akawula/DoraMatic/github/organizations"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
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
	Repository Repository
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
	} `graphql:"timelineItems(itemTypes: REVIEW_REQUESTED_EVENT, first: 1)"`
}

type RepositoryPRs struct {
	org              string
	slug             string
	logger           *slog.Logger
	client           *githubv4.Client
	retries          int
	results          []PullRequest
	variables        map[string]interface{}
	executeDuration  time.Duration
	databaseDuration time.Duration
}

type query struct {
	Organization struct {
		Repository struct {
			PullRequests struct {
				Nodes    []PullRequest
				PageInfo struct {
					HasNextPage githubv4.Boolean
					EndCursor   githubv4.String
				}
			} `graphql:"pullRequests(first:50, orderBy: {field: CREATED_AT, direction: DESC}, states: [MERGED], after: $after)"`
		} `graphql:"repository(name: $name)"`
	} `graphql:"organization(login: $login)"`
}

func New(logger *slog.Logger, org string, slug string) RepositoryPRs {
	return RepositoryPRs{
		org:     org,
		slug:    slug,
		logger:  logger.With("org", org, "slug", slug),
		retries: 3,
	}
}

func (r *RepositoryPRs) init() {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	r.client = githubv4.NewClient(httpClient)

	r.variables = map[string]interface{}{
		"name":  githubv4.String(r.slug),
		"login": githubv4.String(r.org),
		"after": (*githubv4.String)(nil),
	}
	r.results = []PullRequest{}
}

func (r *RepositoryPRs) Process(db *sql.DB, jobs chan string, wg *sync.WaitGroup) {
	r.logger.Debug("Initializing....")
	r.init()
	err := r.execute(r.prepareQuery())
	if err != nil {
		r.logger.Error("There was a problem while fetching PRs", "error", err)
	}

	err = r.save(db)
	if err != nil {
		r.logger.Error("There was an error while saving results to database", "error", err)
	}
	<-jobs
	wg.Done()
	r.logger.Info("Pull Requests fetched!", "length", len(r.results), "db_time", r.databaseDuration.Seconds(), "process_time", r.executeDuration.Seconds())
}

func (r *RepositoryPRs) save(db *sql.DB) (err error) {
	t := time.Now()
	for _, pr := range r.results {
		_, err := db.Exec("insert or ignore into prs(ID, org, repo, title, state, url, merged_at, created_at, additions, deletions, branch, author, avatar) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			pr.Id, pr.Repository.Owner.Login, pr.Repository.Name, pr.Title,
			pr.State, pr.Url, pr.MergedAt, pr.CreatedAt, pr.Additions,
			pr.Deletions, pr.HeadRefName, pr.Author.Login, pr.Author.AvatarUrl)
		if err != nil {
			return err
		}
		for _, c := range pr.Commits.Nodes {
			_, err := db.Exec(`INSERT OR IGNORE INTO commits(ID, prID, message) VALUES (?, ?, ?)`,
				c.Id, pr.Id, c.Commit.Message)
			if err != nil {
				return err
			}
		}
	}
	r.databaseDuration = time.Since(t)
	return
}

func (r *RepositoryPRs) execute(q *query) (err error) {
	t := time.Now()
	for {
		err := r.client.Query(context.Background(), &q, r.variables)
		if err != nil && r.retries != 0 {
			r.logger.Warn("Retrying fetching pull requests", "retries", r.retries)
			r.retries--
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if err != nil {
			return err
		}
		r.retries = 3 // reset the counter after successful request
		r.results = append(r.results, q.Organization.Repository.PullRequests.Nodes...)
		if r.isAlreadySaved(q.Organization.Repository.PullRequests.Nodes) || !bool(q.Organization.Repository.PullRequests.PageInfo.HasNextPage) {
			break
		}
		r.variables["after"] = githubv4.String(q.Organization.Repository.PullRequests.PageInfo.EndCursor)
	}
	r.executeDuration = time.Since(t)

	return
}

func (r *RepositoryPRs) prepareQuery() *query {
	return &query{}
}

func (r *RepositoryPRs) isAlreadySaved(repos []PullRequest) bool {
	if len(repos) == 0 { // there are some repositories without pull requests
		return true
	}
	lastPR := repos[len(repos)-1]
	DBdate, err := time.Parse(time.RFC3339, "2024-04-01T00:00:00Z")
	if err != nil {
		r.logger.Error("DBdate cannot be parsed", "DBdate", DBdate)
		return false
	}
	PRdate, err := time.Parse(time.RFC3339, string(lastPR.MergedAt))
	if err != nil {
		r.logger.Error("PRdate cannot be parsed", "PRdate", PRdate)
	}

	return PRdate.Before(DBdate)
}

func GetRepositories() ([]Repository, error) {
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

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)
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
