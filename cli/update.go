package cli

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/update"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Metadata drift detection and sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Check for metadata drift",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := update.Check(context.Background())
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeNetworkUnavailable, "update check failed", map[string]any{
					"error":               err.Error(),
					"recommended_actions": []string{"check network connectivity to www.lovart.ai"},
				}))
				return nil
			}
			printEnvelope(okPreflight(result, true))
			return nil
		},
	})

	cmd.AddCommand(newUpdateSyncCmd())

	return cmd
}

func newUpdateSyncCmd() *cobra.Command {
	var (
		signerOnly   bool
		metadataOnly bool
		all          bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Refresh runtime signer and metadata caches",
		RunE: func(cmd *cobra.Command, args []string) error {
			selected := 0
			for _, flag := range []bool{signerOnly, metadataOnly, all} {
				if flag {
					selected++
				}
			}
			if selected > 1 {
				printEnvelope(envelope.Err(errors.CodeInputError, "choose only one update sync mode", map[string]any{
					"valid_modes": []string{"--all", "--signer", "--metadata-only"},
				}))
				return nil
			}
			if selected == 0 {
				all = true
			}

			ctx := context.Background()
			switch {
			case signerOnly:
				result, err := update.SyncSigner(ctx)
				return printUpdateSyncResult(result, err)
			case metadataOnly:
				result, err := update.SyncMetadata(ctx)
				return printUpdateSyncResult(result, err)
			case all:
				result, err := update.SyncAll(ctx)
				return printUpdateSyncResult(result, err)
			default:
				return fmt.Errorf("unreachable update sync mode")
			}
		},
	}
	cmd.Flags().BoolVar(&signerOnly, "signer", false, "refresh only the runtime signer WASM")
	cmd.Flags().BoolVar(&metadataOnly, "metadata-only", false, "refresh only signed generator metadata")
	cmd.Flags().BoolVar(&all, "all", false, "refresh signer first, then generator metadata")
	return cmd
}

func printUpdateSyncResult(result any, err error) error {
	if err != nil {
		code := errors.CodeInternal
		if update.IsSignerMissing(err) {
			code = errors.CodeSignerStale
		}
		printEnvelope(envelope.Err(code, "update sync failed", map[string]any{
			"error":               err.Error(),
			"recommended_actions": []string{"run `lovart update sync --all`"},
		}))
		return nil
	}
	printEnvelope(okPreflight(result, true))
	return nil
}
