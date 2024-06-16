package store

import (
	"github.com/akawula/DoraMatic/github/repositories"
)

type Store interface {
	Close()
	GetRepos(page int, search string) ([]repositories.Repository, int, error)
	SaveRepos([]repositories.Repository) error
}
