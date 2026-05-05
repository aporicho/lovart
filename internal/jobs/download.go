package jobs

import (
	"context"
	"path/filepath"
	"time"

	"github.com/aporicho/lovart/internal/downloads"
)

func downloadCompleted(ctx context.Context, state *RunState, opts JobsOptions) error {
	root := opts.DownloadDir
	if root == "" {
		root = filepath.Join(state.RunDir, "downloads")
	}
	for i := range state.Jobs {
		nextArtifactIndex := 1
		for j := range state.Jobs[i].RemoteRequests {
			request := &state.Jobs[i].RemoteRequests[j]
			artifacts := requestDownloadArtifacts(*request, nextArtifactIndex)
			if len(artifacts) > 0 {
				nextArtifactIndex += len(artifacts)
			} else if request.OutputCount > 0 {
				nextArtifactIndex += request.OutputCount
			}

			if request.Status != StatusCompleted && request.Status != StatusDownloaded {
				continue
			}
			if len(artifacts) == 0 {
				continue
			}
			if requestDownloadsComplete(*request, len(artifacts)) {
				request.Status = StatusDownloaded
				continue
			}

			result, err := downloads.DownloadArtifacts(ctx, artifacts, downloads.Options{
				RootDir:      root,
				DirTemplate:  opts.DownloadDirTemplate,
				FileTemplate: opts.DownloadFileTemplate,
				TaskID:       request.TaskID,
				Context: downloads.JobContext{
					Model:  request.Model,
					Mode:   request.Mode,
					JobID:  request.JobID,
					Title:  request.Title,
					Fields: request.Fields,
					Body:   request.Body,
				},
			})
			if err != nil {
				addRequestError(request, "download_failed", "artifact download failed", map[string]any{"error": err.Error()})
				request.UpdatedAt = time.Now().UTC()
				continue
			}
			request.Downloads = result.Files
			if result.IndexError != "" {
				addRequestError(request, "download_index_failed", "download index update failed", map[string]any{"error": result.IndexError})
			}
			if downloadResultsComplete(result.Files, len(artifacts)) {
				request.Status = StatusDownloaded
			} else {
				request.Status = StatusCompleted
				addRequestError(request, "download_incomplete", "one or more artifacts failed to download", map[string]any{"downloads": result.Files})
			}
			request.UpdatedAt = time.Now().UTC()
			RefreshStatuses(state)
			if err := SaveState(state); err != nil {
				return err
			}
		}
	}
	RefreshStatuses(state)
	return SaveState(state)
}

func requestDownloadArtifacts(request RemoteRequest, startIndex int) []downloads.Artifact {
	artifacts := downloads.ArtifactsFromDetails(request.Artifacts)
	for i := range artifacts {
		artifacts[i].Index = startIndex + i
	}
	return artifacts
}

func requestDownloadsComplete(request RemoteRequest, artifactCount int) bool {
	return downloadResultsComplete(request.Downloads, artifactCount)
}

func downloadResultsComplete(results []downloads.FileResult, artifactCount int) bool {
	if artifactCount == 0 || len(results) < artifactCount {
		return false
	}
	ok := 0
	for _, result := range results {
		if result.Error == "" && result.Path != "" {
			ok++
		}
	}
	return ok >= artifactCount
}
