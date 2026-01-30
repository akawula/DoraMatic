package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	// Original imports
	"github.com/akawula/DoraMatic/github/client"
	"github.com/akawula/DoraMatic/github/codeowners"
	"github.com/akawula/DoraMatic/github/organizations"
	"github.com/akawula/DoraMatic/github/pullrequests"
	"github.com/akawula/DoraMatic/github/repositories"
	"github.com/akawula/DoraMatic/slack"
	"github.com/akawula/DoraMatic/store"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// FileFetcherInterface defines the interface for fetching PR files.
type FileFetcherInterface interface {
	FetchAndClassifyFiles(ctx context.Context, owner, repo string, prNumber int, changedFilesCount int) pullrequests.FileFetchResult
}

// --- Helper Functions ---

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

// --- Function Types for Dependencies ---
// These allow swapping real functions with mocks during testing.

// GetTeamsFunc defines the signature for fetching teams (including member avatars).
// Changed return type value from []string to []organizations.MemberInfo
type GetTeamsFunc func(ghClient client.GitHubV4Client) (map[string][]organizations.MemberInfo, error)

// GetReposFunc defines the signature for fetching repositories.
type GetReposFunc func(ghClient client.GitHubV4Client) ([]repositories.Repository, error)

// GetPullRequestsFunc defines the signature for fetching pull requests.
type GetPullRequestsFunc func(ghClient client.GitHubV4Client, org string, repo string, since time.Time, l *slog.Logger) ([]pullrequests.PullRequest, error)

// GetCodeownersFunc defines the signature for fetching CODEOWNERS for a repository.
type GetCodeownersFunc func(ghClient client.GitHubV4Client, org string, repo string, l *slog.Logger) (*codeowners.RepositoryOwnership, error)

// SendMessageFunc defines the signature for sending slack messages.
type SendMessageFunc func(prs []store.SecurityPR)

// --- Application Struct ---

// App holds the application's dependencies.
type App struct {
	log               *slog.Logger
	db                store.Store
	ghClient          client.GitHubV4Client
	getTeamsFunc      GetTeamsFunc
	getReposFunc      GetReposFunc
	getPullReqsFunc   GetPullRequestsFunc
	getCodeownersFunc GetCodeownersFunc
	sendMessageFunc   SendMessageFunc
	fileFetcher       FileFetcherInterface
}

// NewApp creates a new App instance with dependencies.
func NewApp(l *slog.Logger, db store.Store, ghClient client.GitHubV4Client, getTeams GetTeamsFunc, getRepos GetReposFunc, getPRs GetPullRequestsFunc, getCodeowners GetCodeownersFunc, sendMsg SendMessageFunc, fileFetcher FileFetcherInterface) *App {
	return &App{
		log:               l,
		db:                db,
		ghClient:          ghClient,
		getTeamsFunc:      getTeams,
		getReposFunc:      getRepos,
		getPullReqsFunc:   getPRs,
		getCodeownersFunc: getCodeowners,
		sendMessageFunc:   sendMsg,
		fileFetcher:       fileFetcher,
	}
}

// Run executes the main logic of the cron job.
func (a *App) Run(ctx context.Context) error {
	a.log.Info("Starting cron job logic...")

	// Fetch and save teams
	teamsWithAvatars, err := a.getTeamsFunc(a.ghClient) // Renamed variable for clarity
	if err != nil {
		a.log.Error("Failed to fetch teams", "error", err)
		return fmt.Errorf("fetching teams: %w", err) // Return error to indicate failure
	}
	for name, members := range teamsWithAvatars { // Iterate over new map type
		a.log.Debug("Team found", "name", name, "members", len(members)) // Log remains the same
	}
	// Pass the new map type to SaveTeams (interface/implementation update needed)
	if err = a.db.SaveTeams(ctx, teamsWithAvatars); err != nil {
		a.log.Error("Failed to save teams to DB", "error", err)
		// Continue execution even if saving teams fails? Or return err? Decide based on requirements.
		// return fmt.Errorf("saving teams: %w", err)
	}
	a.log.Info("Teams processed and saved.")

	// Fetch repositories
	repos, err := a.getReposFunc(a.ghClient)
	if err != nil {
		a.log.Error("Failed to fetch repositories from GitHub", "error", err)
		return fmt.Errorf("fetching repositories: %w", err) // Exit if repos can't be fetched
	}
	a.log.Info("Repositories fetched.", "count", len(repos))

	// Save repositories to database
	if err = a.db.SaveRepos(ctx, repos); err != nil {
		a.log.Error("Failed to save repositories to DB", "error", err)
		// Continue even if saving fails, we still need to process PRs
	} else {
		a.log.Info("Repositories saved to database.", "count", len(repos))
	}

	// Fetch and save CODEOWNERS for each repository
	a.log.Info("Fetching CODEOWNERS for repositories...")
	var ownerships []codeowners.RepositoryOwnership
	for i, repo := range repos {
		repoOwner := string(repo.Owner.Login)
		repoName := string(repo.Name)

		a.log.Debug(fmt.Sprintf("Fetching CODEOWNERS [%d/%d]", i+1, len(repos)), "org", repoOwner, "repo", repoName)

		ownership, err := a.getCodeownersFunc(a.ghClient, repoOwner, repoName, a.log)
		if err != nil {
			// Log error but continue to next repo - not all repos may have CODEOWNERS
			a.log.Debug("Failed to fetch CODEOWNERS", "org", repoOwner, "repo", repoName, "error", err)
			continue
		}
		if ownership != nil && len(ownership.Teams) > 0 {
			ownerships = append(ownerships, *ownership)
			a.log.Debug("Found CODEOWNERS teams", "org", repoOwner, "repo", repoName, "teams", len(ownership.Teams))
		}
	}

	// Save repository ownerships to database
	if len(ownerships) > 0 {
		if err = a.db.SaveRepositoryOwners(ctx, ownerships); err != nil {
			a.log.Error("Failed to save repository owners to DB", "error", err)
			// Continue even if saving fails
		} else {
			a.log.Info("Repository owners saved to database.", "repos_with_owners", len(ownerships))
		}
	} else {
		a.log.Info("No repository owners found in CODEOWNERS files.")
	}

	// Fetch and save pull requests for each repository
	max := len(repos)
	for i, repo := range repos {
		repoOwner := string(repo.Owner.Login)
		repoName := string(repo.Name)

		// Get the last processed PR date for this repo
		lastPRDate := a.db.GetLastPRDate(ctx, repoOwner, repoName)
		a.log.Info(fmt.Sprintf("Fetching pull requests [%d/%d]", i+1, max), "org", repoOwner, "repo", repoName, "since", lastPRDate)

		// Fetch new pull requests
		newPRs, err := a.getPullReqsFunc(a.ghClient, repoOwner, repoName, lastPRDate, a.log)
		if err != nil {
			// Log error but continue to the next repo? Or return error?
			a.log.Error("Error fetching pull requests", "org", repoOwner, "repo", repoName, "error", err)
			continue // Continue with the next repository
		}

		if len(newPRs) > 0 {
			a.log.Info("Saving new pull requests", "org", repoOwner, "repo", repoName, "count", len(newPRs))
			// Save fetched pull requests
			err = a.db.SavePullRequest(ctx, newPRs)
			if err != nil {
				a.log.Error("Error saving pull requests to DB", "org", repoOwner, "repo", repoName, "error", err)
				// Continue with the next repository even if saving fails?
			}

			// Fetch and classify files for each PR (for generated code tracking)
			if a.fileFetcher != nil {
				a.fetchAndSavePRFiles(ctx, repoOwner, repoName, newPRs)
			}
		} else {
			a.log.Info("No new pull requests found", "org", repoOwner, "repo", repoName)
		}
	}
	a.log.Info("Pull request processing complete.")

	// Fetch security PRs and notify Slack
	secPRs, err := a.db.FetchSecurityPullRequests()
	if err != nil {
		a.log.Error("Failed to fetch security pull requests for notification", "error", err)
		// Continue without notifying? Or return error?
	} else {
		if len(secPRs) > 0 {
			a.log.Info("Sending security pull request notification.", "count", len(secPRs))
			a.sendMessageFunc(secPRs)
		} else {
			a.log.Info("No security pull requests found for notification.")
		}
	}

	a.log.Info("Cron job logic finished successfully.")
	return nil // Indicate success
}

// fetchAndSavePRFiles fetches file-level data for PRs and saves generated code metrics.
func (a *App) fetchAndSavePRFiles(ctx context.Context, repoOwner, repoName string, prs []pullrequests.PullRequest) {
	for i, pr := range prs {
		// Add a small delay between PRs to avoid hitting rate limits (50ms default)
		if i > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
		prID := string(pr.Id)
		prNumber := int(pr.Number)
		changedFiles := int(pr.ChangedFiles)

		// Skip if PR number is 0 (couldn't extract from URL)
		if prNumber == 0 {
			prNumber = pullrequests.ExtractPRNumber(string(pr.Url))
			if prNumber == 0 {
				a.log.Warn("Could not determine PR number, skipping file fetch",
					"pr_id", prID, "url", pr.Url)
				continue
			}
		}

		// Fetch and classify files
		result := a.fileFetcher.FetchAndClassifyFiles(ctx, repoOwner, repoName, prNumber, changedFiles)

		if result.Error != nil {
			a.log.Error("Error fetching PR files",
				"pr_id", prID,
				"org", repoOwner,
				"repo", repoName,
				"pr_number", prNumber,
				"error", result.Error)
			// Mark as incomplete but don't fail the whole job
			if err := a.db.UpdatePRFilesIncomplete(ctx, prID, changedFiles); err != nil {
				a.log.Error("Failed to mark PR files as incomplete", "pr_id", prID, "error", err)
			}
			continue
		}

		// If files weren't fetched due to threshold, mark as incomplete
		if !result.FilesComplete && len(result.Files) == 0 {
			if err := a.db.UpdatePRFilesIncomplete(ctx, prID, changedFiles); err != nil {
				a.log.Error("Failed to mark PR files as incomplete", "pr_id", prID, "error", err)
			}
			continue
		}

		// Convert to store format and save
		storeFiles := make([]store.PRFileChange, len(result.Files))
		for i, f := range result.Files {
			storeFiles[i] = store.PRFileChange{
				Path:        f.Path,
				Additions:   f.Additions,
				Deletions:   f.Deletions,
				Status:      f.Status,
				IsGenerated: f.IsGenerated,
			}
		}

		err := a.db.SavePullRequestFiles(ctx, prID, storeFiles,
			result.GeneratedAdditions, result.GeneratedDeletions, result.FilesComplete)
		if err != nil {
			a.log.Error("Failed to save PR files",
				"pr_id", prID,
				"org", repoOwner,
				"repo", repoName,
				"error", err)
		} else {
			a.log.Debug("Saved PR files",
				"pr_id", prID,
				"total_files", len(result.Files),
				"generated_additions", result.GeneratedAdditions,
				"human_additions", result.HumanAdditions)
		}
	}
}

// --- Main Entry Point ---

func main() {
	l := logger()
	ctx := context.Background() // Main context

	// --- Run Database Migrations ---
	l.Info("Running database migrations...")
	dbConnString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_SERVICE_HOST"),
		os.Getenv("POSTGRES_SERVICE_PORT"),
		os.Getenv("POSTGRES_DB"))

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

	// --- Initialize Real Dependencies ---
	db := store.NewPostgres(ctx, l) // Uses the real Postgres store
	defer db.Close()

	ghClient := client.Get() // Gets the real GitHub client

	// Initialize file fetcher for generated code tracking
	fileFetcher := pullrequests.NewFileFetcher(l)

	// --- Create and Run App ---
	app := NewApp(
		l,
		db,
		ghClient,
		organizations.GetTeams,     // Real function
		repositories.Get,           // Real function
		pullrequests.Get,           // Real function
		codeowners.FetchCodeowners, // Real function for CODEOWNERS
		slack.SendMessage,          // Real function
		fileFetcher,                // File fetcher for generated code tracking
	)

	if err := app.Run(ctx); err != nil {
		l.Error("Cron job failed", "error", err)
		// Optionally, exit with a non-zero code to indicate failure in orchestrators
		// os.Exit(1)
	} else {
		l.Info("Cron job completed successfully.")
	}
}
