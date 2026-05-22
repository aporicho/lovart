package generation

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestMissingConfirmedProjectIDBlocksBeforeSubmit(t *testing.T) {
	_, err := Submit(context.Background(), nil, "model", map[string]any{"prompt": "x"}, Options{ProjectID: "proj_123"})
	var consentErr *Phase2ConsentError
	if !errors.As(err, &consentErr) {
		t.Fatalf("expected Phase2ConsentError before submit, got %T %v", err, err)
	}
	if consentErr.Code != "confirmed_project_id_mismatch" {
		t.Fatalf("code = %q", consentErr.Code)
	}
}

func TestMissingCanvasConsentBlocksBeforeCanvasWrite(t *testing.T) {
	err := EnforceCanvasConsent(Options{
		Canvas:             true,
		ProjectID:          "proj_123",
		ConfirmedProjectID: "proj_123",
		CanvasLayout:       "frame",
	})
	var consentErr *Phase2ConsentError
	if !errors.As(err, &consentErr) {
		t.Fatalf("expected Phase2ConsentError, got %T %v", err, err)
	}
	if consentErr.Code != "confirm_canvas_write_required" {
		t.Fatalf("code = %q", consentErr.Code)
	}
}

func TestLocalCanonicalMismatchBlocksBeforeDownload(t *testing.T) {
	withLovartHome(t)
	_, err := EnforceDownloadConsent(Options{
		Download:               true,
		DownloadDir:            "task-output",
		ConfirmedCanonicalPath: "/tmp/not-the-download-root",
	})
	var consentErr *Phase2ConsentError
	if !errors.As(err, &consentErr) {
		t.Fatalf("expected Phase2ConsentError, got %T %v", err, err)
	}
	if consentErr.Code != "confirmed_canonical_path_mismatch" {
		t.Fatalf("code = %q", consentErr.Code)
	}
}

func TestValidDownloadConsentAllowsFakePath(t *testing.T) {
	withLovartHome(t)
	canonical := filepath.Join(paths.DownloadsDir, "task-output")
	got, err := EnforceDownloadConsent(Options{
		Download:               true,
		DownloadDir:            "task-output",
		ConfirmedCanonicalPath: canonical,
	})
	if err != nil {
		t.Fatalf("valid download consent: %v", err)
	}
	if got != canonical {
		t.Fatalf("canonical = %q want %q", got, canonical)
	}
}

func TestValidCanvasConsentAllowsWrite(t *testing.T) {
	err := EnforceCanvasConsent(Options{
		Canvas:                true,
		ProjectID:             "proj_123",
		ConfirmedProjectID:    "proj_123",
		CanvasLayout:          "frame",
		ConfirmCanvasWrite:    true,
		ConfirmedCanvasLayout: "frame",
	})
	if err != nil {
		t.Fatalf("valid canvas consent: %v", err)
	}
}

func withLovartHome(t *testing.T) {
	t.Helper()
	t.Setenv("LOVART_HOME", t.TempDir())
	paths.Reset()
	t.Cleanup(paths.Reset)
}
