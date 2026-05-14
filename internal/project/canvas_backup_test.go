package project

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestBackupCanvasBeforeWriteStoresRawDecodedAndManifest(t *testing.T) {
	resetProjectRuntimeRoot(t)

	encoded, err := encodeCanvasJSON(syntheticCanvasJSON())
	if err != nil {
		t.Fatalf("encode canvas: %v", err)
	}
	manifestPath, err := backupCanvasBeforeWrite("project-123", &canvasState{
		Canvas:  encoded,
		Version: "version-1",
		Name:    "Project",
	})
	if err != nil {
		t.Fatalf("backupCanvasBeforeWrite: %v", err)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest canvasBackupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if manifest.ProjectID != "project-123" || manifest.ProjectName != "Project" || manifest.Version != "version-1" {
		t.Fatalf("manifest project fields = %#v", manifest)
	}
	if manifest.PicCount != 3 {
		t.Fatalf("manifest pic count = %d, want 3", manifest.PicCount)
	}
	if len(manifest.CoverList) != 3 {
		t.Fatalf("manifest cover list = %#v", manifest.CoverList)
	}
	for _, path := range []string{manifest.CanvasFile, manifest.DecodedFile} {
		if path == "" {
			t.Fatalf("manifest missing backup file path: %#v", manifest)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("backup file %s missing: %v", path, err)
		}
	}
}

func TestSaveCanvasWithBackupStopsWhenBackupFails(t *testing.T) {
	resetProjectRuntimeRoot(t)
	if err := os.WriteFile(filepath.Join(paths.Root, "backups"), []byte("not a directory"), 0600); err != nil {
		t.Fatalf("create blocking backups file: %v", err)
	}

	_, err := saveCanvasWithBackup(context.Background(), nil, "project-123", "cid", &canvasState{
		Canvas: "SHAKKERDATA://invalid",
	}, &canvasState{})
	if err == nil {
		t.Fatal("saveCanvasWithBackup succeeded, want backup error")
	}
	if !strings.Contains(err.Error(), "canvas backup: create dir") {
		t.Fatalf("error = %v, want backup create failure", err)
	}
}

func resetProjectRuntimeRoot(t *testing.T) {
	t.Helper()
	t.Setenv("LOVART_HOME", t.TempDir())
	paths.Reset()
	if err := paths.EnsureRuntimeDirs(); err != nil {
		t.Fatalf("prepare runtime: %v", err)
	}
}
