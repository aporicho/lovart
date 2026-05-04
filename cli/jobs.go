package cli

import (
	"github.com/aporicho/lovart/internal/envelope"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(envelope.OK(map[string]any{"status": "not implemented"}))
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
