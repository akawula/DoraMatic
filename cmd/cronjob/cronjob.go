package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/akawula/DoraMatic/github/organizations"
	"github.com/akawula/DoraMatic/github/pullrequests"
	"github.com/akawula/DoraMatic/github/repositories"

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

	teams, err := organizations.GetTeams()
	if err != nil {
		l.Error("can't fetched teams!", "error", err)
		return
	}

	for name, members := range teams {
		l.Debug("team", "name", name, "members", len(members))
	}

	if err = db.SaveTeams(teams); err != nil {
		l.Error("can't save the teams into DB", "error", err)
	}

	repos, err := repositories.Get()
	if err != nil {
		l.Error("can't fetch the organizations/repositories from github", "error", err)
	}

	max := len(repos)
	i := 0
	for _, repo := range repos {
		i++
		t := db.GetLastPRDate(string(repo.Owner.Login), string(repo.Name))
		l.Info(fmt.Sprintf("starting fetching pull requests [%d/%d]", i, max), "org", repo.Owner.Login, "repo", repo.Name, "lastPRdate", t)
		r, err := pullrequests.Get(string(repo.Owner.Login), string(repo.Name), t, l)
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
