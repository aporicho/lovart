package cli

import (
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config [model]",
		Short: "Return legal config values for a model",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			model := ""
			if len(args) > 0 {
				model = args[0]
			}
			printEnvelope(envelope.OK(map[string]any{
				"model":  model,
				"fields": []any{},
				"status": "not implemented",
			}))
			return nil
		},
	}
}
