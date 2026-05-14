package jobs

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFinalizeJobsDownloadsAndWritesCanvasWithoutResubmit(t *testing.T) {
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
	if err := os.WriteFile(path, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	runOpts := JobsOptions{
		AllowPaid:       true,
		MaxTotalCredits: 10,
		ProjectID:       "project",
		CID:             "cid",
		Wait:            true,
		PollInterval:    0.01,
		TimeoutSeconds:  1,
	}
	state, validation, err := PrepareRun(path, runOpts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0, taskStatus: "completed", artifactURL: server.URL + "/a.png"}
	result, err := RunPreparedJobs(context.Background(), remote, state, runOpts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.RemoteStatusCounts[StatusCompleted] != 1 || remote.submits != 1 {
		t.Fatalf("run summary=%#v submits=%d", result.Summary, remote.submits)
	}

	finalized, err := FinalizeJobs(context.Background(), remote, state.RunDir, JobsOptions{
		Download:     true,
		Canvas:       true,
		DownloadDir:  filepath.Join(dir, "downloads"),
		CanvasLayout: CanvasLayoutFrame,
	})
	if err != nil {
		t.Fatal(err)
	}
	if remote.submits != 1 {
		t.Fatalf("finalize resubmitted task, submits=%d", remote.submits)
	}
	if remote.canvasBatchCalls != 1 || finalized.Summary.CanvasUpdatedRequests != 1 || finalized.Summary.CanvasImages != 1 {
		t.Fatalf("canvas calls=%d summary=%#v", remote.canvasBatchCalls, finalized.Summary)
	}
	if finalized.Summary.RemoteStatusCounts[StatusDownloaded] != 1 || len(finalized.Downloads) != 1 {
		t.Fatalf("downloads summary=%#v downloads=%#v", finalized.Summary, finalized.Downloads)
	}

	finalizedAgain, err := FinalizeJobs(context.Background(), remote, state.RunDir, JobsOptions{
		Download:     true,
		Canvas:       true,
		DownloadDir:  filepath.Join(dir, "downloads"),
		CanvasLayout: CanvasLayoutFrame,
	})
	if err != nil {
		t.Fatal(err)
	}
	if remote.canvasBatchCalls != 1 || finalizedAgain.Summary.CanvasUpdatedRequests != 1 {
		t.Fatalf("finalize repeated canvas writeback, calls=%d summary=%#v", remote.canvasBatchCalls, finalizedAgain.Summary)
	}
}
