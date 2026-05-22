package generation

import (
	"fmt"
	"path/filepath"

	"github.com/aporicho/lovart/internal/paths"
)

// Phase2ConsentError is returned before generation side effects when required
// Phase 2 consent is absent or does not match the exact target. This covers the
// generation submit/project binding and the generation-triggered local download
// and canvas writeback paths. Batch/job finalizers have separate enforcement.
type Phase2ConsentError struct {
	Code    string
	Message string
}

func (e *Phase2ConsentError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("generation: phase 2 consent denied: %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("generation: phase 2 consent denied: %s", e.Code)
}

// EnforceSubmitConsent binds generation remote-write submit effects to the
// selected project before SetMode, TakeSlot, or task submission are attempted.
func EnforceSubmitConsent(opts Options) error {
	if opts.ProjectID == "" {
		return nil
	}
	if opts.ConfirmedProjectID != opts.ProjectID {
		return phase2ConsentError("confirmed_project_id_mismatch", "confirmed_project_id must exactly match project_id before generation submit")
	}
	return nil
}

// EnforceDownloadConsent validates local filesystem-write consent for the
// generation --download path before any directories/files are created.
func EnforceDownloadConsent(opts Options) (string, error) {
	if !opts.Download {
		return "", nil
	}
	root := opts.DownloadDir
	if root == "" {
		root = paths.DownloadsDir
	}
	canonical, err := paths.EnsureWritablePath(paths.DownloadsDir, root)
	if err != nil {
		return "", fmt.Errorf("generation: phase 2 download consent: resolve root: %w", err)
	}
	canonical = filepath.Clean(canonical)
	if filepath.Clean(opts.ConfirmedCanonicalPath) != canonical {
		return "", phase2ConsentError("confirmed_canonical_path_mismatch", "confirmed_canonical_path must exactly match the resolved download root")
	}
	if opts.Overwrite && !opts.ConfirmOverwrite {
		return "", phase2ConsentError("confirm_overwrite_required", "confirm_overwrite=true is required when overwrite=true")
	}
	outside, err := isOutsideDefaultDownloadRoot(canonical)
	if err != nil {
		return "", fmt.Errorf("generation: phase 2 download consent: %w", err)
	}
	if outside && !opts.ConfirmOutsideDefaultRoot {
		return "", phase2ConsentError("confirm_outside_default_root_required", "confirm_outside_default_root=true is required for download roots outside the default Lovart downloads directory")
	}
	return canonical, nil
}

// EnforceCanvasConsent validates project/canvas writeback consent before
// project.AddToCanvas is called. The current generation path has no separate
// layout implementation beyond carrying the user-confirmed layout value.
func EnforceCanvasConsent(opts Options) error {
	if !opts.Canvas {
		return nil
	}
	if !opts.ConfirmCanvasWrite {
		return phase2ConsentError("confirm_canvas_write_required", "confirm_canvas_write=true is required before canvas writeback")
	}
	if opts.ProjectID == "" || opts.ConfirmedProjectID != opts.ProjectID {
		return phase2ConsentError("confirmed_project_id_mismatch", "confirmed_project_id must exactly match project_id before canvas writeback")
	}
	if opts.ConfirmedCanvasLayout == "" {
		return phase2ConsentError("confirmed_canvas_layout_required", "confirmed_canvas_layout is required before canvas writeback")
	}
	if opts.CanvasLayout != "" && opts.ConfirmedCanvasLayout != opts.CanvasLayout {
		return phase2ConsentError("confirmed_canvas_layout_mismatch", "confirmed_canvas_layout must exactly match canvas_layout before canvas writeback")
	}
	return nil
}

func phase2ConsentError(code, message string) error {
	return &Phase2ConsentError{Code: code, Message: message}
}

func isOutsideDefaultDownloadRoot(canonical string) (bool, error) {
	defaultRoot, err := paths.EnsureWritablePath(paths.DownloadsDir, paths.DownloadsDir)
	if err != nil {
		return false, err
	}
	defaultRoot = filepath.Clean(defaultRoot)
	rel, err := filepath.Rel(defaultRoot, canonical)
	if err != nil {
		return false, err
	}
	return rel != "." && (rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) || filepath.IsAbs(rel)), nil
}
