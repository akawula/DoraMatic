package handlers

import (
	"context" // Added for context.Context
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/akawula/DoraMatic/cmd/server/auth"
	"github.com/akawula/DoraMatic/store/sqlc" // Import sqlc package
	"github.com/jackc/pgx/v5"                 // Import pgx for pgx.ErrNoRows
	"golang.org/x/crypto/bcrypt"             // Import bcrypt
)

// DBStore defines the interface required by the LoginHandler for database operations.
// This typically would be implemented by your *store.Postgres or *sqlc.Queries.
type DBStore interface {
	GetUserByUsername(ctx context.Context, username string) (sqlc.User, error)
}

type LoginCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// LoginHandler handles user login requests using database validation.
func LoginHandler(db DBStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creds LoginCredentials
		err := json.NewDecoder(r.Body).Decode(&creds)
		if err != nil {
			writeJSONError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if creds.Username == "" || creds.Password == "" {
			// For missing fields, it's arguably a client error (Bad Request) or still an auth failure.
			// Let's use 401 with the generic message as per "anything else" interpretation for now.
			// A 400 with "Username and password are required" might also be suitable.
			writeJSONError(w, "Wrong login or password", http.StatusUnauthorized)
			return
		}

		user, err := db.GetUserByUsername(r.Context(), creds.Username)
		if err != nil {
			if err == pgx.ErrNoRows { // Check for pgx.ErrNoRows
				writeJSONError(w, "Wrong login or password", http.StatusUnauthorized)
			} else {
				// Log the error for server-side inspection
				fmt.Printf("Error getting user by username: %v\n", err) // Consider using a structured logger
				writeJSONError(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(creds.Password))
		if err != nil {
			// This includes bcrypt.ErrMismatchedHashAndPassword
			writeJSONError(w, "Wrong login or password", http.StatusUnauthorized)
			return
		}

		// Password is correct, generate JWT
		// user.ID is likely int32 from sqlc, convert to string for JWT claim
		userIDStr := fmt.Sprintf("%d", user.ID)
		tokenString, err := auth.GenerateJWT(userIDStr, user.Username)
		if err != nil {
			fmt.Printf("Error generating JWT: %v\n", err) // Consider using a structured logger
			writeJSONError(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Explicitly set OK status
		json.NewEncoder(w).Encode(LoginResponse{Token: tokenString, Username: user.Username})
	}
}
