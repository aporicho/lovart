package cli

import (
	"runtime"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/spf13/cobra"
)

func versionData() map[string]any {
	return map[string]any{
		"package":    "lovart",
		"version":    "2.0.0-dev",
		"go_version": runtime.Version(),
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(versionData()))
			return nil
		},
	}
}
