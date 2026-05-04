package cli

import (
	"fmt"
	"os/exec"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/spf13/cobra"
)

func newProjectOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [project_id]",
		Short: "Open project in browser (defaults to current project)",
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

			url := fmt.Sprintf("https://www.lovart.ai/canvas?projectId=%s", projectID)
			err := exec.Command("open", url).Start()
			printEnvelope(envelope.OK(map[string]any{
				"opened":     err == nil,
				"project_id": projectID,
				"url":        url,
			}))
			return nil
		},
	}
}
