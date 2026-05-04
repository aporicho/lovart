package cli

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/project"
	"github.com/spf13/cobra"
)

func newProjectShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [project_id]",
		Short: "Show project details (defaults to current project)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := ""
			if len(args) > 0 {
				projectID = args[0]
			} else {
				pc, _ := auth.LoadProjectContext()
				if pc != nil {
					projectID = pc.ProjectID
				}
			}
			if projectID == "" {
				printEnvelope(envelope.Err(errors.CodeInputError, "no project specified", nil))
				return nil
			}

			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}
			pc, _ := auth.LoadProjectContext()
			cid := ""
			if pc != nil {
				cid = pc.CID
			}

			p, err := project.Query(context.Background(), client, projectID, cid)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "query project", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(envelope.OK(map[string]any{
				"project_id":   p.ID,
				"project_name": p.Name,
				"canvas_url":   fmt.Sprintf("https://www.lovart.ai/canvas?projectId=%s", p.ID),
			}))
			return nil
		},
	}
}
