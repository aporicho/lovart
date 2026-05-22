package jobs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadStateMigratesV1State(t *testing.T) {
	runDir := t.TempDir()
	stateFile := filepath.Join(runDir, stateFileName)
	writeStateDocument(t, runDir, `{
		"version": 1,
		"run_dir": "`+runDir+`",
		"state_file": "`+stateFile+`",
		"jobs": [{
			"job_id": "a",
			"model": "openai/gpt-image-2",
			"mode": "auto",
			"outputs": 1,
			"status": "pending",
			"remote_requests": []
		}]
	}`)

	state, err := LoadState(runDir)
	if err != nil {
		t.Fatal(err)
	}
	if state.Version != currentRunStateVersion {
		t.Fatalf("version=%d, want %d", state.Version, currentRunStateVersion)
	}
	if len(state.Jobs) != 1 || state.Jobs[0].JobID != "a" {
		t.Fatalf("jobs=%#v", state.Jobs)
	}
}

func TestLoadStateMigratesMissingVersionToV1(t *testing.T) {
	runDir := t.TempDir()
	writeStateDocument(t, runDir, `{
		"jobs_file": "jobs.jsonl",
		"jobs": [{
			"job_id": "legacy",
			"model": "openai/gpt-image-2",
			"mode": "auto",
			"outputs": 1,
			"status": "pending",
			"remote_requests": []
		}]
	}`)

	state, err := LoadState(runDir)
	if err != nil {
		t.Fatal(err)
	}
	if state.Version != currentRunStateVersion {
		t.Fatalf("version=%d, want %d", state.Version, currentRunStateVersion)
	}
	if state.Jobs[0].JobID != "legacy" {
		t.Fatalf("jobs=%#v", state.Jobs)
	}
}

func TestLoadStateRejectsFutureVersion(t *testing.T) {
	runDir := t.TempDir()
	writeStateDocument(t, runDir, `{"version": 2, "jobs": []}`)

	_, err := LoadState(runDir)
	if err == nil {
		t.Fatal("expected future version error")
	}
	var schemaErr *RunStateSchemaError
	if !errors.As(err, &schemaErr) {
		t.Fatalf("error=%T %[1]v, want RunStateSchemaError", err)
	}
	if schemaErr.Version != 2 || !strings.Contains(err.Error(), "unsupported future version") {
		t.Fatalf("error=%v", err)
	}
}

func TestLoadStateFillsRunDirAndStateFileDefaults(t *testing.T) {
	runDir := t.TempDir()
	writeStateDocument(t, runDir, `{"version": 1, "jobs": []}`)

	state, err := LoadState(runDir)
	if err != nil {
		t.Fatal(err)
	}
	if state.RunDir != runDir {
		t.Fatalf("run_dir=%q, want %q", state.RunDir, runDir)
	}
	wantStateFile := filepath.Join(runDir, stateFileName)
	if state.StateFile != wantStateFile {
		t.Fatalf("state_file=%q, want %q", state.StateFile, wantStateFile)
	}
}

func writeStateDocument(t *testing.T, runDir string, document string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(runDir, stateFileName), []byte(document), 0644); err != nil {
		t.Fatal(err)
	}
}
