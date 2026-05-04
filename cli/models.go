package cli

import (
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List known Lovart generator models",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(map[string]any{
				"models": []any{},
				"status": "not implemented",
			}))
			return nil
		},
	}
}
