package cli

import (
	"github.com/aporicho/lovart/internal/selftest"
	"github.com/spf13/cobra"
)

func newSelfTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "self-test",
		Short: "Run self-diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(okLocal(selftest.Run(), true))
			return nil
		},
	}
}
