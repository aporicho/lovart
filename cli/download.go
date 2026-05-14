package cli

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/downloads"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/generation"
	"github.com/aporicho/lovart/internal/project"
	"github.com/spf13/cobra"
)

func newDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download generated or canvas image artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newDownloadTaskCmd())
	cmd.AddCommand(newDownloadCanvasCmd())
	return cmd
}

func newDownloadTaskCmd() *cobra.Command {
	var (
		artifactIndex        int
		detail               string
		downloadDir          string
		downloadDirTemplate  string
		downloadFileTemplate string
		overwrite            bool
	)
	cmd := &cobra.Command{
		Use:   "task <task_id>",
		Short: "Download artifacts from a completed generation task",
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
			if status, _ := task["status"].(string); status != "completed" {
				printEnvelope(envelope.Err(errors.CodeInputError, "task is not completed", map[string]any{
					"task_id": taskID,
					"status":  status,
					"recommended_actions": []string{
						"retry after the task completes",
					},
				}))
				return nil
			}
			artifacts := downloads.ArtifactsFromTask(task)
			artifacts, env := selectDownloadArtifactIndex(artifacts, artifactIndex)
			if env != nil {
				printEnvelope(*env)
				return nil
			}
			result, err := downloads.DownloadArtifacts(context.Background(), artifacts, downloads.Options{
				RootDir:      downloadDir,
				DirTemplate:  downloadDirTemplate,
				FileTemplate: downloadFileTemplate,
				TaskID:       taskID,
				Context: downloads.JobContext{
					Fields: map[string]any{"source": "task", "task_id": taskID},
				},
				Overwrite: overwrite,
			})
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "download task artifacts", map[string]any{"error": err.Error()}))
				return nil
			}
			if downloads.SuccessfulFileCount(result.Files) == 0 {
				printEnvelope(envelope.Err(errors.CodeInternal, "download task artifacts failed", map[string]any{"files": result.Files}))
				return nil
			}
			output := map[string]any{
				"source": map[string]any{
					"type":    "task",
					"task_id": taskID,
					"status":  task["status"],
				},
				"selected_artifacts": len(artifacts),
				"root_dir":           result.RootDir,
				"index_path":         result.IndexPath,
				"files":              result.Files,
			}
			if detail == "full" {
				output["task"] = task
			}
			envOut := okPreflight(output)
			if result.IndexError != "" {
				output["download_index_error"] = result.IndexError
				envOut.Warnings = append(envOut.Warnings, "artifacts were downloaded but the download index could not be fully written")
			}
			printEnvelope(envOut)
			return nil
		},
	}
	cmd.Flags().IntVar(&artifactIndex, "index", 0, "download only the 1-based artifact index")
	cmd.Flags().StringVar(&detail, "detail", "summary", "output detail: summary, full")
	cmd.Flags().StringVar(&downloadDir, "download-dir", "", "directory for downloaded artifacts")
	cmd.Flags().StringVar(&downloadDirTemplate, "download-dir-template", "", "download subdirectory template")
	cmd.Flags().StringVar(&downloadFileTemplate, "download-file-template", "", "download filename template")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "replace existing downloaded files")
	return cmd
}

func newDownloadCanvasCmd() *cobra.Command {
	var (
		projectID            string
		artifactID           string
		artifactIndex        int
		taskID               string
		all                  bool
		original             bool
		downloadDir          string
		downloadDirTemplate  string
		downloadFileTemplate string
		overwrite            bool
	)
	cmd := &cobra.Command{
		Use:   "canvas [project_id]",
		Short: "Download image artifacts from a project canvas",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				projectID = args[0]
			}
			projectID, cid, env := resolveCLIProjectContext(projectID)
			if env != nil {
				printEnvelope(*env)
				return nil
			}
			if err := validateCanvasDownloadSelector(artifactID, artifactIndex, taskID, all); err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, err.Error(), nil))
				return nil
			}
			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}
			list, err := project.ListCanvasArtifacts(context.Background(), client, projectID, cid, project.CanvasArtifactsOptions{})
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "list canvas artifacts", map[string]any{"error": err.Error()}))
				return nil
			}
			selected, env := selectCanvasArtifacts(list.Artifacts, artifactID, artifactIndex, taskID, all)
			if env != nil {
				printEnvelope(*env)
				return nil
			}
			downloadArtifacts := canvasDownloadArtifacts(selected, original)
			result, err := downloads.DownloadArtifacts(context.Background(), downloadArtifacts, downloads.Options{
				RootDir:      downloadDir,
				DirTemplate:  downloadDirTemplate,
				FileTemplate: downloadFileTemplate,
				Context: downloads.JobContext{
					Title: list.ProjectName,
					Fields: map[string]any{
						"source":     "canvas",
						"project_id": projectID,
					},
				},
				Overwrite: overwrite,
			})
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "download canvas artifacts", map[string]any{"error": err.Error()}))
				return nil
			}
			if downloads.SuccessfulFileCount(result.Files) == 0 {
				printEnvelope(envelope.Err(errors.CodeInternal, "download canvas artifacts failed", map[string]any{"files": result.Files}))
				return nil
			}
			output := map[string]any{
				"source": map[string]any{
					"type":         "canvas",
					"project_id":   projectID,
					"project_name": list.ProjectName,
					"canvas_url":   list.CanvasURL,
				},
				"selected_artifacts": len(selected),
				"artifacts":          selected,
				"root_dir":           result.RootDir,
				"index_path":         result.IndexPath,
				"files":              result.Files,
			}
			envOut := okPreflight(output)
			if result.IndexError != "" {
				output["download_index_error"] = result.IndexError
				envOut.Warnings = append(envOut.Warnings, "artifacts were downloaded but the download index could not be fully written")
			}
			printEnvelope(envOut)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project-id", "", "target project ID (defaults to current project context)")
	cmd.Flags().StringVar(&artifactID, "artifact-id", "", "download one canvas artifact by artifact_id")
	cmd.Flags().IntVar(&artifactIndex, "index", 0, "download one canvas artifact by 1-based list index")
	cmd.Flags().StringVar(&taskID, "task-id", "", "download canvas artifacts associated with a generation task ID")
	cmd.Flags().BoolVar(&all, "all", false, "download all canvas image artifacts")
	cmd.Flags().BoolVar(&original, "original", false, "prefer originalUrl when available")
	cmd.Flags().StringVar(&downloadDir, "download-dir", "", "directory for downloaded artifacts")
	cmd.Flags().StringVar(&downloadDirTemplate, "download-dir-template", "", "download subdirectory template")
	cmd.Flags().StringVar(&downloadFileTemplate, "download-file-template", "", "download filename template")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "replace existing downloaded files")
	return cmd
}

func selectDownloadArtifactIndex(artifacts []downloads.Artifact, index int) ([]downloads.Artifact, *envelope.Envelope) {
	if len(artifacts) == 0 {
		env := envelope.Err(errors.CodeInputError, "no downloadable artifacts found", nil)
		return nil, &env
	}
	if index == 0 {
		return artifacts, nil
	}
	if index < 1 || index > len(artifacts) {
		env := envelope.Err(errors.CodeInputError, "artifact index out of range", map[string]any{
			"index": index,
			"count": len(artifacts),
		})
		return nil, &env
	}
	return []downloads.Artifact{artifacts[index-1]}, nil
}

func validateCanvasDownloadSelector(artifactID string, artifactIndex int, taskID string, all bool) error {
	count := 0
	if artifactID != "" {
		count++
	}
	if artifactIndex != 0 {
		count++
	}
	if taskID != "" {
		count++
	}
	if all {
		count++
	}
	if count != 1 {
		return fmt.Errorf("choose exactly one canvas selector: --artifact-id, --index, --task-id, or --all")
	}
	if artifactIndex < 0 {
		return fmt.Errorf("--index must be greater than zero")
	}
	return nil
}

func selectCanvasArtifacts(artifacts []project.CanvasArtifact, artifactID string, artifactIndex int, taskID string, all bool) ([]project.CanvasArtifact, *envelope.Envelope) {
	if all {
		if len(artifacts) == 0 {
			env := envelope.Err(errors.CodeInputError, "no downloadable canvas artifacts found", nil)
			return nil, &env
		}
		return artifacts, nil
	}
	if artifactID != "" {
		for _, artifact := range artifacts {
			if artifact.ArtifactID == artifactID {
				return []project.CanvasArtifact{artifact}, nil
			}
		}
		env := envelope.Err(errors.CodeInputError, "canvas artifact not found", map[string]any{"artifact_id": artifactID, "count": len(artifacts)})
		return nil, &env
	}
	if artifactIndex != 0 {
		if artifactIndex < 1 || artifactIndex > len(artifacts) {
			env := envelope.Err(errors.CodeInputError, "canvas artifact index out of range", map[string]any{"index": artifactIndex, "count": len(artifacts)})
			return nil, &env
		}
		return []project.CanvasArtifact{artifacts[artifactIndex-1]}, nil
	}
	var selected []project.CanvasArtifact
	for _, artifact := range artifacts {
		if artifact.TaskID == taskID {
			selected = append(selected, artifact)
		}
	}
	if len(selected) == 0 {
		env := envelope.Err(errors.CodeInputError, "no canvas artifacts found for task", map[string]any{"task_id": taskID, "count": len(artifacts)})
		return nil, &env
	}
	return selected, nil
}

func canvasDownloadArtifacts(artifacts []project.CanvasArtifact, original bool) []downloads.Artifact {
	out := make([]downloads.Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		url := artifact.URL
		if original && artifact.OriginalURL != "" {
			url = artifact.OriginalURL
		}
		out = append(out, downloads.Artifact{
			URL:    url,
			Width:  artifact.DisplayWidth,
			Height: artifact.DisplayHeight,
			Index:  artifact.Index,
		})
	}
	return out
}
