package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
	"github.com/aporicho/lovart/internal/signing"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	var offline bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Check Lovart CLI readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			authStatus := auth.GetStatus()
			printEnvelope(envelope.OK(map[string]any{
				"status":  "ok",
				"version": "2.0.0-dev",
				"auth": map[string]any{
					"available": authStatus.Available,
					"source":    authStatus.Source,
					"fields":    authStatus.Fields,
				},
				"signer":   checkSigner(),
				"metadata": checkMetadata(),
				"offline":  offline,
			}))
			return nil
		},
	}

	cmd.Flags().BoolVar(&offline, "offline", false, "skip live checks")
	return cmd
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
