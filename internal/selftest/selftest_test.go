package selftest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
)

func TestEmptyRuntimeNeedsSetup(t *testing.T) {
	resetRuntimeRoot(t)

	result := Run()

	if result.Status != StatusNeedsSetup {
		t.Fatalf("status = %q, want %q", result.Status, StatusNeedsSetup)
	}
	if result.Checks.Auth.Status != CheckMissing {
		t.Fatalf("auth status = %q, want missing", result.Checks.Auth.Status)
	}
	if result.Checks.Signer.Status != CheckMissing {
		t.Fatalf("signer status = %q, want missing", result.Checks.Signer.Status)
	}
	if result.Checks.Metadata.Status != CheckMissing {
		t.Fatalf("metadata status = %q, want missing", result.Checks.Metadata.Status)
	}
	if result.Checks.Registry.Status != CheckMissing {
		t.Fatalf("registry status = %q, want missing", result.Checks.Registry.Status)
	}
}

func TestInvalidCredentialsMarkAuthBroken(t *testing.T) {
	resetRuntimeRoot(t)
	if err := os.WriteFile(paths.CredsFile, []byte(`{"cookie":`), 0600); err != nil {
		t.Fatal(err)
	}

	result := Run()

	if result.Status != StatusBroken {
		t.Fatalf("status = %q, want %q", result.Status, StatusBroken)
	}
	if result.Checks.Auth.Status != StatusBroken {
		t.Fatalf("auth status = %q, want broken", result.Checks.Auth.Status)
	}
	if !strings.Contains(result.Checks.Auth.Error, "parse creds file") {
		t.Fatalf("auth error = %q, want parse error", result.Checks.Auth.Error)
	}
}

func TestCredentialsWithoutProjectContextNeedSetup(t *testing.T) {
	resetRuntimeRoot(t)
	writeCreds(t, map[string]any{
		"cookie": "secret-cookie",
		"token":  "secret-token",
	})

	result := Run()

	if result.Status != StatusNeedsSetup {
		t.Fatalf("status = %q, want %q", result.Status, StatusNeedsSetup)
	}
	if !result.Checks.Auth.OK {
		t.Fatalf("auth check not ok: %#v", result.Checks.Auth)
	}
	if result.Checks.Project.Status != CheckIncomplete {
		t.Fatalf("project status = %q, want incomplete", result.Checks.Project.Status)
	}
	if got := result.Checks.Project.Details["project_id_present"]; got != false {
		t.Fatalf("project_id_present = %#v, want false", got)
	}
	if got := result.Checks.Project.Details["project_context_ready"]; got != false {
		t.Fatalf("project_context_ready = %#v, want false", got)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-cookie") || strings.Contains(string(data), "secret-token") {
		t.Fatalf("self-test leaked credential values: %s", data)
	}
}

func TestReadyRuntimePasses(t *testing.T) {
	resetRuntimeRoot(t)
	writeCreds(t, map[string]any{
		"cookie":     "secret-cookie",
		"token":      "secret-token",
		"csrf":       "secret-csrf",
		"project_id": "project-123",
		"cid":        "cid-123",
	})
	writeSigner(t)
	writeMetadata(t)

	result := Run()

	if result.Status != StatusReady {
		t.Fatalf("status = %q, want %q: %#v", result.Status, StatusReady, result)
	}
	if result.Checks.Signer.Details["signature_generated"] != true {
		t.Fatalf("signature_generated = %#v, want true", result.Checks.Signer.Details["signature_generated"])
	}
	if result.Checks.Registry.Details["model_count"] != 1 {
		t.Fatalf("model_count = %#v, want 1", result.Checks.Registry.Details["model_count"])
	}
	if len(result.RecommendedActions) != 0 {
		t.Fatalf("recommended_actions = %#v, want empty", result.RecommendedActions)
	}
	if got := result.Checks.Project.Details["project_context_ready"]; got != true {
		t.Fatalf("project_context_ready = %#v, want true", got)
	}
}

func resetRuntimeRoot(t *testing.T) {
	t.Helper()
	t.Cleanup(paths.Reset)
	dir := t.TempDir()
	t.Setenv("LOVART_REVERSE_ROOT", dir)
	paths.Reset()
}

func writeCreds(t *testing.T, value map[string]any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.CredsFile, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func writeSigner(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "signing", "testdata", "26bd3a5bd74c3c92.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.SignerWASMFile, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func writeMetadata(t *testing.T) {
	t.Helper()
	list := []byte(`{"items":[{"name":"openai/gpt-image-2","display_name":"GPT Image 2","type":"image"}]}`)
	schema := []byte(`{
  "paths": {
    "/openai/gpt-image-2": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/GPTImage2Request"}
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "GPTImage2Request": {
        "required": ["prompt"],
        "properties": {
          "prompt": {"type": "string"}
        }
      }
    }
  }
}`)
	if err := os.WriteFile(paths.GeneratorListFile, list, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.GeneratorSchemaFile, schema, 0644); err != nil {
		t.Fatal(err)
	}
	if err := metadata.WriteManifest(&metadata.Manifest{
		Version:             1,
		Source:              "test",
		GeneratorListHash:   "list-hash",
		GeneratorSchemaHash: "schema-hash",
		SyncedAt:            time.Unix(1700000000, 0).UTC(),
	}); err != nil {
		t.Fatal(err)
	}
}
