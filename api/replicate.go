package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type replicateRequest struct {
	Merkle         string `json:"merkle"`
	SourceProvider string `json:"source_provider"`
	TargetProvider string `json:"target_provider"`
	UploadToken    string `json:"upload_token"`
}

func (s *Server) handleReplicate(w http.ResponseWriter, r *http.Request) {
	var req replicateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode body: "+err.Error())
		return
	}

	// Fetch raw bytes from source.
	data, err := fetchFromSource(req.SourceProvider, req.Merkle, req.UploadToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch source: "+err.Error())
		return
	}

	// Push to target.
	if err := pushToTarget(req.TargetProvider, req.Merkle, req.UploadToken, data); err != nil {
		writeError(w, http.StatusBadGateway, "push target: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func fetchFromSource(sourceAddr, merkle, token string) ([]byte, error) {
	url := fmt.Sprintf("%s/download/%s", sourceAddr, merkle)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("source returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func pushToTarget(targetAddr, merkle, token string, data []byte) error {
	url := fmt.Sprintf("%s/upload/%s?owner=replicated&start=0&chunk_size=10240&proof_type=0", targetAddr, merkle)
	req, err := http.NewRequest(http.MethodPost, url, newBytesReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(len(data))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("target returned %d", resp.StatusCode)
	}
	return nil
}
