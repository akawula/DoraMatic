package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/akawula/DoraMatic/github/pullrequests"
	"github.com/akawula/DoraMatic/github/repositories"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type DBRepository struct {
	Org      string
	Slug     string
	Language string
}

type Count struct {
	Total int
}

type Postgres struct {
	db     *sqlx.DB
	Logger *slog.Logger
}

func NewPostgres(logger *slog.Logger) Store {
	connection := fmt.Sprintf("user=%s dbname=%s sslmode=disable password=%s host=%s port=%s", os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_DB"), os.Getenv("POSTGRES_PASSWORD"), os.Getenv("POSTGRES_SERVICE_HOST"), os.Getenv("POSTGRES_SERVICE_PORT"))
	db, err := sqlx.Connect("postgres", connection)
	if err != nil {
		logger.Error("can't connect to postgres", "error", err)
	}

	return &Postgres{db: db, Logger: logger}
}

func (p *Postgres) Close() {
	p.db.Close()
}

func (p *Postgres) getTotal(q string) int {
	t := Count{}
	p.Logger.Debug("Executing total query", "query", q)

	if err := p.db.Get(&t, q); err != nil {
		p.Logger.Error("can't calculate total", "error", err)
		return 0
	}

	return t.Total
}

func (p *Postgres) GetRepos(page int, search string) ([]DBRepository, int, error) {
	repos := []DBRepository{}
	limit := 20 // TODO: someday make it a param from echo, so customer can choose how many rows to show at once
	offset := calculateOffset(page, limit)
	query, queryTotal := getQueryRepos(search)
	lo := fmt.Sprintf(` LIMIT %d OFFSET %d`, limit, offset)
	p.Logger.Debug("Executing PSQL query", "query", query+lo)

	total := p.getTotal(queryTotal)

	if err := p.db.Select(&repos, query+lo); err != nil {
		p.Logger.Error("can't fetch repositories", "error", err)
		return nil, 0, err
	}

	return repos, total, nil
}

func (p *Postgres) SaveRepos(repos []repositories.Repository) error {
	batchUpdate := []map[string]interface{}{}
	for _, repo := range repos {
		batchUpdate = append(batchUpdate, map[string]interface{}{"org": repo.Owner.Login, "slug": string(repo.Name), "language": string(repo.PrimaryLanguage.Name)})
	}

	_, err := p.db.NamedExec(`INSERT INTO repositories (org, slug, language)
    VALUES (:org, :slug, :language) ON CONFLICT (org, slug) DO NOTHING`, batchUpdate)
	if err != nil {
		p.Logger.Error("can't insert new repository", "error", err)
		return err
	}
	return nil
}

func (p *Postgres) GetLastPRDate(org string, repo string) (t time.Time) {
	t = time.Now().AddDate(-2, 0, 0) // -2 years
	w := map[string]interface{}{"org": org, "repo": repo}
	rows, err := p.db.NamedQuery("SELECT merged_at FROM prs WHERE repository_owner = :org AND repository_name = :repo ORDER BY merged_at DESC LIMIT 1", w)
	if err != nil {
		p.Logger.Error("Can't feetch last date of pr", "error", err, "repo", repo, "org", org)
		return
	}

	for rows.Next() {
		err := rows.Scan(&t)
		if err != nil {
			p.Logger.Error("there was an error while scanning rows for last date", "error", err, "org", org, "repo", repo)
			return
		}
	}

	return
}

func (p *Postgres) SavePullRequest(prs []pullrequests.PullRequest) (err error) {
	if len(prs) == 0 {
		p.Logger.Info("Pull Requests slice is empty, going next...")
		return
	}

	batchUpdate := []map[string]interface{}{}
	for _, pr := range prs {
		var review_at sql.NullString
		if len(pr.TimelineItems.Nodes) > 0 {
			review_at = sql.NullString{
				String: string(pr.TimelineItems.Nodes[0].ReviewRequestedEventFragment.CreatedAt),
				Valid:  true,
			}
		}
		batchUpdate = append(batchUpdate, map[string]interface{}{
			"id":                  pr.Id,
			"url":                 pr.Url,
			"title":               pr.Title,
			"state":               pr.State,
			"author":              pr.Author.Login,
			"additions":           pr.Additions,
			"deletions":           pr.Deletions,
			"merged_at":           pr.MergedAt,
			"created_at":          pr.CreatedAt,
			"branch_name":         pr.HeadRefName,
			"repository_name":     pr.Repository.Name,
			"repository_owner":    pr.Repository.Owner.Login,
			"reviews_requested":   pr.TimelineItems.TotalCount,
			"review_requested_at": review_at,
		})

		if len(pr.Commits.Nodes) > 0 {
			err = p.SaveCommits(string(pr.Id), pr.Commits.Nodes)
		}

		if err != nil {
			p.Logger.Error("can't save commits", "pr", pr.Id, "commits", pr.Commits.Nodes)
		}
	}

	_, err = p.db.NamedExec(`INSERT INTO prs (id, title, state, url, merged_at, created_at, additions, deletions, branch_name, author, repository_name, repository_owner, review_requested_at, reviews_requested)
    VALUES (:id, :title, :state, :url, :merged_at, :created_at, :additions, :deletions, :branch_name, :author, :repository_name, :repository_owner, :review_requested_at, :reviews_requested) ON CONFLICT (id) DO NOTHING`, batchUpdate)
	if err != nil {
		p.Logger.Error("can't insert new pull request", "error", err)
		return
	}
	return
}

func (p *Postgres) SaveCommits(pr_id string, commits []pullrequests.Commit) (err error) {
	batchUpdate := []map[string]interface{}{}
	for _, commit := range commits {
		batchUpdate = append(batchUpdate, map[string]interface{}{
			"id":      string(commit.Id),
			"pr_id":   pr_id,
			"message": commit.Commit.Message,
		})
	}

	_, err = p.db.NamedExec(`INSERT INTO commits (id, pr_id, message)
    VALUES (:id, :pr_id, :message) ON CONFLICT (id) DO NOTHING`, batchUpdate)
	if err != nil {
		p.Logger.Error("can't insert new commit", "error", err)
		return
	}
	return
}

func (p *Postgres) GetAllRepos() ([]DBRepository, error) {
	repos := []DBRepository{}
	if err := p.db.Select(&repos, "SELECT * FROM repositories"); err != nil {
		p.Logger.Error("can't fetch repositories", "error", err)
		return nil, err
	}

	return repos, nil
}
