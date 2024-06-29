package store

import (
	"fmt"

	"github.com/akawula/DoraMatic/github/repositories"
)

type Store interface {
	Close()
	GetRepos(page int, search string) ([]DBRepository, int, error)
	SaveRepos([]repositories.Repository) error
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
