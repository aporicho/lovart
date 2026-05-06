package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
	"github.com/aporicho/lovart/internal/selftest"
	"github.com/aporicho/lovart/internal/signing"
	"github.com/aporicho/lovart/internal/update"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Initialize or repair Lovart runtime files",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := selftest.Run()
			data := setupData(result)
			if !needsRuntimeSync(result) {
				printEnvelope(okLocal(data, true))
				return nil
			}

			syncResult, err := update.SyncAll(context.Background())
			if err != nil {
				data["status"] = selftest.StatusNeedsSetup
				data["ready"] = false
				data["sync_error"] = map[string]any{
					"error": err.Error(),
					"recommended_actions": []string{
						"check network connectivity to www.lovart.ai",
						"rerun `lovart setup`",
						"rerun `lovart update sync --all` for detailed sync diagnostics",
					},
				}
				printEnvelope(okPreflight(data, true))
				return nil
			}

			after := selftest.Run()
			data = setupData(after)
			data["sync"] = syncResult
			printEnvelope(okPreflight(data, true))
			return nil
		},
	}
}

func setupData(result selftest.Result) map[string]any {
	data := map[string]any{
		"status":  result.Status,
		"ready":   result.Status == selftest.StatusReady,
		"version": result.Version,
		"checks":  result.Checks,
	}
	if len(result.RecommendedActions) > 0 {
		data["recommended_actions"] = result.RecommendedActions
	}
	return data
}

func needsRuntimeSync(result selftest.Result) bool {
	return !result.Checks.Signer.OK || !result.Checks.Metadata.OK || !result.Checks.Registry.OK
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
