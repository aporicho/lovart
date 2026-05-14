package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aporicho/lovart/internal/project"
)

func canvasCompleted(ctx context.Context, remote RemoteClient, state *RunState, opts JobsOptions) error {
	layout := opts.CanvasLayout
	if layout == "" {
		layout = CanvasLayoutFrame
	}
	switch layout {
	case CanvasLayoutFrame:
		return canvasCompletedFrame(ctx, remote, state, opts)
	case CanvasLayoutPlain:
		return canvasCompletedPlain(ctx, remote, state, opts)
	default:
		return fmt.Errorf("jobs: canvas layout must be %s or %s", CanvasLayoutFrame, CanvasLayoutPlain)
	}
}

func canvasCompletedPlain(ctx context.Context, remote RemoteClient, state *RunState, opts JobsOptions) error {
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
				request.Canvas = &CanvasResult{Updated: false, ImageCount: len(images), Layout: CanvasLayoutPlain, Error: err.Error(), UpdatedAt: time.Now().UTC()}
				addRequestError(request, "canvas_failed", "canvas writeback failed", map[string]any{"error": err.Error()})
			} else {
				request.Canvas = &CanvasResult{Updated: true, ImageCount: len(images), Layout: CanvasLayoutPlain, UpdatedAt: time.Now().UTC()}
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

type canvasSectionRequests struct {
	section  project.CanvasSection
	requests []*RemoteRequest
	counts   []int
}

func canvasCompletedFrame(ctx context.Context, remote RemoteClient, state *RunState, opts JobsOptions) error {
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

	var pending []canvasSectionRequests
	for i := range state.Jobs {
		section := canvasSectionForJob(&state.Jobs[i])
		if len(section.section.Images) > 0 {
			pending = append(pending, section)
		}
	}
	if len(pending) == 0 {
		RefreshStatuses(state)
		return SaveState(state)
	}

	batch := project.CanvasBatch{Sections: make([]project.CanvasSection, 0, len(pending))}
	for _, section := range pending {
		batch.Sections = append(batch.Sections, section.section)
	}

	now := time.Now().UTC()
	if err := remote.AddBatchToCanvas(ctx, projectID, cid, batch); err != nil {
		for _, section := range pending {
			for i, request := range section.requests {
				count := section.counts[i]
				request.Canvas = &CanvasResult{Updated: false, ImageCount: count, Layout: CanvasLayoutFrame, Error: err.Error(), UpdatedAt: now}
				addRequestError(request, "canvas_failed", "canvas writeback failed", map[string]any{"error": err.Error()})
				request.UpdatedAt = now
			}
		}
	} else {
		for _, section := range pending {
			for i, request := range section.requests {
				count := section.counts[i]
				request.Canvas = &CanvasResult{Updated: true, ImageCount: count, Layout: CanvasLayoutFrame, UpdatedAt: now}
				request.UpdatedAt = now
			}
		}
	}

	RefreshStatuses(state)
	return SaveState(state)
}

func canvasSectionForJob(job *JobState) canvasSectionRequests {
	out := canvasSectionRequests{
		section: project.CanvasSection{
			ID:       job.JobID,
			Title:    canvasJobTitle(*job),
			Subtitle: canvasJobSubtitle(*job),
		},
	}
	for i := range job.RemoteRequests {
		request := &job.RemoteRequests[i]
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
		out.section.Images = append(out.section.Images, images...)
		out.requests = append(out.requests, request)
		out.counts = append(out.counts, len(images))
	}
	return out
}

func canvasJobTitle(job JobState) string {
	if strings.TrimSpace(job.Title) != "" {
		return job.Title
	}
	if strings.TrimSpace(job.JobID) != "" {
		return job.JobID
	}
	return "Generated images"
}

func canvasJobSubtitle(job JobState) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(job.Model) != "" {
		parts = append(parts, job.Model)
	}
	if job.Outputs > 0 {
		unit := "images"
		if job.Outputs == 1 {
			unit = "image"
		}
		parts = append(parts, fmt.Sprintf("%d %s", job.Outputs, unit))
	}
	return strings.Join(parts, " · ")
}

func requestCanvasImages(request RemoteRequest) []project.CanvasImage {
	return project.CanvasImagesFromArtifacts(request.TaskID, requestDownloadArtifacts(request, 1))
}
