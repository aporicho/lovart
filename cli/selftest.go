package cli

import (
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/spf13/cobra"
)

func newSelfTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "self-test",
		Short: "Run self-diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(map[string]any{"status": "not implemented"}))
			return nil
		},
	}
}
