package cli

import (
	"github.com/aporicho/lovart/internal/version"
	"github.com/spf13/cobra"
)

func versionData() map[string]any {
	return version.Data()
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(okLocal(versionData()))
			return nil
		},
	}
}
