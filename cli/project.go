package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/signing"
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
	cmd.AddCommand(newProjectListCmd())
	cmd.AddCommand(newProjectCreateCmd())
	cmd.AddCommand(newProjectSelectCmd())
	cmd.AddCommand(newProjectShowCmd())
	cmd.AddCommand(newProjectOpenCmd())
	cmd.AddCommand(newProjectAdminCmd())
	return cmd
}

func newProjectAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Advanced project maintenance commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newProjectRenameCmd())
	cmd.AddCommand(newProjectDeleteCmd())
	cmd.AddCommand(newProjectRepairCanvasCmd())
	return cmd
}

// newSignedClient creates an authenticated and time-synced HTTP client.
func newSignedClient() (*http.Client, error) {
	creds, err := auth.Load()
	if err != nil {
		return nil, err
	}
	signer, err := signing.NewSigner()
	if err != nil {
		return nil, err
	}
	client := http.NewClient(creds, signer)
	if err := client.SyncTime(context.Background()); err != nil {
		return nil, err
	}
	return client, nil
}
