package cli

import (
	"context"

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

			printEnvelope(envelope.OK(result))
			return nil
		},
	}
}

func newJobsRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <jobs.jsonl>",
		Short: "Submit batch generation",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(map[string]any{"status": "not implemented"}))
			return nil
		},
	}
}

func newJobsResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <jobs.jsonl>",
		Short: "Resume an interrupted batch",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(map[string]any{"status": "not implemented"}))
			return nil
		},
	}
}

func newJobsStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <run_dir>",
		Short: "Read batch run state",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(map[string]any{"status": "not implemented"}))
			return nil
		},
	}
}
