package cli

import (
	"context"
	stderrors "errors"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/jobs"
	"github.com/spf13/cobra"
)

func newJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Batch generation commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newJobsRunCmd())
	cmd.AddCommand(newJobsResumeCmd())
	cmd.AddCommand(newJobsStatusCmd())

	return cmd
}

func newJobsRunCmd() *cobra.Command {
	opts := defaultBatchOptions()
	cmd := &cobra.Command{
		Use:   "run <jobs.jsonl>",
		Short: "Run a complete batch generation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobsFile := args[0]
			state, validationErr, err := jobs.PrepareRun(jobsFile, opts)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, "read jobs file", map[string]any{"error": err.Error()}))
				return nil
			}
			if validationErr != nil {
				printEnvelope(envelope.Err(jobValidationErrorCode(validationErr), "jobs file failed schema validation", map[string]any{
					"validation":          validationErr,
					"recommended_actions": jobValidationRecommendedActions(validationErr),
				}))
				return nil
			}
			applyProjectContext(&opts)
			state.ProjectID = opts.ProjectID
			state.CID = opts.CID
			if opts.ProjectID == "" || opts.CID == "" {
				printEnvelope(envelope.Err(errors.CodeInputError, "missing project context", map[string]any{
					"project_id":            opts.ProjectID,
					"project_context_ready": false,
					"recommended_actions": []string{
						"run `lovart auth login`",
						"run `lovart project list`",
						"run `lovart project select <project_id>`",
					},
				}))
				return nil
			}
			remote, ok := newJobsRemote()
			if !ok {
				return nil
			}
			result, err := jobs.RunPreparedJobs(context.Background(), remote, state, opts)
			printJobsResult(result, err, "run jobs", okSubmit)
			return nil
		},
	}
	addJobsRunFlags(cmd, &opts)
	return cmd
}

func newJobsResumeCmd() *cobra.Command {
	opts := defaultBatchOptions()
	cmd := &cobra.Command{
		Use:   "resume <run_dir>",
		Short: "Resume a batch generation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runDir := args[0]
			applyProjectContext(&opts)
			remote, ok := newJobsRemote()
			if !ok {
				return nil
			}
			result, err := jobs.ResumeJobs(context.Background(), remote, runDir, opts)
			printJobsResult(result, err, "resume jobs", okSubmit)
			return nil
		},
	}
	addJobsResumeFlags(cmd, &opts)
	cmd.Flags().BoolVar(&opts.RetryFailed, "retry-failed", false, "retry failed requests that were never submitted")
	return cmd
}

func newJobsStatusCmd() *cobra.Command {
	var opts jobs.JobsOptions
	cmd := &cobra.Command{
		Use:   "status <run_dir>",
		Short: "Read batch run state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runDir := args[0]
			var remote jobs.RemoteClient
			if opts.Refresh {
				var ok bool
				remote, ok = newJobsRemote()
				if !ok {
					return nil
				}
			}
			result, err := jobs.StatusJobs(context.Background(), remote, runDir, opts)
			if opts.Refresh {
				printJobsResult(result, err, "status jobs", func(data any, _ bool) envelope.Envelope {
					return okPreflight(data)
				})
			} else {
				printJobsResult(result, err, "status jobs", func(data any, _ bool) envelope.Envelope {
					return okLocal(data)
				})
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Detail, "detail", "summary", "output detail: summary, requests, full")
	cmd.Flags().BoolVar(&opts.Refresh, "refresh", false, "refresh active remote task statuses")
	return cmd
}

func defaultBatchOptions() jobs.JobsOptions {
	return jobs.JobsOptions{
		Wait:           true,
		Download:       true,
		Canvas:         true,
		CanvasLayout:   jobs.CanvasLayoutFrame,
		TimeoutSeconds: 3600,
		PollInterval:   5,
		Detail:         "summary",
	}
}

func addJobsGateFlags(cmd *cobra.Command, opts *jobs.JobsOptions) {
	cmd.Flags().BoolVar(&opts.AllowPaid, "allow-paid", false, "allow paid batch generation")
	cmd.Flags().Float64Var(&opts.MaxTotalCredits, "max-total-credits", 0, "max total credits allowed with --allow-paid")
}

func addJobsRunFlags(cmd *cobra.Command, opts *jobs.JobsOptions) {
	addJobsGateFlags(cmd, opts)
	cmd.Flags().StringVar(&opts.DownloadDir, "download-dir", "", "directory for downloaded artifacts")
	cmd.Flags().StringVar(&opts.ProjectID, "project-id", "", "target project ID")
}

func addJobsResumeFlags(cmd *cobra.Command, opts *jobs.JobsOptions) {
	addJobsGateFlags(cmd, opts)
	cmd.Flags().StringVar(&opts.DownloadDir, "download-dir", "", "directory for downloaded artifacts")
}

func newJobsRemote() (jobs.RemoteClient, bool) {
	client, err := newSignedClient()
	if err != nil {
		printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
		return nil, false
	}
	return jobs.NewHTTPRemoteClient(client), true
}

func applyProjectContext(opts *jobs.JobsOptions) {
	pc, _ := auth.LoadProjectContext()
	if pc == nil {
		return
	}
	if opts.ProjectID == "" {
		opts.ProjectID = pc.ProjectID
	}
	if opts.CID == "" {
		opts.CID = pc.CID
	}
}

func printJobsResult(result *jobs.BatchResult, err error, message string, okFn func(any, bool) envelope.Envelope) {
	if err == nil {
		printEnvelope(okFn(result, result != nil && hasSubmittedJobs(result)))
		return
	}
	var validationErr *jobs.ValidationError
	if stderrors.As(err, &validationErr) {
		printEnvelope(envelope.Err(jobValidationErrorCode(validationErr), "jobs file failed schema validation", map[string]any{
			"validation":          validationErr,
			"recommended_actions": jobValidationRecommendedActions(validationErr),
		}))
		return
	}
	var gateErr *jobs.GateError
	if stderrors.As(err, &gateErr) {
		printEnvelope(envelope.Err(gateErr.Code, "batch gate blocked", map[string]any{
			"batch_gate": gateErr.Gate,
			"run_dir":    gateErr.RunDir,
			"state_file": gateErr.StateFile,
		}))
		return
	}
	printEnvelope(envelope.Err(errors.CodeInternal, message, map[string]any{"error": err.Error()}))
}

func hasSubmittedJobs(result *jobs.BatchResult) bool {
	counts := result.Summary.RemoteStatusCounts
	return counts[jobs.StatusSubmitted] > 0 || counts[jobs.StatusRunning] > 0 || counts[jobs.StatusCompleted] > 0 || counts[jobs.StatusDownloaded] > 0
}
