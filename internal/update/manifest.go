package update

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
)

// ReadSignerManifest reads the local signer manifest.
func ReadSignerManifest() (*SignerManifest, error) {
	data, err := os.ReadFile(paths.SignerManifestFile)
	if err != nil {
		return nil, fmt.Errorf("update: read signer manifest: %w", err)
	}
	var manifest SignerManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("update: parse signer manifest: %w", err)
	}
	return &manifest, nil
}

// WriteSignerManifest writes the signer manifest atomically.
func WriteSignerManifest(manifest *SignerManifest) error {
	return metadata.WriteJSONAtomic(paths.SignerManifestFile, manifest, 0644)
}
