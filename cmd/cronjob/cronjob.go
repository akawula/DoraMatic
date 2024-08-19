package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/akawula/DoraMatic/github/pullrequests"
	"github.com/akawula/DoraMatic/store"
)

func debug() slog.Level {
	level := slog.LevelInfo
	if os.Getenv("DEBUG") == "1" {
		level = slog.LevelDebug
	}
	return level
}

func logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: debug(),
	}))
}

func main() {
	l := logger()
	db := store.NewPostgres(l)
	defer db.Close()

	repos, err := db.GetAllRepos()
	if err != nil {
		l.Error("can't fetch all repositories", "error", err)
		return
	}

	max := len(repos)
	i := 0
	for _, repo := range repos {
		i++
		t := db.GetLastPRDate(repo.Org, repo.Slug)
		l.Info(fmt.Sprintf("Starting fetching pull requests [%d/%d]", i, max), "org", repo.Org, "repo", repo.Slug, "lastPRdate", t)
		r, err := pullrequests.Get(repo.Org, repo.Slug, t, l)
		if err != nil {
			slog.Error("there was an error while fetching pull requests", "error", err)
			return
		}

		err = db.SavePullRequest(r)
		if err != nil {
			l.Error("there was a problem while saving prs to db", "error", err)
		}
	}
}
