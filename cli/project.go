package cli

import (
	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage Lovart project context",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newProjectCurrentCmd())
	return cmd
}

func newProjectCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show current project context",
		RunE: func(cmd *cobra.Command, args []string) error {
			pc, err := auth.LoadProjectContext()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, "no project context", map[string]any{
					"error": err.Error(),
					"recommended_actions": []string{
						"run make capture, login to Lovart, open a project",
						"then run make extract to save project context",
					},
				}))
				return nil
			}
			printEnvelope(envelope.OK(map[string]any{
				"project_id": pc.ProjectID,
				"cid":        pc.CID,
			}))
			return nil
		},
	}
}
