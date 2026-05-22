package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aporicho/lovart/internal/audit"
)

func TestRunPreparedJobsWritesAuditEvents(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	if err := os.WriteFile(path, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	opts := JobsOptions{
		AllowPaid:       true,
		MaxTotalCredits: 10,
		ProjectID:       "project",
		CID:             "cid",
		AuditLogger:     audit.NewWriter(&output),
	}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 0}
	if _, err := RunPreparedJobs(context.Background(), remote, state, opts); err != nil {
		t.Fatal(err)
	}

	events := decodeJobsAuditEvents(t, output.String())
	if len(events) < 3 {
		t.Fatalf("audit event count = %d, want at least 3: %#v", len(events), events)
	}
	if events[0]["operation"] != "jobs.run" || events[0]["action"] != "start" {
		t.Fatalf("unexpected start event: %#v", events[0])
	}
	foundSubmit := false
	for _, event := range events {
		if event["operation"] == "jobs.submit_request" && event["decision"] == "success" {
			foundSubmit = true
		}
	}
	if !foundSubmit {
		t.Fatalf("missing successful submit audit event: %#v", events)
	}
	last := events[len(events)-1]
	if last["operation"] != "jobs.run" || last["action"] != "finish" || last["decision"] != "success" {
		t.Fatalf("unexpected finish event: %#v", last)
	}
}

func TestRunPreparedJobsAuditsGateDenial(t *testing.T) {
	setupRuntimeSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.jsonl")
	if err := os.WriteFile(path, []byte(`{"job_id":"a","model":"openai/gpt-image-2","body":{"prompt":"test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	opts := JobsOptions{AuditLogger: audit.NewWriter(&output)}
	state, validation, err := PrepareRun(path, opts)
	if err != nil || validation != nil {
		t.Fatalf("PrepareRun validation=%v err=%v", validation, err)
	}
	remote := &fakeRemote{price: 5}
	_, err = RunPreparedJobs(context.Background(), remote, state, opts)
	if err == nil {
		t.Fatal("expected gate error")
	}

	events := decodeJobsAuditEvents(t, output.String())
	foundDeny := false
	for _, event := range events {
		if event["operation"] == "jobs.run" && event["action"] == "gate_denied" && event["decision"] == "deny" {
			foundDeny = true
		}
	}
	if !foundDeny {
		t.Fatalf("missing gate denial audit event: %#v", events)
	}
}

func decodeJobsAuditEvents(t *testing.T, text string) []map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace([]byte(text)), []byte("\n"))
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("decode audit event %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}
