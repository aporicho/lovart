package cli

import (
	"github.com/aporicho/lovart/internal/config"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	var includeAll bool

	cmd := &cobra.Command{
		Use:   "config <model>",
		Short: "Return legal config values for a model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			model := args[0]

			result, err := config.ForModel(model)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeSchemaInvalid, "config lookup failed", map[string]any{"error": err.Error()}))
				return nil
			}

			if includeAll {
				printEnvelope(envelope.OK(result))
				return nil
			}

			// Filter to user-facing fields only.
			visibleFields := result.Fields[:0]
			for _, f := range result.Fields {
				if f.Type != "" {
					visibleFields = append(visibleFields, f)
				}
			}
			result.Fields = visibleFields

			printEnvelope(envelope.OK(result))
			return nil
		},
	}

	cmd.Flags().BoolVar(&includeAll, "all", false, "include all metadata fields")
	return cmd
}
