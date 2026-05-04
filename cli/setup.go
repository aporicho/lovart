package cli

import (
	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
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
				"signer":  checkSigner(),
				"offline": offline,
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
		return map[string]any{"available": false, "error": err.Error()}
	}
	if err := s.Health(); err != nil {
		return map[string]any{"available": false, "error": err.Error()}
	}
	return map[string]any{"available": true}
}
