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

func (s *SQLiteDB) GetRepos(page int) ([]repositories.Repository, error) {
	limit := 20 // TODO: someday make it a param from echo, so customer can choose how many rows to show at once
	offset := calculateOffset(page, limit)

	query := fmt.Sprintf(`SELECT * FROM repositories ORDER by slug, org LIMIT %d OFFSET %d`, limit, offset)
	s.Logger.Debug("Executing SQLite3 query", "query", query)

	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []repositories.Repository{}
	for rows.Next() {
		var repo repositories.Repository

		if err := rows.Scan(&repo.Owner.Login, &repo.Name, &repo.PrimaryLanguage.Name); err != nil {
			return nil, err
		}
		results = append(results, repo)
	}

	return results, nil
}

func calculateOffset(page, limit int) (offset int) {
	offset = (page - 1) * limit

	if page == 1 {
		offset = 0
	}

	return
}
