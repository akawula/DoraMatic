package store

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/akawula/DoraMatic/github/repositories"
)

type repo struct {
	Slug     string
	Org      string
	Language string
}
type SQLiteDB struct {
	Path   string
	DB     *sql.DB
	Logger *slog.Logger
}

func New(logger *slog.Logger) Store {
	db, err := sql.Open("sqlite3", "./database.db")
	if err != nil {
		logger.Error("There was an error while opening the database", "error", err)
	}

	sqldb := SQLiteDB{Path: "./database", DB: db, Logger: logger}

	return &sqldb
}

func (s *SQLiteDB) Close() {
	s.DB.Close()
}

func (s *SQLiteDB) getTotal(q string) int {
	r := s.DB.QueryRow(q)
	var t int
	r.Scan(&t)

	return t
}

func (s *SQLiteDB) GetRepos(page int, search string) ([]repositories.Repository, int, error) {
	limit := 20 // TODO: someday make it a param from echo, so customer can choose how many rows to show at once
	offset := calculateOffset(page, limit)
	query, queryTotal := getQueryRepos(search)
	lo := fmt.Sprintf(` LIMIT %d OFFSET %d`, limit, offset)
	s.Logger.Debug("Executing SQLite3 query", "query", query+lo)

	total := s.getTotal(queryTotal)

	rows, err := s.DB.Query(query + lo)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	results := []repositories.Repository{}
	for rows.Next() {
		var repo repositories.Repository

		if err := rows.Scan(&repo.Owner.Login, &repo.Name, &repo.PrimaryLanguage.Name); err != nil {
			return nil, 0, err
		}
		results = append(results, repo)
	}

	return results, total, nil
}

func getQueryRepos(search string) (string, string) {
	s := `SELECT * `
	c := `SELECT count(*) `
	q := `FROM repositories ORDER by slug, org`
	if len(search) > 0 {
		q = fmt.Sprintf(`FROM repositories WHERE slug LIKE "%%%s%%" ORDER by slug, org`, search)
	}

	return s + q, c + q
}

func calculateOffset(page, limit int) (offset int) {
	offset = (page - 1) * limit

	if page == 1 {
		offset = 0
	}

	return
}
