package handlers

import (
	"fmt"
	"net/http"
)

// LivezHandler handles the /livez endpoint.
func LivezHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "ok")
}
