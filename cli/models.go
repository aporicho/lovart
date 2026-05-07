package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/discovery"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/registry"
	"github.com/aporicho/lovart/internal/signing"
	"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
	var refresh bool

	cmd := &cobra.Command{
		Use:   "models",
		Short: "List known Lovart generator models",
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if !refresh {
				reg, err := registry.Load()
				if err != nil {
					printEnvelope(envelope.Err(errors.CodeMetadataStale, "failed to load model registry", map[string]any{
						"error":               err.Error(),
						"recommended_actions": []string{"run `lovart update sync --all`"},
					}))
					return nil
				}
				models := summarizeRegistryModels(reg.Models())
				printEnvelope(okLocal(map[string]any{
					"models": models,
					"count":  len(models),
					"source": "registry",
				}, true))
				return nil
			}

			// 1. Load credentials.
			creds, err := auth.Load()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeAuthMissing, "no credentials found", map[string]any{
					"error": err.Error(),
					"recommended_actions": []string{
						"run `lovart auth login`",
						"run `lovart dev auth-login` for developer browser capture",
					},
				}))
				return nil
			}

			// 2. Create signer.
			signer, err := signing.NewSigner()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeSignerStale, "failed to load signer", map[string]any{
					"error": err.Error(),
				}))
				return nil
			}

			// 3. Create HTTP client.
			client := http.NewClient(creds, signer)

			// 4. Sync time.
			if err := client.SyncTime(ctx); err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "time sync failed", map[string]any{
					"error": err.Error(),
					"recommended_actions": []string{
						"check network connectivity to www.lovart.ai",
					},
				}))
				return nil
			}

			// 5. Fetch model list.
			models, err := discovery.List(ctx, client, true)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "failed to fetch model list", map[string]any{
					"error": err.Error(),
				}))
				return nil
			}

			printEnvelope(okPreflight(map[string]any{
				"source": "remote",
				"count":  len(models),
				"models": models,
			}, false))
			return nil
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh", false, "fetch current model list from Lovart API")
	return cmd
}

type registryModelSummary struct {
	Model       string `json:"model"`
	DisplayName string `json:"display_name,omitempty"`
	Type        string `json:"type,omitempty"`
}

func summarizeRegistryModels(records []registry.ModelRecord) []registryModelSummary {
	models := make([]registryModelSummary, 0, len(records))
	for _, record := range records {
		models = append(models, registryModelSummary{
			Model:       record.Model,
			DisplayName: record.DisplayName,
			Type:        record.Type,
		})
	}
	return models
}
