package cli

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/project"
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
	cmd.AddCommand(newProjectRenameCmd())
	cmd.AddCommand(newProjectDeleteCmd())
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
			// Auto-bind.
			auth.SetProject(p.ID, cid)
			printEnvelope(envelope.OK(map[string]any{
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

func newProjectSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select <project_id>",
		Short: "Select a project to bind generation tasks to",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			if err := auth.SetProject(projectID, ""); err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "set project", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(envelope.OK(map[string]any{
				"selected":   true,
				"project_id": projectID,
			}))
			return nil
		},
	}
}

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
				"canvas_url":   fmt.Sprintf("https://www.lovart.ai/canvas/projectId/%s", p.ID),
			}))
			return nil
		},
	}
}

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

			url := fmt.Sprintf("https://www.lovart.ai/canvas/projectId/%s", projectID)
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
