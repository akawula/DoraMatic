package main

import (
	"context"
	"database/sql" // Re-add this import for the connection retry loop
	"fmt"
	"log"      // Keep standard log for initial messages if needed
	"log/slog" // Import slog
	"net/http"
	"os"
	"time"

	// Assuming 'github.com/akawula/DoraMatic' is your module path in go.mod
	"github.com/akawula/DoraMatic/cmd/server/auth"     // Import the auth package
	"github.com/akawula/DoraMatic/cmd/server/handlers" // Import the handlers package
	"github.com/akawula/DoraMatic/store"               // Import the store package
	_ "github.com/lib/pq"                              // PostgreSQL driver
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
	// --- Logger Initialization ---
	logger := logger()
	// --- End Logger Initialization ---

	// --- Database Connection Initialization ---
	dbUser := os.Getenv("POSTGRES_USER")
	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	dbHost := os.Getenv("POSTGRES_SERVICE_HOST")
	dbPort := os.Getenv("POSTGRES_SERVICE_PORT")
	dbName := os.Getenv("POSTGRES_DB")

	if dbUser == "" || dbPassword == "" || dbHost == "" || dbPort == "" || dbName == "" {
		log.Fatal("Database environment variables (POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_SERVICE_HOST, POSTGRES_SERVICE_PORT, POSTGRES_DB) must be set")
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	var db *sql.DB
	var err error
	retries := 5
	for i := 0; i < retries; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.PingContext(context.Background())
			if err == nil {
				log.Println("Successfully connected to the database.")
				break // Connection successful
			}
		}
		log.Printf("Failed to connect to database (attempt %d/%d): %v. Retrying in 5 seconds...", i+1, retries, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		// Use logger for fatal errors
		logger.Error("Failed to connect to database after retries", "attempts", retries, "error", err)
		os.Exit(1) // Exit if DB connection fails
	}
	// defer db.Close() // pgxpool handles closing connections

	// Use NewPostgres constructor with context and logger
	dbStore := store.NewPostgres(context.Background(), logger)
	// --- End Database Connection Initialization ---

	// Use handlers from the handlers package
	http.HandleFunc("GET /livez", handlers.LivezHandler) // Pass the function directly
	http.HandleFunc("GET /api/search/teams", handlers.SearchTeamsHandler(dbStore)) // Added /api prefix
	// Register the team stats handler
	http.HandleFunc("GET /api/teams/{teamName}/stats", handlers.GetTeamStatsHandler(dbStore)) // Added /api prefix

	// --- Auth Routes ---
	http.HandleFunc("POST /api/auth/login", handlers.LoginHandler(dbStore))

	// --- Protected Routes ---
	// Register the new pull requests list handler, now protected by JWT middleware
	http.Handle("GET /api/prs", auth.JWTMiddleware(http.HandlerFunc(handlers.GetPullRequests(logger, dbStore))))
	// Register the new team members handler
	http.HandleFunc("GET /api/teams/{teamName}/members", handlers.GetTeamMembersHandler(dbStore)) // Added /api prefix
	// Register the new diagnostic handler
	http.HandleFunc("GET /diagnose/leadtimes", handlers.DiagnoseLeadTimesHandler(dbStore)) // This might need /api/ if called from frontend and protected
	// Register the new JIRA references handler
	http.HandleFunc("GET /prs/jira", handlers.GetPullRequestsJiraReferences(logger, dbStore)) // This might need /api/ if called from frontend and protected

	port := "8080"                               // Changed default port to 8080
	logger.Info("Server starting", "port", port) // Use structured logging

	// Configure the server with timeouts
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      nil, // Use http.DefaultServeMux
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Fix assignment: use '=' instead of ':=' as err is already declared
	err = server.ListenAndServe()
	if err != nil {
		logger.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
