// Package selftest provides local diagnostics for the Lovart runtime.
package selftest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
	"github.com/aporicho/lovart/internal/registry"
	"github.com/aporicho/lovart/internal/signing"
	"github.com/aporicho/lovart/internal/version"
)

const (
	StatusReady      = "ready"
	StatusNeedsSetup = "needs_setup"
	StatusBroken     = "broken"

	CheckOK         = "ok"
	CheckMissing    = "missing"
	CheckIncomplete = "incomplete"
)

// Result is the public self-test report.
type Result struct {
	Status             string   `json:"status"`
	Version            string   `json:"version"`
	Root               string   `json:"root"`
	Checks             Checks   `json:"checks"`
	RecommendedActions []string `json:"recommended_actions,omitempty"`
}

// Checks groups all local diagnostics.
type Checks struct {
	CLI      Check `json:"cli"`
	Auth     Check `json:"auth"`
	Project  Check `json:"project"`
	Signer   Check `json:"signer"`
	Metadata Check `json:"metadata"`
	Registry Check `json:"registry"`
}

// Check is one diagnostic result. Details contains non-secret check-specific
// facts such as project_id_present or model_count.
type Check struct {
	OK                 bool           `json:"ok"`
	Status             string         `json:"status"`
	Path               string         `json:"path,omitempty"`
	Source             string         `json:"source,omitempty"`
	Fields             []string       `json:"fields,omitempty"`
	Error              string         `json:"error,omitempty"`
	Details            map[string]any `json:"details,omitempty"`
	RecommendedActions []string       `json:"recommended_actions,omitempty"`
}

type credsSnapshot struct {
	source     string
	missing    bool
	readErr    error
	parseErr   error
	creds      *auth.Credentials
	credsErr   error
	project    *auth.ProjectContext
	projectErr error
}

// Run executes all self-test checks without network access.
func Run() Result {
	creds := loadCredsSnapshot()
	checks := Checks{
		CLI:      checkCLI(),
		Auth:     checkAuth(creds),
		Project:  checkProject(creds),
		Signer:   checkSigner(),
		Metadata: checkMetadata(),
		Registry: checkRegistry(),
	}

	all := []Check{checks.CLI, checks.Auth, checks.Project, checks.Signer, checks.Metadata, checks.Registry}
	return Result{
		Status:             aggregateStatus(all),
		Version:            version.Version,
		Root:               paths.Root,
		Checks:             checks,
		RecommendedActions: recommendedActions(all),
	}
}

func checkCLI() Check {
	return Check{
		OK:     true,
		Status: CheckOK,
		Details: map[string]any{
			"package":                version.Package,
			"version":                version.Version,
			"go_version":             version.GoVersion(),
			"runtime_root":           paths.Root,
			"creds_file":             paths.CredsFile,
			"signer_wasm_file":       paths.SignerWASMFile,
			"metadata_manifest_file": paths.MetadataManifestFile,
			"generator_list_file":    paths.GeneratorListFile,
			"generator_schema_file":  paths.GeneratorSchemaFile,
		},
	}
}

func checkAuth(snapshot credsSnapshot) Check {
	if snapshot.missing {
		return Check{
			OK:                 false,
			Status:             CheckMissing,
			Source:             snapshot.source,
			Error:              "credentials file not found",
			RecommendedActions: []string{"run `lovart-reverse start`", "run `lovart-reverse extract captures/<file>.json`"},
		}
	}
	if snapshot.readErr != nil {
		return Check{
			OK:                 false,
			Status:             StatusBroken,
			Source:             snapshot.source,
			Error:              snapshot.readErr.Error(),
			RecommendedActions: []string{"check credentials file permissions"},
		}
	}
	if snapshot.parseErr != nil {
		return Check{
			OK:                 false,
			Status:             StatusBroken,
			Source:             snapshot.source,
			Error:              snapshot.parseErr.Error(),
			RecommendedActions: []string{"rerun `lovart-reverse extract captures/<file>.json`"},
		}
	}
	if snapshot.credsErr != nil {
		return Check{
			OK:                 false,
			Status:             CheckIncomplete,
			Source:             snapshot.source,
			Error:              snapshot.credsErr.Error(),
			RecommendedActions: []string{"rerun `lovart-reverse extract captures/<file>.json`"},
		}
	}
	fields := credentialFields(snapshot.creds)
	return Check{
		OK:     true,
		Status: CheckOK,
		Source: snapshot.source,
		Fields: fields,
	}
}

func checkProject(snapshot credsSnapshot) Check {
	details := map[string]any{
		"project_id_present": false,
		"cid_present":        false,
	}
	if snapshot.missing {
		return Check{
			OK:                 false,
			Status:             CheckMissing,
			Source:             snapshot.source,
			Error:              "credentials file not found",
			Details:            details,
			RecommendedActions: []string{"run `lovart-reverse extract captures/<file>.json`"},
		}
	}
	if snapshot.readErr != nil || snapshot.parseErr != nil {
		err := snapshot.readErr
		if err == nil {
			err = snapshot.parseErr
		}
		return Check{
			OK:                 false,
			Status:             StatusBroken,
			Source:             snapshot.source,
			Error:              err.Error(),
			Details:            details,
			RecommendedActions: []string{"rerun `lovart-reverse extract captures/<file>.json`"},
		}
	}
	if snapshot.projectErr != nil {
		return Check{
			OK:                 false,
			Status:             CheckIncomplete,
			Source:             snapshot.source,
			Error:              snapshot.projectErr.Error(),
			Details:            details,
			RecommendedActions: []string{"run `lovart project list`", "run `lovart project select <project_id>`"},
		}
	}

	projectIDPresent := snapshot.project != nil && snapshot.project.ProjectID != ""
	cidPresent := snapshot.project != nil && snapshot.project.CID != ""
	details["project_id_present"] = projectIDPresent
	details["cid_present"] = cidPresent
	fields := make([]string, 0, 2)
	if projectIDPresent {
		fields = append(fields, "project_id")
	}
	if cidPresent {
		fields = append(fields, "cid")
	}
	if projectIDPresent && cidPresent {
		return Check{OK: true, Status: CheckOK, Source: snapshot.source, Fields: fields, Details: details}
	}

	actions := []string{"run `lovart project list`", "run `lovart project select <project_id>`"}
	if !cidPresent {
		actions = append(actions, "rerun `lovart-reverse extract captures/<file>.json` to capture cid")
	}
	return Check{
		OK:                 false,
		Status:             CheckIncomplete,
		Source:             snapshot.source,
		Fields:             fields,
		Error:              "project_id and cid are required for generation",
		Details:            details,
		RecommendedActions: actions,
	}
}

func checkSigner() Check {
	if _, err := os.Stat(paths.SignerWASMFile); err != nil {
		status := CheckMissing
		if !errors.Is(err, os.ErrNotExist) {
			status = StatusBroken
		}
		return Check{
			OK:                 false,
			Status:             status,
			Path:               paths.SignerWASMFile,
			Error:              err.Error(),
			RecommendedActions: []string{"run `lovart update sync --all`"},
		}
	}

	signer, err := signing.NewSigner()
	if err != nil {
		status := StatusBroken
		if errors.Is(err, signing.ErrNoSigner) {
			status = CheckMissing
		}
		return Check{
			OK:                 false,
			Status:             status,
			Path:               paths.SignerWASMFile,
			Error:              err.Error(),
			RecommendedActions: []string{"run `lovart update sync --all`"},
		}
	}
	defer closeSigner(signer)

	if err := signer.Health(); err != nil {
		return Check{
			OK:                 false,
			Status:             StatusBroken,
			Path:               paths.SignerWASMFile,
			Error:              err.Error(),
			RecommendedActions: []string{"run `lovart update sync --all`"},
		}
	}
	result, err := signer.Sign(context.Background(), signing.SigningPayload{
		Timestamp: "1746600000000",
		ReqUUID:   "test1234567890abcdef1234567890ab",
	})
	if err != nil {
		return Check{
			OK:                 false,
			Status:             StatusBroken,
			Path:               paths.SignerWASMFile,
			Error:              err.Error(),
			RecommendedActions: []string{"run `lovart update sync --all`"},
		}
	}
	if result.Signature == "" {
		return Check{
			OK:                 false,
			Status:             StatusBroken,
			Path:               paths.SignerWASMFile,
			Error:              "signer returned empty signature",
			RecommendedActions: []string{"run `lovart update sync --all`"},
		}
	}
	return Check{
		OK:     true,
		Status: CheckOK,
		Path:   paths.SignerWASMFile,
		Details: map[string]any{
			"signature_generated": true,
		},
	}
}

func checkMetadata() Check {
	manifest, err := metadata.ReadManifest()
	if err != nil {
		status := StatusBroken
		if errors.Is(err, os.ErrNotExist) {
			status = CheckMissing
		}
		return Check{
			OK:                 false,
			Status:             status,
			Path:               paths.MetadataManifestFile,
			Error:              err.Error(),
			RecommendedActions: []string{"run `lovart update sync --all`"},
		}
	}

	missing := missingMetadataFiles()
	if len(missing) > 0 {
		return Check{
			OK:                 false,
			Status:             CheckMissing,
			Path:               paths.MetadataManifestFile,
			Error:              fmt.Sprintf("metadata cache files missing: %s", strings.Join(missing, ", ")),
			Details:            map[string]any{"missing_files": missing},
			RecommendedActions: []string{"run `lovart update sync --all`"},
		}
	}
	return Check{
		OK:     true,
		Status: CheckOK,
		Path:   paths.MetadataManifestFile,
		Details: map[string]any{
			"generator_list_hash":   manifest.GeneratorListHash,
			"generator_schema_hash": manifest.GeneratorSchemaHash,
			"synced_at":             manifest.SyncedAt,
		},
	}
}

func checkRegistry() Check {
	reg, err := registry.Load()
	if err != nil {
		status := StatusBroken
		if errors.Is(err, os.ErrNotExist) {
			status = CheckMissing
		}
		return Check{
			OK:                 false,
			Status:             status,
			Error:              err.Error(),
			RecommendedActions: []string{"run `lovart update sync --all`"},
		}
	}
	models := reg.Models()
	if len(models) == 0 {
		return Check{
			OK:                 false,
			Status:             StatusBroken,
			Error:              "registry contains no models",
			RecommendedActions: []string{"run `lovart update sync --all`"},
		}
	}
	return Check{
		OK:     true,
		Status: CheckOK,
		Details: map[string]any{
			"model_count": len(models),
		},
	}
}

func loadCredsSnapshot() credsSnapshot {
	source, missing, readErr := credentialSource()
	snapshot := credsSnapshot{source: source, missing: missing, readErr: readErr}
	if missing || readErr != nil {
		return snapshot
	}

	data, err := os.ReadFile(source)
	if err != nil {
		snapshot.readErr = err
		return snapshot
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		snapshot.parseErr = fmt.Errorf("auth: parse creds file: %w", err)
		return snapshot
	}

	snapshot.creds, snapshot.credsErr = auth.Load()
	snapshot.project, snapshot.projectErr = auth.LoadProjectContext()
	return snapshot
}

func credentialSource() (string, bool, error) {
	if _, err := os.Stat(paths.CredsFile); err == nil {
		return paths.CredsFile, false, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return paths.CredsFile, false, err
	}

	legacy := filepath.Join(paths.Root, "scripts", "creds.json")
	if _, err := os.Stat(legacy); err == nil {
		return legacy, false, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return legacy, false, err
	}
	return paths.CredsFile, true, nil
}

func credentialFields(creds *auth.Credentials) []string {
	if creds == nil {
		return nil
	}
	fields := make([]string, 0, 3)
	if creds.Cookie != "" {
		fields = append(fields, "cookie")
	}
	if creds.Token != "" {
		fields = append(fields, "token")
	}
	if creds.CSRF != "" {
		fields = append(fields, "csrf")
	}
	return fields
}

func missingMetadataFiles() []string {
	var missing []string
	for _, path := range []string{paths.GeneratorListFile, paths.GeneratorSchemaFile} {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			missing = append(missing, path)
		}
	}
	return missing
}

func closeSigner(signer signing.Signer) {
	if closer, ok := signer.(interface{ Close(context.Context) error }); ok {
		_ = closer.Close(context.Background())
	}
}

func aggregateStatus(checks []Check) string {
	status := StatusReady
	for _, check := range checks {
		if check.Status == StatusBroken {
			return StatusBroken
		}
		if !check.OK {
			status = StatusNeedsSetup
		}
	}
	return status
}

func recommendedActions(checks []Check) []string {
	var out []string
	seen := map[string]bool{}
	for _, check := range checks {
		for _, action := range check.RecommendedActions {
			if action == "" || seen[action] {
				continue
			}
			seen[action] = true
			out = append(out, action)
		}
	}
	return out
}
