package jobs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalizeJobsMissingCanvasConsentBlocksBeforeRemoteWrite(t *testing.T) {
	state := completedFinalizeState(t)
	remote := &fakeRemote{}
	_, err := FinalizeJobs(context.Background(), remote, state.RunDir, JobsOptions{Canvas: true, CanvasLayout: CanvasLayoutFrame})
	if err == nil || !strings.Contains(err.Error(), "structured_consent_missing_confirm_canvas_write") {
		t.Fatalf("FinalizeJobs err=%v, want missing canvas consent", err)
	}
	if remote.canvasCalls != 0 || remote.canvasBatchCalls != 0 {
		t.Fatalf("canvas write happened: plain=%d batch=%d", remote.canvasCalls, remote.canvasBatchCalls)
	}
}

func TestFinalizeJobsCanvasConsentLayoutAndProjectMismatchBlock(t *testing.T) {
	state := completedFinalizeState(t)
	remote := &fakeRemote{}
	_, err := FinalizeJobs(context.Background(), remote, state.RunDir, JobsOptions{
		Canvas:                true,
		CanvasLayout:          CanvasLayoutPlain,
		ConfirmCanvasWrite:    true,
		ConfirmedCanvasLayout: CanvasLayoutFrame,
		ConfirmedProjectID:    state.ProjectID,
	})
	if err == nil || !strings.Contains(err.Error(), "structured_consent_canvas_layout_mismatch") {
		t.Fatalf("FinalizeJobs layout err=%v", err)
	}
	if remote.canvasCalls != 0 || remote.canvasBatchCalls != 0 {
		t.Fatalf("canvas write happened after layout mismatch: plain=%d batch=%d", remote.canvasCalls, remote.canvasBatchCalls)
	}

	_, err = FinalizeJobs(context.Background(), remote, state.RunDir, JobsOptions{
		Canvas:                true,
		CanvasLayout:          CanvasLayoutFrame,
		ConfirmCanvasWrite:    true,
		ConfirmedCanvasLayout: CanvasLayoutFrame,
		ConfirmedProjectID:    "other-project",
	})
	if err == nil || !strings.Contains(err.Error(), "structured_consent_project_id_mismatch") {
		t.Fatalf("FinalizeJobs project err=%v", err)
	}
	if remote.canvasCalls != 0 || remote.canvasBatchCalls != 0 {
		t.Fatalf("canvas write happened after project mismatch: plain=%d batch=%d", remote.canvasCalls, remote.canvasBatchCalls)
	}
}

func TestFinalizeJobsValidCanvasConsentAllowsFakeRemotePath(t *testing.T) {
	state := completedFinalizeState(t)
	remote := &fakeRemote{}
	result, err := FinalizeJobs(context.Background(), remote, state.RunDir, JobsOptions{
		Canvas:                true,
		CanvasLayout:          CanvasLayoutFrame,
		ConfirmCanvasWrite:    true,
		ConfirmedCanvasLayout: CanvasLayoutFrame,
		ConfirmedProjectID:    state.ProjectID,
	})
	if err != nil {
		t.Fatalf("FinalizeJobs: %v", err)
	}
	if remote.canvasBatchCalls != 1 || result.Summary.CanvasUpdatedRequests != 1 {
		t.Fatalf("canvas not written through fake remote: calls=%d summary=%#v", remote.canvasBatchCalls, result.Summary)
	}
}

func TestFinalizeJobsLocalWriteCanonicalMismatchBlocks(t *testing.T) {
	state := completedFinalizeState(t)
	downloadDir := filepath.Join(t.TempDir(), "downloads")
	_, err := FinalizeJobs(context.Background(), nil, state.RunDir, JobsOptions{
		Download:                  true,
		DownloadDir:               downloadDir,
		ConfirmedCanonicalPath:    filepath.Join(t.TempDir(), "different"),
		ConfirmOutsideDefaultRoot: true,
	})
	if err == nil || !strings.Contains(err.Error(), "structured_consent_canonical_path_mismatch") {
		t.Fatalf("FinalizeJobs err=%v, want canonical mismatch", err)
	}
	if _, statErr := os.Stat(downloadDir); !os.IsNotExist(statErr) {
		t.Fatalf("download dir was created despite consent failure: stat=%v", statErr)
	}
}

func completedFinalizeState(t *testing.T) *RunState {
	t.Helper()
	setupRuntimeSchema(t)
	dir := t.TempDir()
	jobsPath := filepath.Join(dir, "jobs.jsonl")
	if err := os.WriteFile(jobsPath, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	state, validation, err := PrepareRun(jobsPath, JobsOptions{ProjectID: "project", CID: "cid"})
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	request := &state.Jobs[0].RemoteRequests[0]
	request.Status = StatusCompleted
	request.TaskID = "task-1"
	request.Artifacts = []map[string]any{{"url": "https://example.test/a.png"}}
	RefreshStatuses(state)
	if err := SaveState(state); err != nil {
		t.Fatal(err)
	}
	return state
}
