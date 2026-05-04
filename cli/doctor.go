package cli

import (
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run architecture integrity checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(map[string]any{"status": "not implemented"}))
			return nil
		},
	}
}
