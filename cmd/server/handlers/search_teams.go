package handlers

import (
	"encoding/json"
	"log" // Keep only one log import
	"net/http"
	"net/url"

	"github.com/akawula/DoraMatic/store" // Import the store package
	// Remove fuzzysearch import as it's no longer needed
)

// SearchTeamsHandler performs prefix search on team names stored in the database.
// It takes a store.Store as input to interact with the database.
func SearchTeamsHandler(dbStore store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			http.Error(w, "Missing search query parameter 'q'", http.StatusBadRequest)
			return
		}
		// Decode URL-encoded query
		decodedQuery, err := url.QueryUnescape(query)
		if err != nil {
			http.Error(w, "Invalid search query encoding", http.StatusBadRequest)
			return
		}

		log.Printf("Searching team names in DB with prefix: %s", decodedQuery)
		// Fetch team names matching the prefix using the database store
		matches, err := dbStore.SearchDistinctTeamNamesByPrefix(r.Context(), decodedQuery) // Use request context and pass prefix
		if err != nil {
			log.Printf("Error searching team names by prefix: %v", err)
			http.Error(w, "Failed to search team names in database", http.StatusInternalServerError)
			return
		}

		// If no matches found, matches will be an empty slice (or nil, depending on DB driver/sqlc)
		if matches == nil {
			matches = []string{} // Ensure we return an empty JSON array, not null
		}

		log.Printf("Prefix search completed, found %d matches.", len(matches))

		// Return matches as JSON
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(matches); err != nil {
			log.Printf("Error encoding search results: %v", err)
			http.Error(w, "Failed to encode search results", http.StatusInternalServerError)
		}
	}
}
