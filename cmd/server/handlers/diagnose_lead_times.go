package handlers

import (
	"encoding/json"
	"net/http"

	"log/slog"

	"github.com/akawula/DoraMatic/store"
)

// DiagnoseLeadTimesHandler handles requests for diagnosing lead time data.
func DiagnoseLeadTimesHandler(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		results, err := s.DiagnoseLeadTimes(ctx)
		if err != nil {
			slog.Error("Failed to execute DiagnoseLeadTimes query", "error", err)
			http.Error(w, "Failed to retrieve diagnostic data", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(results); err != nil {
			slog.Error("Failed to encode diagnostic data response", "error", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}
