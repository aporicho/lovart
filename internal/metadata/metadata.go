// Package metadata owns runtime Lovart metadata cache files.
package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/aporicho/lovart/internal/paths"
)

// Manifest records the runtime generator metadata snapshot.
type Manifest struct {
	Version             int       `json:"version"`
	Source              string    `json:"source"`
	GeneratorListHash   string    `json:"generator_list_hash"`
	GeneratorSchemaHash string    `json:"generator_schema_hash"`
	SignerSHA256        string    `json:"signer_sha256,omitempty"`
	SyncedAt            time.Time `json:"synced_at"`
}

// ReadGeneratorSchema returns the cached OpenAPI schema bytes.
func ReadGeneratorSchema() ([]byte, error) {
	return readRequired(paths.GeneratorSchemaFile, "generator schema")
}

// ReadGeneratorList returns the cached model list bytes.
func ReadGeneratorList() ([]byte, error) {
	return readRequired(paths.GeneratorListFile, "generator list")
}

// ReadManifest reads the cached metadata manifest.
func ReadManifest() (*Manifest, error) {
	data, err := os.ReadFile(paths.MetadataManifestFile)
	if err != nil {
		return nil, fmt.Errorf("metadata: read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("metadata: parse manifest: %w", err)
	}
	return &manifest, nil
}

// WriteManifest writes the metadata manifest atomically.
func WriteManifest(manifest *Manifest) error {
	return WriteJSONAtomic(paths.MetadataManifestFile, manifest, 0644)
}

// WriteJSONAtomic writes JSON via a temporary file then renames it into place.
func WriteJSONAtomic(path string, value any, perm os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("metadata: marshal %s: %w", path, err)
	}
	data = append(data, '\n')
	return WriteFileAtomic(path, data, perm)
}

// WriteFileAtomic writes bytes via a temporary file then renames it into place.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("metadata: create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("metadata: create temp for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("metadata: write temp for %s: %w", path, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("metadata: chmod temp for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("metadata: close temp for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("metadata: replace %s: %w", path, err)
	}
	return nil
}

// StableHash returns a deterministic hash for JSON-like values.
func StableHash(value any) (string, error) {
	normalized := stableValue(value)
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("metadata: marshal stable value: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// HashBytes returns the sha256 hex digest for raw bytes.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readRequired(path, name string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("metadata: %s cache missing at %s; run `lovart update sync --all`: %w", name, path, err)
	}
	return data, nil
}

func stableValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			out[key] = stableValue(v[key])
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, stableValue(item))
		}
		sort.Slice(out, func(i, j int) bool {
			left, _ := json.Marshal(out[i])
			right, _ := json.Marshal(out[j])
			return string(left) < string(right)
		})
		return out
	default:
		return value
	}
}
