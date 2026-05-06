package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
	"github.com/aporicho/lovart/internal/signing"
	"github.com/aporicho/lovart/internal/update"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Check Lovart CLI readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			authStatus := auth.GetStatus()
			updateStatus, updateErr := update.Check(context.Background())
			data := map[string]any{
				"version": "2.0.0-dev",
				"auth": map[string]any{
					"available": authStatus.Available,
					"source":    authStatus.Source,
					"fields":    authStatus.Fields,
				},
				"signer":   checkSigner(),
				"metadata": checkMetadata(),
			}
			if updateErr != nil {
				data["status"] = "network_unavailable"
				data["ready"] = false
				data["update_error"] = map[string]any{
					"error":               updateErr.Error(),
					"recommended_actions": []string{"check network connectivity to www.lovart.ai", "rerun `lovart setup`"},
				}
			} else {
				data["status"] = "ok"
				data["ready"] = authStatus.Available
				data["update"] = updateStatus
			}
			printEnvelope(okPreflight(data, true))
			return nil
		},
	}
}

func checkSigner() map[string]any {
	s, err := signing.NewSigner()
	if err != nil {
		return map[string]any{
			"available":           false,
			"path":                paths.SignerWASMFile,
			"error":               err.Error(),
			"recommended_actions": []string{"run `lovart update sync --all`"},
		}
	}
	if err := s.Health(); err != nil {
		return map[string]any{
			"available":           false,
			"path":                paths.SignerWASMFile,
			"error":               err.Error(),
			"recommended_actions": []string{"run `lovart update sync --all`"},
		}
	}
	if closer, ok := s.(interface{ Close(context.Context) error }); ok {
		_ = closer.Close(context.Background())
	}
	return map[string]any{"available": true, "path": paths.SignerWASMFile}
}

func checkMetadata() map[string]any {
	manifest, err := metadata.ReadManifest()
	if err != nil {
		return map[string]any{
			"available":           false,
			"path":                paths.MetadataManifestFile,
			"error":               err.Error(),
			"recommended_actions": []string{"run `lovart update sync --all`"},
		}
	}
	return map[string]any{
		"available":             true,
		"path":                  paths.MetadataManifestFile,
		"generator_list_hash":   manifest.GeneratorListHash,
		"generator_schema_hash": manifest.GeneratorSchemaHash,
		"synced_at":             manifest.SyncedAt,
	}
}
