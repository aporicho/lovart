package cli

import (
	"github.com/aporicho/lovart/internal/cleanup"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var opts cleanup.Options
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Preview or remove Lovart local data",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			hasScope := cleanHasScope(opts)
			if !hasScope {
				opts.All = true
				opts.DryRun = true
			}
			if hasScope && !opts.DryRun && !opts.Yes {
				printEnvelope(envelope.Err(errors.CodeInputError, "clean requires --yes", map[string]any{
					"recommended_actions": []string{"rerun with --dry-run to preview", "rerun with --yes to delete selected data"},
				}))
				return nil
			}
			result, err := cleanup.Run(opts)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "clean failed", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(okLocal(result, true))
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.Runs, "runs", false, "select generated run records")
	cmd.Flags().BoolVar(&opts.Downloads, "downloads", false, "select downloaded artifacts")
	cmd.Flags().BoolVar(&opts.Cache, "cache", false, "select metadata, signer, and temporary caches")
	cmd.Flags().BoolVar(&opts.Auth, "auth", false, "select stored Lovart credentials")
	cmd.Flags().BoolVar(&opts.Extension, "extension", false, "select installed browser extension files")
	cmd.Flags().BoolVar(&opts.All, "all", false, "select all Lovart runtime data except binary and MCP configs")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "preview selected data without deleting it")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "confirm deletion of selected data")
	return cmd
}

func cleanHasScope(opts cleanup.Options) bool {
	return opts.Runs || opts.Downloads || opts.Cache || opts.Auth || opts.Extension || opts.All
}
