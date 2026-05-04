package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/discovery"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/signing"
	"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
	var live bool

	cmd := &cobra.Command{
		Use:   "models",
		Short: "List known Lovart generator models",
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if !live {
				printEnvelope(envelope.OK(map[string]any{
					"models": []any{},
					"source": "ref",
					"status": "offline mode — use --live to fetch",
				}))
				return nil
			}

			// 1. Load credentials.
			creds, err := auth.Load()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeAuthMissing, "no credentials found", map[string]any{
					"error": err.Error(),
					"recommended_actions": []string{
						"run `lovart-reverse start` to capture auth from browser",
						"run `lovart-reverse auth extract captures/<file>.json` to extract credentials",
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

			printEnvelope(envelope.OK(map[string]any{
				"source": "live",
				"count":  len(models),
				"models": models,
			}))
			return nil
		},
	}

	cmd.Flags().BoolVar(&live, "live", false, "fetch live model list from Lovart API")
	return cmd
}
