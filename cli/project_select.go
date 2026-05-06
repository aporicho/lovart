package cli

import (
	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/spf13/cobra"
)

func newProjectSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select <project_id>",
		Short: "Select a project to bind generation tasks to",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			pc, _ := auth.LoadProjectContext()
			cid := ""
			if pc != nil {
				cid = pc.CID
			}
			if err := auth.SetProject(projectID, cid); err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "set project", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(okLocal(map[string]any{
				"selected":   true,
				"project_id": projectID,
			}))
			return nil
		},
	}
}
