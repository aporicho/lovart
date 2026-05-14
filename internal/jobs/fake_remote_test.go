package jobs

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/generation"
	"github.com/aporicho/lovart/internal/pricing"
	"github.com/aporicho/lovart/internal/project"
)

type fakeRemote struct {
	price               float64
	submits             int
	quotes              int
	taskStatus          string
	artifactURL         string
	canvasCalls         int
	canvasImages        int
	canvasBatchCalls    int
	canvasBatchSections int
	canvasSectionImages []int
	canvasSectionTitles []string
	taskExtra           map[string]any
	submittedOutputs    map[string]int
	quotedBodies        []map[string]any
	submittedBodies     []map[string]any
	submitErrors        []error
	runningTasks        []map[string]any
	listRunningCalls    int
}

func (f *fakeRemote) Quote(ctx context.Context, model string, body map[string]any, mode string) (*pricing.QuoteResult, error) {
	f.quotes++
	f.quotedBodies = append(f.quotedBodies, copyBody(body))
	return &pricing.QuoteResult{Price: f.price, Balance: 100, PriceDetail: pricing.PriceDetail{UnitPrice: f.price}}, nil
}

func (f *fakeRemote) Submit(ctx context.Context, model string, body map[string]any, opts generation.Options) (*generation.SubmitResult, error) {
	f.submits++
	f.submittedBodies = append(f.submittedBodies, copyBody(body))
	if f.submits <= len(f.submitErrors) && f.submitErrors[f.submits-1] != nil {
		return nil, f.submitErrors[f.submits-1]
	}
	taskID := fmt.Sprintf("task-%d", f.submits)
	if f.submittedOutputs == nil {
		f.submittedOutputs = map[string]int{}
	}
	f.submittedOutputs[taskID] = fakeOutputCount(body)
	return &generation.SubmitResult{TaskID: taskID, Status: "submitted"}, nil
}

func (f *fakeRemote) FetchTask(ctx context.Context, taskID string) (map[string]any, error) {
	status := f.taskStatus
	if status == "" {
		status = "running"
	}
	artifactURL := f.artifactURL
	if artifactURL == "" {
		artifactURL = "https://example.test/a.png"
	}
	count := f.submittedOutputs[taskID]
	if count <= 0 {
		count = 1
	}
	artifacts := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		artifacts = append(artifacts, map[string]any{
			"url": fmt.Sprintf("%s?i=%d", artifactURL, i+1),
		})
	}
	task := map[string]any{
		"task_id":          taskID,
		"status":           status,
		"artifact_details": artifacts,
	}
	for key, value := range f.taskExtra {
		task[key] = value
	}
	return task, nil
}

func (f *fakeRemote) ListRunningTasks(ctx context.Context) (*generation.TaskPoolResult, error) {
	f.listRunningCalls++
	if f.runningTasks != nil {
		return &generation.TaskPoolResult{ActiveCount: len(f.runningTasks), Tasks: f.runningTasks}, nil
	}
	tasks := make([]map[string]any, 0, len(f.submittedOutputs))
	for taskID := range f.submittedOutputs {
		tasks = append(tasks, map[string]any{"task_id": taskID})
	}
	return &generation.TaskPoolResult{ActiveCount: len(tasks), Tasks: tasks}, nil
}

func (f *fakeRemote) AddToCanvas(ctx context.Context, projectID, cid string, images []project.CanvasImage) error {
	f.canvasCalls++
	f.canvasImages += len(images)
	return nil
}

func (f *fakeRemote) AddBatchToCanvas(ctx context.Context, projectID, cid string, batch project.CanvasBatch) error {
	f.canvasBatchCalls++
	f.canvasBatchSections += len(batch.Sections)
	for _, section := range batch.Sections {
		f.canvasSectionImages = append(f.canvasSectionImages, len(section.Images))
		f.canvasSectionTitles = append(f.canvasSectionTitles, section.Title)
	}
	return nil
}

func fakeOutputCount(body map[string]any) int {
	for _, key := range []string{"n", "max_images", "num_images", "count"} {
		switch v := body[key].(type) {
		case int:
			if v > 0 {
				return v
			}
		case float64:
			if v > 0 {
				return int(v)
			}
		}
	}
	return 1
}
