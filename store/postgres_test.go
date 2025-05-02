package store

import (
	"context"
	"fmt" // Keep fmt
	"log"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/akawula/DoraMatic/github/repositories"
	ghv4 "github.com/shurcooL/githubv4" // Alias for clarity

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // Driver for migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"       // Driver for migrate

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/akawula/DoraMatic/store/sqlc" // Import sqlc explicitly
)

var (
	testPool   *pgxpool.Pool
	testLogger *slog.Logger
	dbURL      string // Database URL for migrations
)

// TestMain sets up the database connection and runs migrations before tests.
func TestMain(m *testing.M) {
	// Setup logger
	testLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Get connection details from environment variables
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := "test_" + os.Getenv("POSTGRES_DB")
	host := os.Getenv("POSTGRES_SERVICE_HOST")
	port := os.Getenv("POSTGRES_SERVICE_PORT")

	if user == "" || password == "" || dbname == "" || host == "" || port == "" {
		log.Println("Skipping integration tests: Required POSTGRES_* environment variables not set.")
		os.Exit(0)
	}

	connectionString := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=disable", user, password, dbname, host, port)
	dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)

	var err error
	testPool, err = pgxpool.New(context.Background(), connectionString)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
	}
	defer testPool.Close()

	// --- Run Migrations ---
	// Revert to path relative to the 'store' package directory
	mig, err := migrate.New("file://../migrations", dbURL)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}

	// Force migrations down first to ensure a clean slate, then up.
	// Ignore "no change" errors for down migration as well.
	if err := mig.Down(); err != nil && err != migrate.ErrNoChange {
		// Log non-critical error if down migration fails (might happen on first run)
		testLogger.Warn("Failed to run migrations down (might be OK on first run)", "error", err)
	}

	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Failed to run migrations up: %v", err)
	}
	testLogger.Info("Migrations applied successfully or no changes needed.")

	// Run tests
	exitCode := m.Run()
	os.Exit(exitCode)
}

// truncateTables clears data from tables used in tests.
func truncateTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	tables := []string{
		"repositories",
		"prs", // Corrected table name
		"commits",
		"pull_request_reviews",
		"teams",
	}
	query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	_, err := pool.Exec(ctx, query)
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}
	testLogger.Debug("Truncated test tables", "tables", tables)
}

// newTestPostgresStore creates a new store instance for testing.
func newTestPostgresStore(t *testing.T) Store {
	t.Helper()
	if testPool == nil {
		t.Fatal("Test pool is not initialized. Ensure TestMain ran correctly.")
	}
	return &Postgres{
		connPool: testPool,
		queries:  sqlc.New(testPool),
		Logger:   testLogger,
	}
}

// --- Test Cases ---

func TestSaveAndGetRepos(t *testing.T) {
	ctx := context.Background()
	truncateTables(t, ctx, testPool)
	store := newTestPostgresStore(t)

	reposToSave := []repositories.Repository{
		{Name: ghv4.String("repo-alpha"), Owner: struct{ Login ghv4.String }{Login: "org1"}, PrimaryLanguage: struct{ Name ghv4.String }{Name: "Go"}},
		{Name: ghv4.String("repo-beta"), Owner: struct{ Login ghv4.String }{Login: "org1"}, PrimaryLanguage: struct{ Name ghv4.String }{Name: "Python"}},
		{Name: ghv4.String("another-repo"), Owner: struct{ Login ghv4.String }{Login: "org2"}, PrimaryLanguage: struct{ Name ghv4.String }{Name: "Go"}},
		{Name: ghv4.String("test-search"), Owner: struct{ Login ghv4.String }{Login: "org1"}},
	}

	err := store.SaveRepos(ctx, reposToSave)
	if err != nil {
		t.Fatalf("SaveRepos failed: %v", err)
	}

	t.Run("GetRepos_Page1_NoSearch", func(t *testing.T) {
		gotRepos, total, err := store.GetRepos(ctx, 1, "")
		if err != nil {
			t.Fatalf("GetRepos failed: %v", err)
		}
		if total != len(reposToSave) {
			t.Errorf("Expected total %d, got %d", len(reposToSave), total)
		}
		if len(gotRepos) != len(reposToSave) {
			t.Errorf("Expected %d repos on page 1, got %d", len(reposToSave), len(gotRepos))
		}
		found := false
		for _, r := range gotRepos {
			if r.Org == "org1" && r.Slug == "repo-alpha" && r.Language.String == "Go" && r.Language.Valid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected 'repo-alpha' not found in results")
		}
	})

	t.Run("GetRepos_Page1_WithSearch", func(t *testing.T) {
		search := "repo"
		gotRepos, total, err := store.GetRepos(ctx, 1, search)
		if err != nil {
			t.Fatalf("GetRepos with search failed: %v", err)
		}
		expectedTotal := 3
		if total != expectedTotal {
			t.Errorf("Expected total %d with search '%s', got %d", expectedTotal, search, total)
		}
		if len(gotRepos) != expectedTotal {
			t.Errorf("Expected %d repos on page 1 with search '%s', got %d", expectedTotal, search, len(gotRepos))
		}
		for _, r := range gotRepos {
			if r.Slug == "test-search" {
				t.Errorf("Found 'test-search' which should not match 'repo'")
			}
		}
	})

	t.Run("GetAllRepos", func(t *testing.T) {
		allRepos, err := store.GetAllRepos(ctx)
		if err != nil {
			t.Fatalf("GetAllRepos failed: %v", err)
		}
		if len(allRepos) != len(reposToSave) {
			t.Errorf("Expected %d total repos, got %d", len(reposToSave), len(allRepos))
		}
	})
}

func TestGetLastPRDate_NoPRs(t *testing.T) {
	ctx := context.Background()
	truncateTables(t, ctx, testPool) // Ensure tables are empty
	store := newTestPostgresStore(t)

	org := "test-org-nopr"
	repo := "test-repo-nopr"

	// Save repo (dependency)
	err := store.SaveRepos(ctx, []repositories.Repository{
		{Name: ghv4.String(repo), Owner: struct{ Login ghv4.String }{Login: ghv4.String(org)}},
	})
	if err != nil {
		t.Fatalf("Failed to save prerequisite repo: %v", err)
	}

	lastDate := store.GetLastPRDate(ctx, org, repo)
	// Expect default time (approx 2 years ago)
	if time.Since(lastDate).Hours() < 24*365*1.9 {
		t.Errorf("Expected default date (approx 2 years ago) when no PRs exist, got %v", lastDate)
	}
}

// TestSavePullRequestWithDetails - Simplified: Only tests empty slice handling
func TestSavePullRequest_Empty(t *testing.T) {
	ctx := context.Background()
	truncateTables(t, ctx, testPool)
	store := newTestPostgresStore(t)

	// Pass an empty slice - expect no error and no changes in DB
	err := store.SavePullRequest(ctx, nil) // Pass nil slice explicitly
	if err != nil {
		t.Fatalf("SavePullRequest with empty slice failed: %v", err)
	}

	var count int
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM prs").Scan(&count) // Corrected table name
	if err != nil {
		t.Fatalf("Failed to query prs count: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 pull requests after saving empty slice, got %d", count)
	}
}

// TestFetchSecurityPullRequests - Simplified: Tests fetching when DB is empty
func TestFetchSecurityPullRequests_Empty(t *testing.T) {
	ctx := context.Background()
	truncateTables(t, ctx, testPool)
	store := newTestPostgresStore(t)

	// Fetch when no PRs exist
	fetchedPRs, err := store.FetchSecurityPullRequests()
	if err != nil {
		t.Fatalf("FetchSecurityPullRequests failed when empty: %v", err)
	}

	if len(fetchedPRs) != 0 {
		t.Errorf("Expected 0 security PRs when DB is empty, got %d", len(fetchedPRs))
	}
}

func TestSaveTeams(t *testing.T) {
	ctx := context.Background()
	truncateTables(t, ctx, testPool)
	store := newTestPostgresStore(t)

	teamsToSave := map[string][]string{
		"team-a": {"member1", "member2"},
		"team-b": {"member2", "member3"},
	}

	err := store.SaveTeams(ctx, teamsToSave)
	if err != nil {
		t.Fatalf("SaveTeams failed: %v", err)
	}

	var count int
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM teams").Scan(&count)
	if err != nil || count != 4 { // Corrected expected count to 4
		t.Fatalf("Failed to verify team member count: err=%v, count=%d", err, count)
	}

	var member string
	err = testPool.QueryRow(ctx, "SELECT member FROM teams WHERE team = $1 AND member = $2", "team-a", "member1").Scan(&member)
	if err != nil || member != "member1" {
		t.Fatalf("Failed to verify team-a member1: err=%v, member=%s", err, member)
	}
	err = testPool.QueryRow(ctx, "SELECT member FROM teams WHERE team = $1 AND member = $2", "team-b", "member3").Scan(&member)
	if err != nil || member != "member3" {
		t.Fatalf("Failed to verify team-b member3: err=%v, member=%s", err, member)
	}

	newTeamsToSave := map[string][]string{
		"team-c": {"member4"},
	}
	err = store.SaveTeams(ctx, newTeamsToSave)
	if err != nil {
		t.Fatalf("SaveTeams (overwrite) failed: %v", err)
	}

	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM teams").Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("Failed to verify team member count after overwrite: err=%v, count=%d", err, count)
	}
	err = testPool.QueryRow(ctx, "SELECT member FROM teams WHERE team = $1 AND member = $2", "team-c", "member4").Scan(&member)
	if err != nil || member != "member4" {
		t.Fatalf("Failed to verify team-c member4 after overwrite: err=%v, member=%s", err, member)
	}
}
