package cli

import (
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate <model> --body-file <file>",
		Short: "Submit a single generation request",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(map[string]any{
				"submitted": false,
				"status":    "not implemented",
			}))
			return nil
		},
	}
}
