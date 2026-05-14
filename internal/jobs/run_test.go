package jobs

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aporicho/lovart/internal/generation"
	"github.com/aporicho/lovart/internal/pricing"
	"github.com/aporicho/lovart/internal/project"
)

func TestParseJobsFileDefaultsAndConflicts(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := `{"model":"openai/gpt-image-2","body":{"prompt":"test"}}
{"job_id":"explicit","model":"openai/gpt-image-2","mode":"relax","outputs":2,"body":{"prompt":"test"}}
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	lines, err := ParseJobsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if lines[0].JobID != "line-000001" || lines[0].Mode != "auto" || lines[0].Outputs != 1 {
		t.Fatalf("defaults not applied: %#v", lines[0])
	}
	if !lines[1].OutputsExplicit {
		t.Fatalf("expected explicit outputs: %#v", lines[1])
	}

	conflict := filepath.Join(dir, "conflict.jsonl")
	if err := os.WriteFile(conflict, []byte(`{"model":"openai/gpt-image-2","outputs":1,"body":{"prompt":"test","n":1}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseJobsFile(conflict); err == nil {
		t.Fatal("expected quantity conflict")
	}
}

func TestRunPreparedJobsSavesStateAndResumeDoesNotResubmit(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"a","model":"openai/gpt-image-2","outputs":2,"body":{"prompt":"test"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 10, ProjectID: "project", CID: "cid", PollInterval: 0.01, TimeoutSeconds: 1}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.RemoteRequests != 1 || remote.submits != 1 {
		t.Fatalf("run result=%#v submits=%d", result.Summary, remote.submits)
	}
	if _, err := os.Stat(state.StateFile); err != nil {
		t.Fatalf("state not saved: %v", err)
	}

	resumed, err := ResumeJobs(context.Background(), remote, state.RunDir, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.submits != 1 {
		t.Fatalf("resume resubmitted task, submits=%d", remote.submits)
	}
	if resumed.Summary.RemoteStatusCounts[StatusSubmitted] != 1 {
		t.Fatalf("resume summary=%#v", resumed.Summary)
	}
}

func TestRunPreparedJobsQuotesAndSubmitsNormalizedBody(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	if err := os.WriteFile(path, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 10, ProjectID: "project", CID: "cid"}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0}
	if _, err := RunPreparedJobs(context.Background(), remote, state, opts); err != nil {
		t.Fatal(err)
	}
	if len(remote.quotedBodies) != 1 || remote.quotedBodies[0]["resolution"] != "1K" {
		t.Fatalf("quoted bodies=%#v, want normalized resolution", remote.quotedBodies)
	}
	if len(remote.submittedBodies) != 1 || remote.submittedBodies[0]["resolution"] != "1K" {
		t.Fatalf("submitted bodies=%#v, want normalized resolution", remote.submittedBodies)
	}
	request := state.Jobs[0].RemoteRequests[0]
	if request.NormalizedBody["resolution"] != "1K" {
		t.Fatalf("state normalized body=%#v", request.NormalizedBody)
	}
}

func TestRunPreparedJobsBlocksPaidWithoutAllowance(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	if err := os.WriteFile(path, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	state, validation, err := PrepareRun(path, JobsOptions{})
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 5}
	_, err = RunPreparedJobs(context.Background(), remote, state, JobsOptions{})
	var gateErr *GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("expected gate error, got %v", err)
	}
	if gateErr.Code != "credit_risk" || remote.submits != 0 {
		t.Fatalf("gate=%#v submits=%d", gateErr, remote.submits)
	}
	if gateErr.RunDir != state.RunDir || gateErr.StateFile != state.StateFile {
		t.Fatalf("gate state paths = %#v, want run_dir=%q state_file=%q", gateErr, state.RunDir, state.StateFile)
	}
}

func TestStatusJobsRefreshesActiveTasks(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	if err := os.WriteFile(path, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{AllowPaid: true, MaxTotalCredits: 10, ProjectID: "project", CID: "cid"}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0}
	if _, err := RunPreparedJobs(context.Background(), remote, state, opts); err != nil {
		t.Fatal(err)
	}
	remote.taskStatus = "completed"
	result, err := StatusJobs(context.Background(), remote, state.RunDir, JobsOptions{Refresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.RemoteStatusCounts[StatusCompleted] != 1 {
		t.Fatalf("status summary=%#v", result.Summary)
	}
}

func TestRunPreparedJobsDownloadsArtifactsByFields(t *testing.T) {
	setupRuntimeSchema(t)
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"scene-001","title":"001 Test Scene / film","fields":{"series":"series-a","scene_no":"001","scene_name":"Test Scene"},"model":"openai/gpt-image-2","outputs":1,"body":{"prompt":"download prompt"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	downloadDir := filepath.Join(dir, "downloads")
	opts := JobsOptions{
		AllowPaid:            true,
		MaxTotalCredits:      10,
		ProjectID:            "project",
		CID:                  "cid",
		Wait:                 true,
		Download:             true,
		DownloadDir:          downloadDir,
		DownloadDirTemplate:  "{{fields.series}}/{{fields.scene_no}} {{fields.scene_name}}",
		DownloadFileTemplate: "artifact-{{artifact.index:02}}.{{ext}}",
		PollInterval:         0.01,
		TimeoutSeconds:       1,
	}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0, taskStatus: "completed", artifactURL: server.URL + "/a.png"}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.RemoteStatusCounts[StatusDownloaded] != 1 {
		t.Fatalf("summary=%#v", result.Summary)
	}
	wantPath := filepath.Join(downloadDir, "series-a", "001 Test Scene", "artifact-01.png")
	if len(result.Downloads) != 1 || result.Downloads[0].Path != wantPath {
		t.Fatalf("downloads=%#v want path %s", result.Downloads, wantPath)
	}
	file, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(file, []byte("download prompt")) || bytes.Contains(file, []byte("project")) {
		t.Fatalf("effect metadata not embedded correctly")
	}

	resumed, err := ResumeJobs(context.Background(), remote, state.RunDir, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.submits != 1 {
		t.Fatalf("resume resubmitted task, submits=%d", remote.submits)
	}
	if resumed.Summary.RemoteStatusCounts[StatusDownloaded] != 1 {
		t.Fatalf("resume summary=%#v", resumed.Summary)
	}
}

func TestRunPreparedJobsWritesDefaultFrameCanvasAndResumeDoesNotRepeat(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	if err := os.WriteFile(path, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{
		AllowPaid:       true,
		MaxTotalCredits: 10,
		ProjectID:       "project",
		CID:             "cid",
		Wait:            true,
		Canvas:          true,
		PollInterval:    0.01,
		TimeoutSeconds:  1,
	}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0, taskStatus: "completed"}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.canvasBatchCalls != 1 || remote.canvasBatchSections != 1 {
		t.Fatalf("batch canvas calls=%d sections=%d", remote.canvasBatchCalls, remote.canvasBatchSections)
	}
	if result.Summary.CanvasUpdatedRequests != 1 || result.Summary.CanvasImages != 1 {
		t.Fatalf("summary=%#v", result.Summary)
	}

	resumed, err := ResumeJobs(context.Background(), remote, state.RunDir, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.canvasBatchCalls != 1 {
		t.Fatalf("resume repeated canvas writeback, calls=%d", remote.canvasBatchCalls)
	}
	if resumed.Summary.CanvasUpdatedRequests != 1 {
		t.Fatalf("resume summary=%#v", resumed.Summary)
	}
}

func TestRunPreparedJobsWritesFrameCanvasBatchAndResumeDoesNotRepeat(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	data := `{"job_id":"cat","title":"Cat","model":"openai/gpt-image-2","body":{"prompt":"cat","n":2}}
{"job_id":"dog","title":"Dog","model":"openai/gpt-image-2","body":{"prompt":"dog","n":3}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{
		AllowPaid:       true,
		MaxTotalCredits: 10,
		ProjectID:       "project",
		CID:             "cid",
		Wait:            true,
		Canvas:          true,
		PollInterval:    0.01,
		TimeoutSeconds:  1,
	}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0, taskStatus: "completed"}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.canvasBatchCalls != 1 {
		t.Fatalf("batch canvas calls=%d, want 1", remote.canvasBatchCalls)
	}
	if remote.canvasBatchSections != 2 {
		t.Fatalf("batch sections=%d, want 2", remote.canvasBatchSections)
	}
	if !reflect.DeepEqual(remote.canvasSectionImages, []int{2, 3}) {
		t.Fatalf("section images=%#v, want [2 3]", remote.canvasSectionImages)
	}
	if !reflect.DeepEqual(remote.canvasSectionTitles, []string{"Cat", "Dog"}) {
		t.Fatalf("section titles=%#v, want Cat/Dog", remote.canvasSectionTitles)
	}
	if result.Summary.CanvasUpdatedRequests != 2 || result.Summary.CanvasImages != 5 {
		t.Fatalf("summary=%#v", result.Summary)
	}

	resumed, err := ResumeJobs(context.Background(), remote, state.RunDir, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.canvasBatchCalls != 1 {
		t.Fatalf("resume repeated canvas writeback, calls=%d", remote.canvasBatchCalls)
	}
	if resumed.Summary.CanvasUpdatedRequests != 2 || resumed.Summary.CanvasImages != 5 {
		t.Fatalf("resume summary=%#v", resumed.Summary)
	}
}

func TestRunPreparedJobsPlainCanvasLayoutUsesLegacyCalls(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	if err := os.WriteFile(path, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	opts := JobsOptions{
		AllowPaid:       true,
		MaxTotalCredits: 10,
		ProjectID:       "project",
		CID:             "cid",
		Wait:            true,
		Canvas:          true,
		CanvasLayout:    CanvasLayoutPlain,
		PollInterval:    0.01,
		TimeoutSeconds:  1,
	}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0, taskStatus: "completed"}
	result, err := RunPreparedJobs(context.Background(), remote, state, opts)
	if err != nil {
		t.Fatal(err)
	}
	if remote.canvasCalls != 1 || remote.canvasBatchCalls != 0 {
		t.Fatalf("canvas calls=%d batch calls=%d", remote.canvasCalls, remote.canvasBatchCalls)
	}
	if result.Summary.CanvasUpdatedRequests != 1 || result.Summary.CanvasImages != 1 {
		t.Fatalf("summary=%#v", result.Summary)
	}
}

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
	submittedOutputs    map[string]int
	quotedBodies        []map[string]any
	submittedBodies     []map[string]any
}

func (f *fakeRemote) Quote(ctx context.Context, model string, body map[string]any, mode string) (*pricing.QuoteResult, error) {
	f.quotes++
	f.quotedBodies = append(f.quotedBodies, copyBody(body))
	return &pricing.QuoteResult{Price: f.price, Balance: 100, PriceDetail: pricing.PriceDetail{UnitPrice: f.price}}, nil
}

func (f *fakeRemote) Submit(ctx context.Context, model string, body map[string]any, opts generation.Options) (*generation.SubmitResult, error) {
	f.submits++
	f.submittedBodies = append(f.submittedBodies, copyBody(body))
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
	return map[string]any{
		"task_id":          taskID,
		"status":           status,
		"artifact_details": artifacts,
	}, nil
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
