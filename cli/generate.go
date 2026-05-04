package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/generation"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	var (
		bodyFile   string
		mode       string
		dryRun     bool
		allowPaid  bool
		maxCredits float64
		wait       bool
		download   bool
	)

	cmd := &cobra.Command{
		Use:   "generate <model> --body-file <file> [--mode auto|fast|relax]",
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

			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}

			// Get project context from stored credentials.
			projectID := ""
			cid := ""
			if pc, err := auth.LoadProjectContext(); err == nil && pc != nil {
				projectID = pc.ProjectID
				cid = pc.CID
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
					"submitted": false,
					"preflight": preflight,
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
				"submitted": true,
				"task_id":   result.TaskID,
				"status":    result.Status,
				"preflight": preflight,
			}

			// Wait for completion.
			if wait {
				task, err := generation.Wait(ctx, client, result.TaskID)
				if err != nil {
					output["poll_error"] = err.Error()
				} else {
					output["task"] = task
					output["status"] = task["status"]
				}
			}

			printEnvelope(envelope.OK(output))
			return nil
		},
	}

	cmd.Flags().StringVar(&bodyFile, "body-file", "", "path to request JSON file")
	cmd.Flags().StringVar(&mode, "mode", "auto", "generation mode: auto, fast, relax")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without submitting")
	cmd.Flags().BoolVar(&allowPaid, "allow-paid", false, "allow paid generation")
	cmd.Flags().Float64Var(&maxCredits, "max-credits", 0, "max credits to spend")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for task completion")
	cmd.Flags().BoolVar(&download, "download", false, "download artifacts")
	return cmd
}
