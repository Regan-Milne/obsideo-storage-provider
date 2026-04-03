package api

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	merkle := chi.URLParam(r, "merkle")

	// Verify token.
	tok, err := bearerToken(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	claims, err := s.verifier.Verify(tok)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if claims.Type != "upload" {
		writeError(w, http.StatusForbidden, "expected upload token")
		return
	}
	if claims.MerkleRoot != merkle {
		writeError(w, http.StatusForbidden, "token merkle_root does not match URL")
		return
	}

	// Parse chunk_size query param (optional; defaults to 10240).
	chunkSize := 10240
	if cs := r.URL.Query().Get("chunk_size"); cs != "" {
		if n, err := strconv.Atoi(cs); err == nil && n > 0 {
			chunkSize = n
		}
	}

	// Read body.
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	if err := s.store.Put(merkle, data, chunkSize); err != nil {
		writeError(w, http.StatusInternalServerError, "store: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
