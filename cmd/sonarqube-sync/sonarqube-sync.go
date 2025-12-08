package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/akawula/DoraMatic/sonarqube/client"
	"github.com/akawula/DoraMatic/store"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

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

// --- Application Struct ---

// App holds the application's dependencies.
type App struct {
	log         *slog.Logger
	db          store.Store
	sonarClient *client.SonarQubeClient
	metricKeys  []string
}

// NewApp creates a new App instance with dependencies.
func NewApp(l *slog.Logger, db store.Store, sonarClient *client.SonarQubeClient, metricKeys []string) *App {
	return &App{
		log:         l,
		db:          db,
		sonarClient: sonarClient,
		metricKeys:  metricKeys,
	}
}

// ProgressTracker tracks progress of project processing
type ProgressTracker struct {
	total     int
	processed int
	failed    int
	mu        sync.Mutex
	log       *slog.Logger
	startTime time.Time
}

func NewProgressTracker(total int, log *slog.Logger) *ProgressTracker {
	return &ProgressTracker{
		total:     total,
		processed: 0,
		failed:    0,
		log:       log,
		startTime: time.Now(),
	}
}

func (pt *ProgressTracker) Increment(success bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.processed++
	if !success {
		pt.failed++
	}

	// Log progress every 10% or every 50 projects
	logInterval := pt.total / 10
	if logInterval < 50 {
		logInterval = 50
	}

	if pt.processed%logInterval == 0 || pt.processed == pt.total {
		elapsed := time.Since(pt.startTime)
		percentage := float64(pt.processed) / float64(pt.total) * 100

		pt.log.Info("Progress update",
			"processed", pt.processed,
			"total", pt.total,
			"percentage", fmt.Sprintf("%.1f%%", percentage),
			"failed", pt.failed,
			"elapsed", elapsed.Round(time.Second).String(),
		)
	}
}

func (pt *ProgressTracker) GetStats() (processed, failed, total int, elapsed time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.processed, pt.failed, pt.total, time.Since(pt.startTime)
}

// Run executes the SonarQube metrics collection logic.
func (a *App) Run(ctx context.Context) error {
	a.log.Info("Starting SonarQube metrics collection...")

	// Fetch all projects
	a.log.Info("Fetching all projects from SonarQube...")
	projects, err := a.sonarClient.GetAllProjects(ctx)
	if err != nil {
		a.log.Error("Failed to fetch projects", "error", err)
		return err
	}

	a.log.Info("Projects fetched", "count", len(projects))

	if len(projects) == 0 {
		a.log.Warn("No projects found")
		return nil
	}

	// Initialize progress tracker
	tracker := NewProgressTracker(len(projects), a.log)
	recordedAt := time.Now()

	// Process each project
	for _, project := range projects {
		success := a.processProject(ctx, project, recordedAt)
		tracker.Increment(success)
	}

	// Final statistics
	processed, failed, total, elapsed := tracker.GetStats()
	a.log.Info("SonarQube metrics collection complete",
		"total_projects", total,
		"processed", processed,
		"failed", failed,
		"success_rate", fmt.Sprintf("%.1f%%", float64(processed-failed)/float64(total)*100),
		"total_duration", elapsed.Round(time.Second).String(),
	)

	return nil
}

func (a *App) processProject(ctx context.Context, project client.Project, recordedAt time.Time) bool {
	// Fetch metrics for the project
	metrics, err := a.sonarClient.GetProjectMetrics(ctx, project.Key, a.metricKeys)
	if err != nil {
		a.log.Warn("Failed to fetch metrics for project",
			"project_key", project.Key,
			"project_name", project.Name,
			"error", err,
		)
		return false
	}

	// Save project metadata
	if err := a.db.SaveSonarQubeProject(ctx, project.Key, project.Name); err != nil {
		a.log.Error("Failed to save project metadata",
			"project_key", project.Key,
			"error", err,
		)
		return false
	}

	// Convert metrics to map[string]float64
	metricsMap := make(map[string]float64)
	for key, metric := range metrics.Metrics {
		metricsMap[key] = metric.Value
	}

	// Save metrics
	if err := a.db.SaveSonarQubeMetrics(ctx, project.Key, metricsMap, recordedAt); err != nil {
		a.log.Error("Failed to save metrics",
			"project_key", project.Key,
			"error", err,
		)
		return false
	}

	a.log.Debug("Successfully saved metrics",
		"project_key", project.Key,
		"project_name", project.Name,
		"metric_count", len(metrics.Metrics),
	)

	return true
}

// --- Main Entry Point ---

func main() {
	l := logger()
	ctx := context.Background()

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
		"file://migrations",
		"postgres",
		driver)
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

	// --- Initialize Dependencies ---
	db := store.NewPostgres(ctx, l)
	defer db.Close()

	// Get SonarQube configuration
	sonarURL := os.Getenv("SONAR_URL")
	if sonarURL == "" {
		sonarURL = "https://sonarcloud.io"
	}

	sonarToken := os.Getenv("SONAR_TOKEN")
	if sonarToken == "" {
		l.Error("SONAR_TOKEN environment variable is required")
		os.Exit(1)
	}

	sonarClient := client.NewClient(sonarURL, sonarToken)

	// Default metrics to collect
	metricKeys := []string{
		"bugs",
		"vulnerabilities",
		"code_smells",
		"coverage",
		"duplicated_lines_density",
		"ncloc",
		"sqale_index",
		"sqale_debt_ratio",
		"reliability_rating",
		"security_rating",
		"sqale_rating",
	}

	l.Info("SonarQube configuration",
		"url", sonarURL,
		"metrics", metricKeys,
	)

	// --- Create and Run App ---
	app := NewApp(l, db, sonarClient, metricKeys)

	if err := app.Run(ctx); err != nil {
		l.Error("SonarQube metrics collection failed", "error", err)
		os.Exit(1)
	} else {
		l.Info("SonarQube metrics collection completed successfully.")
	}
}
