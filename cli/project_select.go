package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/project"
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
			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}
			projects, err := project.List(context.Background(), client)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "list projects", map[string]any{"error": err.Error()}))
				return nil
			}
			selectedProject, ok := project.FindByID(projects, projectID)
			if !ok {
				printEnvelope(envelope.Err(errors.CodeInputError, "project not found", map[string]any{
					"project_id": projectID,
					"recommended_actions": []string{
						"run `lovart project list`",
						"select a project_id from the returned projects",
					},
				}))
				return nil
			}
			if err := auth.SetProjectContext(projectID, cid); err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "set project", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(okPreflight(map[string]any{
				"selected":              true,
				"project_id":            selectedProject.ID,
				"project_name":          selectedProject.Name,
				"project_context_ready": cid != "",
			}))
			return nil
		},
	}
}
