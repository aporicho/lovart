package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/project"
	"github.com/spf13/cobra"
)

func newProjectRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <project_id> <new_name>",
		Short: "Rename a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			newName := args[1]

			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}

			if err := project.Rename(context.Background(), client, projectID, newName); err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "rename project", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(envelope.OK(map[string]any{
				"renamed":    true,
				"project_id": projectID,
				"name":       newName,
			}))
			return nil
		},
	}
}
