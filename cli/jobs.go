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

	cmd.AddCommand(newJobsQuoteCmd())
	cmd.AddCommand(newJobsDryRunCmd())
	cmd.AddCommand(newJobsRunCmd())
	cmd.AddCommand(newJobsResumeCmd())
	cmd.AddCommand(newJobsStatusCmd())

	return cmd
}

func newJobsQuoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quote <jobs.jsonl>",
		Short: "Quote a batch jobs JSONL file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobsFile := args[0]
			ctx := context.Background()

			preparedJobs, validationErr, err := jobs.PrepareJobsFile(jobsFile)
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

			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}

			result, err := jobs.QuotePreparedJobs(ctx, client, preparedJobs, false)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "quote jobs", map[string]any{"error": err.Error()}))
				return nil
			}

			printEnvelope(okPreflight(result))
			return nil
		},
	}
}

func newJobsRunCmd() *cobra.Command {
	var opts jobs.JobsOptions
	var post jobPostprocessFlags
	cmd := &cobra.Command{
		Use:   "run <jobs.jsonl>",
		Short: "Submit batch generation",
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
			applyJobPostprocessFlags(&opts, post)
			applyProjectContext(&opts)
			state.ProjectID = opts.ProjectID
			state.CID = opts.CID
			if opts.ProjectID == "" || opts.CID == "" {
				printEnvelope(envelope.Err(errors.CodeInputError, "missing project context", map[string]any{
					"project_id":         opts.ProjectID,
					"cid_present":        opts.CID != "",
					"recommended_action": "pass --project-id and ensure cid is available from credentials, or run `lovart project select <project_id>`",
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
	addJobsRunFlags(cmd, &opts, &post)
	return cmd
}

func newJobsDryRunCmd() *cobra.Command {
	var opts jobs.JobsOptions
	cmd := &cobra.Command{
		Use:   "dry-run <jobs.jsonl>",
		Short: "Validate, quote, and gate a batch without submitting",
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
			remote, ok := newJobsRemote()
			if !ok {
				return nil
			}
			result, err := jobs.DryRunPreparedJobs(context.Background(), remote, state, opts)
			printJobsResult(result, err, "dry-run jobs", okPreflightSubmission)
			return nil
		},
	}
	addJobsCommonFlags(cmd, &opts)
	addJobsGateFlags(cmd, &opts)
	return cmd
}

func newJobsResumeCmd() *cobra.Command {
	var opts jobs.JobsOptions
	var post jobPostprocessFlags
	cmd := &cobra.Command{
		Use:   "resume <run_dir>",
		Short: "Resume an interrupted batch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runDir := args[0]
			applyJobPostprocessFlags(&opts, post)
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
	addJobsResumeFlags(cmd, &opts, &post)
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

func addJobsCommonFlags(cmd *cobra.Command, opts *jobs.JobsOptions) {
	cmd.Flags().StringVar(&opts.OutDir, "out-dir", "", "directory for batch state")
	cmd.Flags().StringVar(&opts.Detail, "detail", "summary", "output detail: summary, requests, full")
}

func addJobsGateFlags(cmd *cobra.Command, opts *jobs.JobsOptions) {
	cmd.Flags().BoolVar(&opts.AllowPaid, "allow-paid", false, "allow paid batch generation")
	cmd.Flags().Float64Var(&opts.MaxTotalCredits, "max-total-credits", 0, "max total credits allowed with --allow-paid")
}

type jobPostprocessFlags struct {
	noWait     bool
	noDownload bool
	noCanvas   bool
}

func addJobsRunFlags(cmd *cobra.Command, opts *jobs.JobsOptions, post *jobPostprocessFlags) {
	addJobsCommonFlags(cmd, opts)
	addJobsGateFlags(cmd, opts)
	cmd.Flags().BoolVar(&opts.Wait, "wait", true, "wait for submitted tasks")
	cmd.Flags().BoolVar(&post.noWait, "no-wait", false, "submit and return without waiting")
	cmd.Flags().BoolVar(&opts.Download, "download", true, "download artifacts after completion")
	cmd.Flags().BoolVar(&post.noDownload, "no-download", false, "skip artifact download")
	cmd.Flags().BoolVar(&opts.Canvas, "canvas", true, "add completed artifacts to the project canvas")
	cmd.Flags().BoolVar(&post.noCanvas, "no-canvas", false, "skip project canvas writeback")
	cmd.Flags().StringVar(&opts.CanvasLayout, "canvas-layout", jobs.CanvasLayoutFrame, "canvas layout for batch results: frame, plain")
	cmd.Flags().StringVar(&opts.DownloadDir, "download-dir", "", "directory for downloaded artifacts")
	cmd.Flags().StringVar(&opts.DownloadDirTemplate, "download-dir-template", "", "download subdirectory template")
	cmd.Flags().StringVar(&opts.DownloadFileTemplate, "download-file-template", "", "download filename template")
	cmd.Flags().Float64Var(&opts.TimeoutSeconds, "timeout-seconds", 3600, "local wait timeout in seconds")
	cmd.Flags().Float64Var(&opts.PollInterval, "poll-interval", 5, "task polling interval in seconds")
	cmd.Flags().StringVar(&opts.ProjectID, "project-id", "", "target project ID")
	cmd.Flags().StringVar(&opts.CID, "cid", "", "client id for project-bound generation")
}

func addJobsResumeFlags(cmd *cobra.Command, opts *jobs.JobsOptions, post *jobPostprocessFlags) {
	cmd.Flags().StringVar(&opts.Detail, "detail", "summary", "output detail: summary, requests, full")
	addJobsGateFlags(cmd, opts)
	cmd.Flags().BoolVar(&opts.Wait, "wait", true, "wait for submitted tasks")
	cmd.Flags().BoolVar(&post.noWait, "no-wait", false, "submit and return without waiting")
	cmd.Flags().BoolVar(&opts.Download, "download", true, "download artifacts after completion")
	cmd.Flags().BoolVar(&post.noDownload, "no-download", false, "skip artifact download")
	cmd.Flags().BoolVar(&opts.Canvas, "canvas", true, "add completed artifacts to the project canvas")
	cmd.Flags().BoolVar(&post.noCanvas, "no-canvas", false, "skip project canvas writeback")
	cmd.Flags().StringVar(&opts.CanvasLayout, "canvas-layout", jobs.CanvasLayoutFrame, "canvas layout for batch results: frame, plain")
	cmd.Flags().StringVar(&opts.DownloadDir, "download-dir", "", "directory for downloaded artifacts")
	cmd.Flags().StringVar(&opts.DownloadDirTemplate, "download-dir-template", "", "download subdirectory template")
	cmd.Flags().StringVar(&opts.DownloadFileTemplate, "download-file-template", "", "download filename template")
	cmd.Flags().Float64Var(&opts.TimeoutSeconds, "timeout-seconds", 3600, "local wait timeout in seconds")
	cmd.Flags().Float64Var(&opts.PollInterval, "poll-interval", 5, "task polling interval in seconds")
	cmd.Flags().StringVar(&opts.ProjectID, "project-id", "", "target project ID")
	cmd.Flags().StringVar(&opts.CID, "cid", "", "client id for project-bound generation")
}

func applyJobPostprocessFlags(opts *jobs.JobsOptions, post jobPostprocessFlags) {
	if post.noWait {
		opts.Wait = false
	}
	if post.noDownload {
		opts.Download = false
	}
	if post.noCanvas {
		opts.Canvas = false
	}
	if !opts.Wait {
		opts.Download = false
		opts.Canvas = false
		return
	}
	if opts.Download || opts.Canvas {
		opts.Wait = true
	}
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
		}))
		return
	}
	printEnvelope(envelope.Err(errors.CodeInternal, message, map[string]any{"error": err.Error()}))
}

func hasSubmittedJobs(result *jobs.BatchResult) bool {
	counts := result.Summary.RemoteStatusCounts
	return counts[jobs.StatusSubmitted] > 0 || counts[jobs.StatusRunning] > 0 || counts[jobs.StatusCompleted] > 0 || counts[jobs.StatusDownloaded] > 0
}
