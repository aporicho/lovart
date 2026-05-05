package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/project"
	"github.com/spf13/cobra"
)

func newProjectRepairCanvasCmd() *cobra.Command {
	var cid string
	cmd := &cobra.Command{
		Use:   "repair-canvas [project_id]",
		Short: "Normalize and repair a Lovart project canvas",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := ""
			if len(args) > 0 {
				projectID = args[0]
			}
			pc, _ := auth.LoadProjectContext()
			if pc != nil {
				if projectID == "" {
					projectID = pc.ProjectID
				}
				if cid == "" {
					cid = pc.CID
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
			result, err := project.RepairCanvas(context.Background(), client, projectID, cid)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "repair canvas", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(envelope.OK(map[string]any{
				"project_id": projectID,
				"repair":     result,
			}))
			return nil
		},
	}
	cmd.Flags().StringVar(&cid, "cid", "", "client id for project-bound repair")
	return cmd
}
