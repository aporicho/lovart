package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/generation"
	"github.com/aporicho/lovart/internal/project"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	var (
		bodyFile   string
		mode       string
		projectID  string
		cid        string
		dryRun     bool
		allowPaid  bool
		maxCredits float64
		wait       bool
		download   bool
	)

	cmd := &cobra.Command{
		Use:   "generate <model> --body-file <file> [--project-id <id>] [--mode auto|fast|relax]",
		Short: "Submit a single generation request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			model := args[0]
			ctx := context.Background()

			body, err := loadBodyFile(bodyFile)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, "read body file", map[string]any{"error": err.Error()}))
				return nil
			}

			// Resolve project context. Explicit flags are per-command overrides and
			// do not mutate the stored current project.
			if pc, err := auth.LoadProjectContext(); err == nil && pc != nil {
				if projectID == "" {
					projectID = pc.ProjectID
				}
				if cid == "" {
					cid = pc.CID
				}
			}

			if !dryRun && (projectID == "" || cid == "") {
				printEnvelope(envelope.Err(errors.CodeInputError, "missing project context", map[string]any{
					"project_id":         projectID,
					"cid_present":        cid != "",
					"recommended_action": "pass --project-id and ensure cid is available from credentials, or run `lovart project select <project_id>`",
				}))
				return nil
			}

			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}

			opts := generation.Options{
				Mode:       mode,
				AllowPaid:  allowPaid,
				MaxCredits: maxCredits,
				ProjectID:  projectID,
				CID:        cid,
				Wait:       wait,
				Download:   download,
			}

			// Preflight.
			preflight, err := generation.Preflight(ctx, client, model, body, opts)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "preflight error", map[string]any{"error": err.Error()}))
				return nil
			}

			if dryRun {
				printEnvelope(envelope.OK(map[string]any{
					"submitted":  false,
					"preflight":  preflight,
					"project_id": projectID,
				}))
				return nil
			}

			if !preflight.CanSubmit {
				printEnvelope(envelope.Err(errors.CodeCreditRisk, "cannot submit", map[string]any{
					"preflight": preflight,
				}))
				return nil
			}

			// Submit.
			result, err := generation.Submit(ctx, client, model, body, opts)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "submit failed", map[string]any{"error": err.Error()}))
				return nil
			}

			output := map[string]any{
				"submitted":  true,
				"task_id":    result.TaskID,
				"status":     result.Status,
				"preflight":  preflight,
				"project_id": projectID,
			}

			// Wait for completion.
			if wait {
				task, err := generation.Wait(ctx, client, result.TaskID)
				if err != nil {
					output["poll_error"] = err.Error()
				} else {
					output["task"] = task
					output["status"] = task["status"]

					// Add generated images to project canvas.
					if task["status"] == "completed" && projectID != "" && cid != "" {
						details, _ := task["artifact_details"].([]map[string]any)
						var images []project.CanvasImage
						for _, d := range details {
							url, _ := d["url"].(string)
							w, _ := d["width"].(float64)
							h, _ := d["height"].(float64)
							if url != "" {
								if w == 0 {
									w = 1024
								}
								if h == 0 {
									h = 1024
								}
								images = append(images, project.CanvasImage{
									TaskID: result.TaskID,
									URL:    url,
									Width:  int(w),
									Height: int(h),
								})
							}
						}
						if len(images) > 0 {
							if err := project.AddToCanvas(ctx, client, projectID, cid, images); err != nil {
								output["canvas_error"] = err.Error()
							} else {
								output["canvas_updated"] = true
							}
						}
					}
				}
			}

			printEnvelope(envelope.OK(output))
			return nil
		},
	}

	cmd.Flags().StringVar(&bodyFile, "body-file", "", "path to request JSON file")
	cmd.Flags().StringVar(&mode, "mode", "auto", "generation mode: auto, fast, relax")
	cmd.Flags().StringVar(&projectID, "project-id", "", "target project ID (defaults to current project context)")
	cmd.Flags().StringVar(&cid, "cid", "", "client id for project-bound generation (defaults to stored cid)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without submitting")
	cmd.Flags().BoolVar(&allowPaid, "allow-paid", false, "allow paid generation")
	cmd.Flags().Float64Var(&maxCredits, "max-credits", 0, "max credits to spend")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for task completion")
	cmd.Flags().BoolVar(&download, "download", false, "download artifacts")
	return cmd
}
