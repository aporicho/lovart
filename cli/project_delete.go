package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/project"
	"github.com/spf13/cobra"
)

func newProjectDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <project_id>",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]

			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}

			if err := project.Delete(context.Background(), client, projectID); err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "delete project", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(envelope.OK(map[string]any{
				"deleted":    true,
				"project_id": projectID,
			}))
			return nil
		},
	}
}
