package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/akawula/DoraMatic/github/pullrequests"
	"github.com/akawula/DoraMatic/github/repositories"
)

type SecurityPR struct {
	Title           string
	RepositoryName  string `db:"repository_name"`
	RepositoryOwner string `db:"repository_owner"`
	Author          string
	Additions       int
	Deletions       int
	State           string
	CreatedAt       string         `db:"created_at"`
	MergedAt        sql.NullString `db:"merged_at"`
	Url             string
	Id              string
}

type Store interface {
	Close()
	GetRepos(page int, search string) ([]DBRepository, int, error)
	SaveRepos([]repositories.Repository) error
	GetLastPRDate(org string, repo string) time.Time
	SavePullRequest(prs []pullrequests.PullRequest) (err error)
	GetAllRepos() ([]DBRepository, error)
	SaveTeams(teams map[string][]string) error
	FetchSecurityPullRequests() ([]SecurityPR, error)
}

func getQueryRepos(search string) (string, string) {
	s := `SELECT org, slug, language `
	c := `SELECT count(*) as total `
	q := `FROM repositories`
	if len(search) > 0 {
		q = fmt.Sprintf(`FROM repositories WHERE slug LIKE '%%%s%%'`, search)
	}

	return s + q + " ORDER by slug, org", c + q
}

func calculateOffset(page, limit int) (offset int) {
	offset = (page - 1) * limit

	if page == 1 {
		offset = 0
	}

	return
}
