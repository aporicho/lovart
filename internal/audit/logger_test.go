package audit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriterEmitsValidJSONL(t *testing.T) {
	var output bytes.Buffer
	logger := NewWriter(&output)

	eventTime := time.Date(2026, 5, 22, 10, 30, 0, 0, time.UTC)
	err := logger.Write(Event{
		Timestamp: eventTime,
		Operation: "tool.invoke",
		Actor:     "cli-user",
		Source:    "cli",
		Tool:      "browser",
		Action:    "navigate",
		Decision:  "allow",
		RiskFlags: []string{"network"},
		Metadata:  map[string]any{"url": "https://example.test"},
	})
	if err != nil {
		t.Fatalf("write event: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one JSONL line, got %d", len(lines))
	}

	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("line is not valid JSON: %v", err)
	}
	if record["operation"] != "tool.invoke" || record["decision"] != "allow" {
		t.Fatalf("unexpected record: %#v", record)
	}
	if record["timestamp"] != eventTime.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected timestamp %v", record["timestamp"])
	}
}

func TestWriterRedactsSensitiveMetadataAndError(t *testing.T) {
	var output bytes.Buffer
	logger := NewWriter(&output)

	err := logger.Write(Event{
		Timestamp: time.Date(2026, 5, 22, 10, 30, 0, 0, time.UTC),
		Operation: "tool.invoke",
		Metadata: map[string]any{
			"token": "secret-token",
			"nested": map[string]any{
				"authorization": "Bearer abc123",
				"message":       "callback?userToken=abc123",
			},
		},
		Error: "request failed authorization: bearer abc123",
	})
	if err != nil {
		t.Fatalf("write event: %v", err)
	}

	var record struct {
		Metadata map[string]any `json:"metadata"`
		Error    string         `json:"error"`
	}
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if record.Metadata["token"] != "[REDACTED]" {
		t.Fatalf("token was not redacted: %#v", record.Metadata["token"])
	}
	nested, ok := record.Metadata["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested metadata missing: %#v", record.Metadata["nested"])
	}
	if nested["authorization"] != "[REDACTED]" {
		t.Fatalf("authorization was not redacted: %#v", nested["authorization"])
	}
	if strings.Contains(output.String(), "abc123") || strings.Contains(output.String(), "secret-token") {
		t.Fatalf("output contains sensitive material: %s", output.String())
	}
}

func TestDisabledLoggersDoNotWriteOrCreateFiles(t *testing.T) {
	var output bytes.Buffer
	if err := Disabled().Write(Event{Operation: "ignored"}); err != nil {
		t.Fatalf("disabled write: %v", err)
	}
	if err := NewWriter(nil).Write(Event{Operation: "ignored"}); err != nil {
		t.Fatalf("nil writer write: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected output: %q", output.String())
	}

	tempDir := t.TempDir()
	logger, err := NewPath("")
	if err != nil {
		t.Fatalf("empty path logger: %v", err)
	}
	if logger.Enabled() {
		t.Fatal("empty path logger should be disabled")
	}
	if err := logger.Write(Event{Operation: "ignored"}); err != nil {
		t.Fatalf("empty path write: %v", err)
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("disabled path logger created files: %#v", entries)
	}
}

func TestPathLoggerAppendsJSONLLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit", "events.jsonl")

	logger, err := NewPath(path)
	if err != nil {
		t.Fatalf("open first logger: %v", err)
	}
	if err := logger.Write(Event{Timestamp: time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC), Operation: "first"}); err != nil {
		t.Fatalf("write first event: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("close first logger: %v", err)
	}

	logger, err = NewPath(path)
	if err != nil {
		t.Fatalf("open second logger: %v", err)
	}
	if err := logger.Write(Event{Timestamp: time.Date(2026, 5, 22, 10, 1, 0, 0, time.UTC), Operation: "second"}); err != nil {
		t.Fatalf("write second event: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("close second logger: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two appended lines, got %d: %s", len(lines), content)
	}
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("invalid appended JSON line %q: %v", line, err)
		}
	}
}
