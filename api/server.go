package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/Regan-Milne/obsideo-storage-provider/store"
	"github.com/Regan-Milne/obsideo-storage-provider/tokens"
)

// Server is the provider HTTP server.
type Server struct {
	store    *store.Store
	verifier *tokens.Verifier
}

// New creates a Server.
func New(st *store.Store, v *tokens.Verifier) *Server {
	return &Server{store: st, verifier: v}
}

// Handler builds and returns the router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	// Authenticated (coordinator-issued JWT required).
	r.Post("/upload/{merkle}", s.handleUpload)
	r.Get("/download/{merkle}", s.handleDownload)

	// Internal — called by coordinator; restrict at firewall in production.
	r.Post("/challenge", s.handleChallenge)
	r.Post("/replicate", s.handleReplicate)
	r.Delete("/objects/{merkle}", s.handleDelete)
	r.Get("/list", s.handleList)

	// Health.
	r.Get("/health", s.handleHealth)

	return r
}

// --- shared helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// bearerToken extracts the token from "Authorization: Bearer <token>".
func bearerToken(r *http.Request) (string, error) {
	hdr := r.Header.Get("Authorization")
	if len(hdr) < 8 || hdr[:7] != "Bearer " {
		return "", fmt.Errorf("missing or malformed Authorization header")
	}
	return hdr[7:], nil
}
