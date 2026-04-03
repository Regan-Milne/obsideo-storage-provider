// Package store implements plain-filesystem object storage for provider-clean.
//
// Layout:
//
//	{data_dir}/objects/{merkle_hex}         raw file bytes
//	{data_dir}/index/{merkle_hex}.json      {"chunk_size":N,"total_chunks":N,"chunk_hashes":["hex",...]}
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Index holds per-object metadata needed to answer challenge requests.
type Index struct {
	ChunkSize   int      `json:"chunk_size"`
	TotalChunks int      `json:"total_chunks"`
	ChunkHashes []string `json:"chunk_hashes"` // hex-encoded sha256(fmt.Sprintf("%d%x", i, chunk))
}

// Store manages objects on the local filesystem.
type Store struct {
	objDir   string
	indexDir string
}

// New creates a Store rooted at dataDir, creating subdirectories if needed.
func New(dataDir string) (*Store, error) {
	objDir := filepath.Join(dataDir, "objects")
	idxDir := filepath.Join(dataDir, "index")
	for _, d := range []string{objDir, idxDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("create store dir %s: %w", d, err)
		}
	}
	return &Store{objDir: objDir, indexDir: idxDir}, nil
}

// Put stores raw bytes under merkleHex, computing and persisting the chunk index.
// chunkSize determines how bytes are split for Merkle/challenge purposes.
// Writes are atomic (temp file → rename).
func (s *Store) Put(merkleHex string, data []byte, chunkSize int) error {
	if chunkSize <= 0 {
		chunkSize = 10240
	}

	// Build chunk hashes.
	idx := buildIndex(data, chunkSize)

	// Write object atomically.
	if err := atomicWrite(s.objPath(merkleHex), data); err != nil {
		return fmt.Errorf("write object: %w", err)
	}

	// Write index atomically.
	idxBytes, err := json.Marshal(idx)
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	if err := atomicWrite(s.idxPath(merkleHex), idxBytes); err != nil {
		return fmt.Errorf("write index: %w", err)
	}
	return nil
}

// Get returns the raw bytes for merkleHex, or an error if not found.
func (s *Store) Get(merkleHex string) ([]byte, error) {
	data, err := os.ReadFile(s.objPath(merkleHex))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return data, nil
}

// GetIndex returns the stored Index for merkleHex.
func (s *Store) GetIndex(merkleHex string) (*Index, error) {
	data, err := os.ReadFile(s.idxPath(merkleHex))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}
	return &idx, nil
}

// Delete removes the object and its index. Returns nil if the object does not exist.
func (s *Store) Delete(merkleHex string) error {
	_ = os.Remove(s.objPath(merkleHex))
	_ = os.Remove(s.idxPath(merkleHex))
	return nil
}

// List returns all stored merkle root hex strings.
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.objDir)
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}
	roots := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			name := e.Name()
			// Validate: must be a 64-char hex string (32-byte sha256 or sha3-512 prefix)
			if isHexName(name) {
				roots = append(roots, name)
			}
		}
	}
	return roots, nil
}

// ErrNotFound is returned by Get/GetIndex when the object does not exist.
var ErrNotFound = fmt.Errorf("object not found")

// --- helpers ---

func (s *Store) objPath(merkleHex string) string {
	return filepath.Join(s.objDir, merkleHex)
}

func (s *Store) idxPath(merkleHex string) string {
	return filepath.Join(s.indexDir, merkleHex+".json")
}

func buildIndex(data []byte, chunkSize int) Index {
	total := (len(data) + chunkSize - 1) / chunkSize
	if total == 0 {
		total = 1
	}
	hashes := make([]string, total)
	for i := 0; i < total; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[start:end]
		// Platform spec: sha256(fmt.Sprintf("%d%x", index, chunk_bytes))
		h := sha256.Sum256([]byte(fmt.Sprintf("%d%x", i, chunk)))
		hashes[i] = hex.EncodeToString(h[:])
	}
	return Index{
		ChunkSize:   chunkSize,
		TotalChunks: total,
		ChunkHashes: hashes,
	}
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func isHexName(s string) bool {
	if len(s) == 0 {
		return false
	}
	return strings.IndexFunc(s, func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F'))
	}) == -1
}

// StreamTo writes the object for merkleHex to w. Returns ErrNotFound if absent.
func (s *Store) StreamTo(merkleHex string, w io.Writer) error {
	f, err := os.Open(s.objPath(merkleHex))
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
