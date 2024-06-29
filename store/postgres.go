package store

import (
	"fmt"
	"log/slog"
	"os"

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
    VALUES (:org, :slug, :language)`, batchUpdate)
	if err != nil {
		p.Logger.Error("can't insert new repository", "error", err)
		return err
	}
	return nil
}
