package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"github.com/akawula/DoraMatic/store"      // Import the store package
	"github.com/akawula/DoraMatic/store/sqlc" // Import the sqlc package
)

// GetTeamMembersHandler fetches members for a specific team.
// It takes a store.Store as input to interact with the database.
func GetTeamMembersHandler(dbStore store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Use standard library's PathValue (requires Go 1.22+)
		teamName := r.PathValue("teamName")
		if teamName == "" {
			// This check might be redundant if the router guarantees the param exists
			// but good for robustness.
			http.Error(w, "Missing team name in path", http.StatusBadRequest)
			return
		}

		// Decode URL-encoded team name if necessary (though less common for path params)
		decodedTeamName, err := url.PathUnescape(teamName)
		if err != nil {
			log.Printf("Error decoding team name '%s': %v", teamName, err)
			http.Error(w, "Invalid team name encoding", http.StatusBadRequest)
			return
		}

		log.Printf("Fetching members for team: %s", decodedTeamName)
		members, err := dbStore.GetTeamMembers(r.Context(), decodedTeamName)
		if err != nil {
			log.Printf("Error fetching team members for '%s': %v", decodedTeamName, err)
			// Consider returning 404 if team not found, but 500 is safer for general DB errors
			http.Error(w, "Failed to fetch team members from database", http.StatusInternalServerError)
			return
		}

		// Ensure we return an empty JSON array [] instead of null if no members found
		if members == nil {
			members = []sqlc.GetTeamMembersRow{} // Use the type from the sqlc package
		}

		log.Printf("Found %d members for team %s.", len(members), decodedTeamName)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(members); err != nil {
			log.Printf("Error encoding team members result: %v", err)
			http.Error(w, "Failed to encode team members result", http.StatusInternalServerError)
		}
	}
}
