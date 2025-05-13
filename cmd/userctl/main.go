package main

import (
	"context"
	"errors" // For errors.As
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jackc/pgx/v5/pgconn" // For pgx error checking
	"github.com/jackc/pgx/v5/pgxpool" // For pgx connection pool

	// Assuming your module path is github.com/akawula/DoraMatic
	"github.com/akawula/DoraMatic/store/sqlc" // SQLC generated code
)

func main() {
	logger := log.New(os.Stdout, "userctl: ", log.LstdFlags)

	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")

	if username == "" || password == "" {
		logger.Fatal("USERNAME and PASSWORD environment variables must be set.")
	}

	// --- Database Connection Initialization (similar to server.go) ---
	dbUser := os.Getenv("POSTGRES_USER")
	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	dbHost := os.Getenv("POSTGRES_SERVICE_HOST")
	dbPort := os.Getenv("POSTGRES_SERVICE_PORT")
	dbName := os.Getenv("POSTGRES_DB")

	if dbUser == "" || dbPassword == "" || dbHost == "" || dbPort == "" || dbName == "" {
		logger.Fatal("Database environment variables (POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_SERVICE_HOST, POSTGRES_SERVICE_PORT, POSTGRES_DB) must be set")
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	var dbpool *pgxpool.Pool
	var err error
	retries := 5
	for i := 0; i < retries; i++ {
		dbpool, err = pgxpool.New(context.Background(), connStr)
		if err == nil {
			err = dbpool.Ping(context.Background())
			if err == nil {
				logger.Println("Successfully connected to the database using pgxpool.")
				break
			}
		}
		logger.Printf("Failed to connect to database with pgxpool (attempt %d/%d): %v. Retrying in 5 seconds...", i+1, retries, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		logger.Fatalf("Failed to connect to database with pgxpool after retries: %v", err)
	}
	defer dbpool.Close()

	queries := sqlc.New(dbpool)
	// --- End Database Connection Initialization ---

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Fatalf("Failed to hash password: %v", err)
	}

	params := sqlc.CreateUserParams{
		Username:       username,
		HashedPassword: string(hashedPassword),
	}

	newUser, err := queries.CreateUser(context.Background(), params)
	if err != nil {
		// Check for unique constraint violation (common error)
		var pgErr *pgconn.PgError
		if ok := errors.As(err, &pgErr); ok && pgErr.Code == "23505" { // 23505 is unique_violation
			logger.Fatalf("Failed to create user: username '%s' already exists. Error: %v", username, err)
		}
		logger.Fatalf("Failed to create user: %v", err)
	}

	logger.Printf("Successfully created user: ID=%d, Username=%s\n", newUser.ID, newUser.Username)
}
