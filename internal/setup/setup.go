// Package setup reports Lovart CLI readiness.
package setup

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
	"github.com/aporicho/lovart/internal/signing"
)

// Status reports auth, refs, signer, and runtime readiness.
type Status struct {
	Ready    bool           `json:"ready"`
	Auth     map[string]any `json:"auth"`
	Signer   map[string]any `json:"signer"`
	Refs     map[string]any `json:"refs"`
	Warnings []string       `json:"warnings,omitempty"`
}

// Readiness checks local components needed before online generation.
func Readiness() *Status {
	authStatus := auth.GetStatus()
	status := &Status{
		Auth: map[string]any{
			"available": authStatus.Available,
			"source":    authStatus.Source,
			"fields":    authStatus.Fields,
		},
		Signer: signerStatus(),
		Refs:   metadataStatus(),
	}
	status.Ready = authStatus.Available && boolField(status.Signer, "available") && boolField(status.Refs, "available")
	if !status.Ready {
		status.Warnings = append(status.Warnings, "run `lovart update sync --all` after credentials are available")
	}
	return status
}

func signerStatus() map[string]any {
	signer, err := signing.NewSigner()
	if err != nil {
		return map[string]any{"available": false, "path": paths.SignerWASMFile, "error": err.Error()}
	}
	if closer, ok := signer.(interface{ Close(context.Context) error }); ok {
		defer closer.Close(context.Background())
	}
	if err := signer.Health(); err != nil {
		return map[string]any{"available": false, "path": paths.SignerWASMFile, "error": err.Error()}
	}
	return map[string]any{"available": true, "path": paths.SignerWASMFile}
}

func metadataStatus() map[string]any {
	manifest, err := metadata.ReadManifest()
	if err != nil {
		return map[string]any{"available": false, "path": paths.MetadataManifestFile, "error": err.Error()}
	}
	return map[string]any{
		"available":             true,
		"path":                  paths.MetadataManifestFile,
		"generator_list_hash":   manifest.GeneratorListHash,
		"generator_schema_hash": manifest.GeneratorSchemaHash,
	}
}

func boolField(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}
