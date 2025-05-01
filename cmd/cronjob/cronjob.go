package main

import (
	"database/sql" // Import standard sql package
	"fmt"
	"log/slog"
	"os"

	"github.com/akawula/DoraMatic/github/client" // Import client package
	"github.com/akawula/DoraMatic/github/organizations"
	"github.com/akawula/DoraMatic/github/pullrequests"
	"github.com/akawula/DoraMatic/github/repositories"
	"github.com/akawula/DoraMatic/slack"
	"github.com/akawula/DoraMatic/store"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // Driver for reading migration files
	_ "github.com/lib/pq"                                // Ensure pq driver is loaded for migrate
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

	// --- Run Database Migrations ---
	l.Info("Running database migrations...")
	dbConnString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_SERVICE_HOST"),
		os.Getenv("POSTGRES_SERVICE_PORT"),
		os.Getenv("POSTGRES_DB"))

	// Need a temporary DB connection for migrate
	tempDb, err := sql.Open("postgres", dbConnString)
	if err != nil {
		l.Error("Failed to open temporary DB connection for migration", "error", err)
		os.Exit(1)
	}
	defer tempDb.Close()

	driver, err := postgres.WithInstance(tempDb, &postgres.Config{})
	if err != nil {
		l.Error("Failed to create postgres migration driver", "error", err)
		os.Exit(1)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations", // Source URL for migration files
		"postgres",          // Database name
		driver)              // Database driver instance
	if err != nil {
		l.Error("Failed to initialize migration instance", "error", err)
		os.Exit(1)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		l.Error("Failed to apply migrations", "error", err)
		os.Exit(1)
	} else if err == migrate.ErrNoChange {
		l.Info("No database migrations to apply.")
	} else {
		l.Info("Database migrations applied successfully.")
	}
	// --- End Migrations ---

	db := store.NewPostgres(l)
	defer db.Close()

	// Create GitHub client
	ghClient := client.Get()

	// Pass ghClient to organizations.GetTeams
	teams, err := organizations.GetTeams(ghClient)
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

	// Pass ghClient to repositories.Get
	repos, err := repositories.Get(ghClient)
	if err != nil {
		l.Error("can't fetch the organizations/repositories from github", "error", err)
		// Exit if repos can't be fetched, as the rest depends on it
		return
	}

	max := len(repos)
	i := 0
	for _, repo := range repos {
		i++
		t := db.GetLastPRDate(string(repo.Owner.Login), string(repo.Name))
		l.Info(fmt.Sprintf("starting fetching pull requests [%d/%d]", i, max), "org", repo.Owner.Login, "repo", repo.Name, "lastPRdate", t)
		// Pass the ghClient to pullrequests.Get
		r, err := pullrequests.Get(ghClient, string(repo.Owner.Login), string(repo.Name), t, l)
		if err != nil {
			slog.Error("there was an error while fetching pull requests", "error", err)
			return
		}

		err = db.SavePullRequest(r)
		if err != nil {
			l.Error("there was a problem while saving prs to db", "error", err)
		}
	}

	// TODO: It's a hacky way to inform security on slack about the last day change, in the future link this dashboard to them
	prs, err := db.FetchSecurityPullRequests()
	if err != nil {
		l.Error("can't fetch the pull requests for security", "error", err)
	}

	slack.SendMessage(prs)
}
