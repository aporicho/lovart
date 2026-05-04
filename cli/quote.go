package cli

import (
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/spf13/cobra"
)

func newQuoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quote <model> --body-file <file>",
		Short: "Fetch exact Lovart credit quote for a model request",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(map[string]any{
				"quoted": false,
				"status": "not implemented",
			}))
			return nil
		},
	}
}
