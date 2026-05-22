package envelope

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintJSONRedactsEnvelopeOutput(t *testing.T) {
	secrets := []string{
		"message-token-secret-123",
		"detail-cookie-secret-123",
		"data-session-secret-123",
		"warning-user-token-secret-123",
	}
	e := Err("internal", "failed with Authorization: Bearer message-token-secret-123", map[string]any{
		"headers": map[string]any{"Cookie": "session=detail-cookie-secret-123"},
	})
	e.Data = map[string]any{"url": "https://example.test/?session=data-session-secret-123"}
	e.Warnings = []string{"callback https://example.test/?usertoken=warning-user-token-secret-123"}

	output := captureStdout(t, func() { PrintJSON(e) })
	for _, secret := range secrets {
		if strings.Contains(output, secret) {
			t.Fatalf("secret %q leaked in envelope output %q", secret, output)
		}
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Fatalf("expected replacement marker in output: %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = writer
	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	os.Stdout = original
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(out)
}
