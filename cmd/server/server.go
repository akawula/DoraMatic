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
	"github.com/akawula/DoraMatic/cmd/server/handlers"
	"github.com/akawula/DoraMatic/store" // Import the store package
	_ "github.com/lib/pq"                // PostgreSQL driver needed by pgx? Check pgx docs. Keep for now.
)

func main() {
	// --- Logger Initialization ---
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
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
	http.HandleFunc("/livez", handlers.LivezHandler)
	// Pass the database store to the handler factory
	http.HandleFunc("/search/teams", handlers.SearchTeamsHandler(dbStore))

	port := "10000" // Default port, can be overridden by env var later if needed
	logger.Info("Server starting", "port", port) // Use structured logging

	// Fix assignment: use '=' instead of ':=' as err is already declared
	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		logger.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
