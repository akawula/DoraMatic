package store

import (
	"github.com/akawula/DoraMatic/github/repositories"
)

type Store interface {
	Close()
	GetRepos(page int) ([]repositories.Repository, error)
}
