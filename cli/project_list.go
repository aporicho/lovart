package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/project"
	"github.com/spf13/cobra"
)

func newProjectListCmd() *cobra.Command {
	var live bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Lovart projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !live {
				printEnvelope(envelope.OK(map[string]any{
					"projects": []any{},
					"status":   "use --live to fetch from API",
				}))
				return nil
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
			printEnvelope(envelope.OK(map[string]any{
				"count":    len(projects),
				"projects": projects,
			}))
			return nil
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "fetch from Lovart API")
	return cmd
}
