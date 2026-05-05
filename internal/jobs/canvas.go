package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/aporicho/lovart/internal/downloads"
	"github.com/aporicho/lovart/internal/project"
)

func canvasCompleted(ctx context.Context, remote RemoteClient, state *RunState, opts JobsOptions) error {
	projectID := opts.ProjectID
	if projectID == "" {
		projectID = state.ProjectID
	}
	cid := opts.CID
	if cid == "" {
		cid = state.CID
	}
	if projectID == "" || cid == "" {
		return fmt.Errorf("jobs: canvas writeback requires project context")
	}
	for i := range state.Jobs {
		for j := range state.Jobs[i].RemoteRequests {
			request := &state.Jobs[i].RemoteRequests[j]
			if request.Status != StatusCompleted && request.Status != StatusDownloaded {
				continue
			}
			if request.Canvas != nil && request.Canvas.Updated {
				continue
			}
			images := requestCanvasImages(*request)
			if len(images) == 0 {
				continue
			}
			if err := remote.AddToCanvas(ctx, projectID, cid, images); err != nil {
				request.Canvas = &CanvasResult{Updated: false, ImageCount: len(images), Error: err.Error(), UpdatedAt: time.Now().UTC()}
				addRequestError(request, "canvas_failed", "canvas writeback failed", map[string]any{"error": err.Error()})
			} else {
				request.Canvas = &CanvasResult{Updated: true, ImageCount: len(images), UpdatedAt: time.Now().UTC()}
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

func requestCanvasImages(request RemoteRequest) []project.CanvasImage {
	artifacts := downloads.ArtifactsFromDetails(request.Artifacts)
	images := make([]project.CanvasImage, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.URL == "" {
			continue
		}
		width := artifact.Width
		if width == 0 {
			width = 1024
		}
		height := artifact.Height
		if height == 0 {
			height = 1024
		}
		images = append(images, project.CanvasImage{
			TaskID: request.TaskID,
			URL:    artifact.URL,
			Width:  width,
			Height: height,
		})
	}
	return images
}
