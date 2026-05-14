package cli

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/generation"
	"github.com/aporicho/lovart/internal/project"
	"github.com/spf13/cobra"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Inspect Lovart generation tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newTaskListCmd())
	cmd.AddCommand(newTaskStatusCmd())
	cmd.AddCommand(newTaskWaitCmd())
	cmd.AddCommand(newTaskCanvasCmd())
	cmd.AddCommand(newTaskCancelCmd())
	return cmd
}

func newTaskListCmd() *cobra.Command {
	var active bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active generation tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !active {
				printEnvelope(envelope.Err(errors.CodeInputError, "only active task listing is supported", map[string]any{"active": active}))
				return nil
			}
			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}
			result, err := generation.ListRunningTasks(context.Background(), client)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "list active tasks", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(okPreflight(result))
			return nil
		},
	}
	cmd.Flags().BoolVar(&active, "active", true, "list active running tasks")
	return cmd
}

func newTaskStatusCmd() *cobra.Command {
	var detail string
	cmd := &cobra.Command{
		Use:   "status <task_id>",
		Short: "Read a generation task status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if detail != "summary" && detail != "full" {
				printEnvelope(envelope.Err(errors.CodeInputError, "invalid detail", map[string]any{"detail": detail}))
				return nil
			}
			taskID := args[0]
			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}
			task, err := generation.FetchTask(context.Background(), client, taskID)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "fetch task", map[string]any{"error": err.Error(), "task_id": taskID}))
				return nil
			}
			printEnvelope(okPreflight(generation.TaskView(task, detail)))
			return nil
		},
	}
	cmd.Flags().StringVar(&detail, "detail", "summary", "output detail: summary, full")
	return cmd
}

func newTaskWaitCmd() *cobra.Command {
	var (
		detail         string
		timeoutSeconds float64
		pollInterval   float64
	)
	cmd := &cobra.Command{
		Use:   "wait <task_id>",
		Short: "Wait for a generation task to finish",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if detail != "summary" && detail != "full" {
				printEnvelope(envelope.Err(errors.CodeInputError, "invalid detail", map[string]any{"detail": detail}))
				return nil
			}
			if timeoutSeconds <= 0 {
				printEnvelope(envelope.Err(errors.CodeInputError, "timeout-seconds must be positive", map[string]any{"timeout_seconds": timeoutSeconds}))
				return nil
			}
			if pollInterval <= 0 {
				printEnvelope(envelope.Err(errors.CodeInputError, "poll-interval must be positive", map[string]any{"poll_interval": pollInterval}))
				return nil
			}
			taskID := args[0]
			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
			defer cancel()
			task, err := generation.WaitWithOptions(ctx, client, taskID, generation.WaitOptions{PollInterval: time.Duration(pollInterval * float64(time.Second))})
			if err != nil {
				code := errors.CodeInternal
				message := "wait task"
				if stderrors.Is(err, context.DeadlineExceeded) {
					code = errors.CodeTimeout
					message = "task wait timed out"
				}
				printEnvelope(envelope.Err(code, message, map[string]any{
					"error":           err.Error(),
					"task_id":         taskID,
					"timeout_seconds": timeoutSeconds,
				}))
				return nil
			}
			printEnvelope(okPreflight(generation.TaskView(task, detail)))
			return nil
		},
	}
	cmd.Flags().StringVar(&detail, "detail", "summary", "output detail: summary, full")
	cmd.Flags().Float64Var(&timeoutSeconds, "timeout-seconds", 3600, "seconds to wait for task completion")
	cmd.Flags().Float64Var(&pollInterval, "poll-interval", 2, "seconds between status polls")
	return cmd
}

func newTaskCanvasCmd() *cobra.Command {
	var (
		projectID string
		detail    string
	)
	cmd := &cobra.Command{
		Use:   "canvas <task_id>",
		Short: "Write completed task artifacts to the project canvas",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if detail != "summary" && detail != "full" {
				printEnvelope(envelope.Err(errors.CodeInputError, "invalid detail", map[string]any{"detail": detail}))
				return nil
			}
			taskID := args[0]
			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}
			task, err := generation.FetchTask(context.Background(), client, taskID)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "fetch task", map[string]any{"error": err.Error(), "task_id": taskID}))
				return nil
			}
			normalizedStatus := generation.NormalizeTaskStatus(task)
			if normalizedStatus == generation.TaskStatusFailed {
				printEnvelope(taskFailureEnvelope(taskID, task, detail))
				return nil
			}
			status, _ := task["status"].(string)
			if normalizedStatus != generation.TaskStatusCompleted {
				printEnvelope(envelope.Err(errors.CodeInputError, "task is not completed", map[string]any{
					"task_id": taskID,
					"status":  status,
					"recommended_actions": []string{
						"run `lovart task wait " + taskID + "`",
						"run `lovart task status " + taskID + "`",
					},
				}))
				return nil
			}
			projectID, cid, env := resolveCLIProjectContext(projectID)
			if env != nil {
				printEnvelope(*env)
				return nil
			}
			images := project.CanvasImagesFromTask(taskID, task)
			if len(images) == 0 {
				printEnvelope(envelope.Err(errors.CodeInputError, "no canvas images found for task", map[string]any{
					"task_id": taskID,
					"task":    generation.TaskView(task, detail),
				}))
				return nil
			}
			if err := project.AddToCanvas(context.Background(), client, projectID, cid, images); err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "write task artifacts to canvas", map[string]any{"error": err.Error(), "task_id": taskID, "project_id": projectID}))
				return nil
			}
			output := map[string]any{
				"task_id":        taskID,
				"project_id":     projectID,
				"canvas_url":     "https://www.lovart.ai/canvas?projectId=" + projectID,
				"canvas_updated": true,
				"image_count":    len(images),
			}
			if detail == "full" {
				output["task"] = generation.TaskView(task, detail)
			}
			printEnvelope(okSubmit(output, true))
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project-id", "", "target project ID (defaults to current project context)")
	cmd.Flags().StringVar(&detail, "detail", "summary", "output detail: summary, full")
	return cmd
}

func newTaskCancelCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "cancel <task_id...>",
		Short: "Cancel active generation tasks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				printEnvelope(envelope.Err(errors.CodeInputError, "refusing to cancel tasks without --yes", map[string]any{
					"task_ids": args,
					"recommended_actions": []string{
						"rerun with --yes to cancel these active tasks",
					},
				}))
				return nil
			}
			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}
			result, err := generation.CancelRunningTasks(context.Background(), client, args)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "cancel active tasks", map[string]any{"error": err.Error(), "task_ids": args}))
				return nil
			}
			printEnvelope(okSubmit(result, true))
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm active task cancellation")
	return cmd
}

func taskFailureEnvelope(taskID string, task map[string]any, detail string) envelope.Envelope {
	failure := generation.ClassifyTaskFailure(task)
	code := errors.CodeTaskFailed
	message := "generation task failed"
	if failure != nil {
		code = failure.Code
		message = failure.Message
	}
	details := map[string]any{
		"task_id": taskID,
		"task":    generation.TaskView(task, detail),
	}
	if failure != nil {
		details["failure"] = failure
	}
	return envelope.Err(code, message, details)
}
