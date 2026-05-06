package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/project"
	"github.com/spf13/cobra"
)

func newProjectCreateCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Lovart project",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			p, err := project.Create(context.Background(), client, cid, name)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "create project", map[string]any{"error": err.Error()}))
				return nil
			}
			auth.SetProject(p.ID, cid)
			printEnvelope(okRemoteWrite(map[string]any{
				"created":    true,
				"project_id": p.ID,
				"name":       p.Name,
			}))
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name")
	return cmd
}
