package jobs

import (
	"fmt"
	"path/filepath"

	"github.com/aporicho/lovart/internal/consent"
	"github.com/aporicho/lovart/internal/paths"
)

func enforceFinalizeConsent(state *RunState, opts JobsOptions) error {
	if opts.Download {
		if err := enforceFinalizeDownloadConsent(state, opts); err != nil {
			return err
		}
	}
	if opts.Canvas {
		if err := enforceFinalizeCanvasConsent(state, opts); err != nil {
			return err
		}
	}
	return nil
}

func enforceFinalizeDownloadConsent(state *RunState, opts JobsOptions) error {
	actual, defaultRoot, err := finalizeDownloadRoot(state, opts)
	if err != nil {
		return err
	}
	if opts.ConfirmedCanonicalPath == "" {
		return finalizeConsentDenied("structured_consent_missing_confirmed_canonical_path", "confirmed_canonical_path", consent.StatusMissing)
	}
	confirmed, err := paths.EnsureWritablePath(defaultRoot, opts.ConfirmedCanonicalPath)
	if err != nil || filepath.Clean(confirmed) != actual {
		return finalizeConsentDenied("structured_consent_canonical_path_mismatch", "confirmed_canonical_path", consent.StatusMismatched)
	}
	if finalizePathOutside(defaultRoot, actual) && !opts.ConfirmOutsideDefaultRoot {
		return finalizeConsentDenied("structured_consent_missing_confirm_outside_default_root", "confirm_outside_default_root", consent.StatusMissing)
	}
	if finalizeDownloadWouldOverwrite(state, opts, actual) && !opts.ConfirmOverwrite {
		return finalizeConsentDenied("structured_consent_missing_confirm_overwrite", "confirm_overwrite", consent.StatusMissing)
	}
	return nil
}

func enforceFinalizeCanvasConsent(state *RunState, opts JobsOptions) error {
	if !opts.ConfirmCanvasWrite {
		return finalizeConsentDenied("structured_consent_missing_confirm_canvas_write", "confirm_canvas_write", consent.StatusMissing)
	}
	layout := opts.CanvasLayout
	if layout == "" {
		layout = CanvasLayoutFrame
	}
	if opts.ConfirmedCanvasLayout != layout {
		return finalizeConsentDenied("structured_consent_canvas_layout_mismatch", "confirmed_canvas_layout", consent.StatusMismatched)
	}
	projectID := opts.ProjectID
	if projectID == "" {
		projectID = state.ProjectID
	}
	if opts.ConfirmedProjectID != projectID {
		return finalizeConsentDenied("structured_consent_project_id_mismatch", "confirmed_project_id", consent.StatusMismatched)
	}
	return nil
}

func finalizeDownloadRoot(state *RunState, opts JobsOptions) (actual string, defaultRoot string, err error) {
	defaultRoot = filepath.Join(state.RunDir, "downloads")
	root := opts.DownloadDir
	if root == "" {
		root = defaultRoot
	}
	resolved, err := paths.EnsureWritablePath(defaultRoot, root)
	if err != nil {
		return "", "", fmt.Errorf("jobs: resolve download root: %w", err)
	}
	resolvedDefault, err := paths.EnsureWritablePath(defaultRoot, defaultRoot)
	if err != nil {
		return "", "", fmt.Errorf("jobs: resolve default download root: %w", err)
	}
	return filepath.Clean(resolved), filepath.Clean(resolvedDefault), nil
}

func finalizePathOutside(defaultRoot, actual string) bool {
	rel, err := filepath.Rel(defaultRoot, actual)
	if err != nil {
		return true
	}
	return rel == ".." || filepath.IsAbs(rel) || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)
}

func finalizeDownloadWouldOverwrite(state *RunState, opts JobsOptions, root string) bool {
	// Jobs downloads currently skip existing artifact files and append index records,
	// so there is no overwrite side effect to confirm. Keep this hook next to the
	// consent gate for any future overwrite-enabled finalize option.
	return false
}

func finalizeConsentDenied(code, field, status string) error {
	out := consent.Outcome{
		SchemaVersion: consent.SchemaVersion,
		Allowed:       false,
		Code:          code,
		Fields:        map[string]string{field: status},
	}
	return out
}
