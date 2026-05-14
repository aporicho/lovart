package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/project"
	"github.com/spf13/cobra"
)

func newProjectArtifactsCmd() *cobra.Command {
	var (
		taskID string
		limit  int
		offset int
		detail string
	)
	cmd := &cobra.Command{
		Use:   "artifacts [project_id]",
		Short: "List downloadable image artifacts on a project canvas",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := ""
			if len(args) > 0 {
				projectID = args[0]
			}
			projectID, cid, env := resolveCLIProjectContext(projectID)
			if env != nil {
				printEnvelope(*env)
				return nil
			}
			if detail != "summary" && detail != "full" {
				printEnvelope(envelope.Err(errors.CodeInputError, "invalid detail", map[string]any{"detail": detail}))
				return nil
			}
			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}
			result, err := project.ListCanvasArtifacts(context.Background(), client, projectID, cid, project.CanvasArtifactsOptions{
				TaskID:     taskID,
				Limit:      limit,
				Offset:     offset,
				IncludeRaw: detail == "full",
			})
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "list canvas artifacts", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(okPreflight(result))
			return nil
		},
	}
	cmd.Flags().StringVar(&taskID, "task-id", "", "filter artifacts by generation task ID")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of artifacts to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "number of artifacts to skip")
	cmd.Flags().StringVar(&detail, "detail", "summary", "output detail: summary, full")
	return cmd
}

func newProjectArtifactCmd() *cobra.Command {
	var (
		projectID  string
		includeRaw bool
	)
	cmd := &cobra.Command{
		Use:   "artifact <artifact_id>",
		Short: "Show one downloadable canvas artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			artifactID := args[0]
			projectID, cid, env := resolveCLIProjectContext(projectID)
			if env != nil {
				printEnvelope(*env)
				return nil
			}
			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}
			result, err := project.GetCanvasArtifact(context.Background(), client, projectID, cid, artifactID, includeRaw)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, "canvas artifact not found", map[string]any{
					"artifact_id": artifactID,
					"error":       err.Error(),
				}))
				return nil
			}
			printEnvelope(okPreflight(result))
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project-id", "", "target project ID (defaults to current project context)")
	cmd.Flags().BoolVar(&includeRaw, "include-raw", false, "include raw canvas shape JSON")
	return cmd
}

func resolveCLIProjectContext(projectID string) (string, string, *envelope.Envelope) {
	cid := ""
	if pc, err := auth.LoadProjectContext(); err == nil && pc != nil {
		if projectID == "" {
			projectID = pc.ProjectID
		}
		cid = pc.CID
	}
	if projectID == "" || cid == "" {
		env := envelope.Err(errors.CodeInputError, "missing project context", map[string]any{
			"project_id":            projectID,
			"project_context_ready": false,
			"recommended_actions": []string{
				"run `lovart auth login`",
				"run `lovart project list`",
				"run `lovart project select <project_id>`",
			},
		})
		return "", "", &env
	}
	return projectID, cid, nil
}
