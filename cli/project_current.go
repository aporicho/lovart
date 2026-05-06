package cli

import (
	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/spf13/cobra"
)

func newProjectCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show current project context",
		RunE: func(cmd *cobra.Command, args []string) error {
			pc, err := auth.LoadProjectContext()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, "no project context", map[string]any{
					"error": err.Error(),
				}))
				return nil
			}
			printEnvelope(okLocal(map[string]any{
				"project_id": pc.ProjectID,
				"cid":        pc.CID,
			}))
			return nil
		},
	}
}
