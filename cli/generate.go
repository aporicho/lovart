package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/downloads"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/generation"
	"github.com/aporicho/lovart/internal/pricing"
	"github.com/aporicho/lovart/internal/project"
	"github.com/aporicho/lovart/internal/registry"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	var (
		bodyFile             string
		prompt               string
		mode                 string
		projectID            string
		allowPaid            bool
		maxCredits           float64
		wait                 = true
		download             = true
		canvas               = true
		noWait               bool
		noDownload           bool
		noCanvas             bool
		downloadDir          string
		downloadDirTemplate  string
		downloadFileTemplate string
	)

	cmd := &cobra.Command{
		Use:   "generate <model> (--body-file <file>|--prompt <text>) [--project-id <id>] [--mode auto|fast|relax]",
		Short: "Submit a single generation request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			model := args[0]
			ctx := context.Background()

			body, err := loadBodyInput(bodyFile, prompt)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, "read body file", map[string]any{"error": err.Error()}))
				return nil
			}
			if validation := registry.ValidateRequest(model, body); !validation.OK {
				printEnvelope(envelope.Err(validationErrorCode(validation), "request body failed schema validation", map[string]any{
					"validation":          validation,
					"recommended_actions": validationRecommendedActions(validation),
				}))
				return nil
			}
			if _, err := pricing.NormalizeMode(mode); err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, "invalid mode", map[string]any{"error": err.Error()}))
				return nil
			}

			// Resolve project context. Explicit flags are per-command overrides and
			// do not mutate the stored current project.
			cid := ""
			if pc, err := auth.LoadProjectContext(); err == nil && pc != nil {
				if projectID == "" {
					projectID = pc.ProjectID
				}
				cid = pc.CID
			}

			if projectID == "" || cid == "" {
				printEnvelope(envelope.Err(errors.CodeInputError, "missing project context", map[string]any{
					"project_id":            projectID,
					"project_context_ready": false,
					"recommended_actions": []string{
						"run `lovart auth login`",
						"run `lovart project list`",
						"run `lovart project select <project_id>`",
					},
				}))
				return nil
			}
			if noWait {
				wait = false
			}
			if noDownload {
				download = false
			}
			if noCanvas {
				canvas = false
			}
			if !wait {
				download = false
				canvas = false
			}
			if download || canvas {
				wait = true
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
				"submitted":       true,
				"task_id":         result.TaskID,
				"status":          result.Status,
				"normalized_body": result.NormalizedBody,
				"preflight":       preflight,
				"project_id":      projectID,
			}
			downloadBody := result.NormalizedBody
			if len(downloadBody) == 0 {
				downloadBody = body
			}

			// Wait for completion.
			var warnings []string
			if wait {
				task, err := generation.Wait(ctx, client, result.TaskID)
				if err != nil {
					output["poll_error"] = err.Error()
					warnings = append(warnings, "task was submitted but polling failed; rerun a status or resume-capable command when available")
				} else {
					output["task"] = task
					output["status"] = task["status"]
					if task["status"] == "failed" {
						printEnvelope(envelope.Err(errors.CodeTaskFailed, "generation task failed", map[string]any{
							"task_id": result.TaskID,
							"task":    task,
						}))
						return nil
					}

					if task["status"] == "completed" && download {
						downloadResult, err := downloads.DownloadArtifacts(ctx, downloads.ArtifactsFromTask(task), downloads.Options{
							RootDir:      downloadDir,
							DirTemplate:  downloadDirTemplate,
							FileTemplate: downloadFileTemplate,
							TaskID:       result.TaskID,
							Context: downloads.JobContext{
								Model: model,
								Mode:  mode,
								Body:  downloadBody,
							},
						})
						if err != nil {
							output["download_error"] = err.Error()
							warnings = append(warnings, "artifacts were generated but download failed; rerun generation with downloads disabled or retry artifact download when available")
						} else {
							output["downloads"] = downloadResult.Files
							if downloadResult.IndexError != "" {
								output["download_index_error"] = downloadResult.IndexError
								warnings = append(warnings, "artifacts were downloaded but the download index could not be fully written")
							}
						}
					}

					// Add generated images to project canvas.
					if task["status"] == "completed" && canvas && projectID != "" && cid != "" {
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
								warnings = append(warnings, "artifacts were generated but project canvas writeback failed")
							} else {
								output["canvas_updated"] = true
							}
						}
					}
				}
			}

			env := okSubmit(output, true)
			env.Warnings = warnings
			printEnvelope(env)
			return nil
		},
	}

	cmd.Flags().StringVar(&bodyFile, "body-file", "", "path to request JSON file")
	cmd.Flags().StringVar(&prompt, "prompt", "", "prompt text for a minimal generation request")
	cmd.Flags().StringVar(&mode, "mode", "auto", "generation mode: auto, fast, relax")
	cmd.Flags().StringVar(&projectID, "project-id", "", "target project ID (defaults to current project context)")
	cmd.Flags().BoolVar(&allowPaid, "allow-paid", false, "allow paid generation")
	cmd.Flags().Float64Var(&maxCredits, "max-credits", 0, "max credits to spend")
	cmd.Flags().BoolVar(&wait, "wait", true, "wait for task completion")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "submit and return without waiting")
	cmd.Flags().BoolVar(&download, "download", true, "download artifacts")
	cmd.Flags().BoolVar(&noDownload, "no-download", false, "skip artifact download")
	cmd.Flags().BoolVar(&canvas, "canvas", true, "add completed artifacts to the project canvas")
	cmd.Flags().BoolVar(&noCanvas, "no-canvas", false, "skip project canvas writeback")
	cmd.Flags().StringVar(&downloadDir, "download-dir", "", "directory for downloaded artifacts")
	cmd.Flags().StringVar(&downloadDirTemplate, "download-dir-template", "", "download subdirectory template")
	cmd.Flags().StringVar(&downloadFileTemplate, "download-file-template", "", "download filename template")
	return cmd
}
