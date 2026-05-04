package cli

import (
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/setup"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	var offline bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Check Lovart CLI readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			status := setup.Readiness(offline)
			printEnvelope(envelope.OK(map[string]any{
				"status":  "ok",
				"version": "2.0.0-dev",
				"ready":   status.Ready,
				"auth":    status.Auth,
				"signer":  status.Signer,
				"refs":    status.Refs,
				"warnings": status.Warnings,
			}))
			return nil
		},
	}

	cmd.Flags().BoolVar(&offline, "offline", false, "skip live checks")
	return cmd
}
