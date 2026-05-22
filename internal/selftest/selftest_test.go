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
	assertNextAction(t, result.NextAction, "lovart auth login", "lovart_auth_login")
	assertNextAction(t, result.Checks.Auth.NextAction, "lovart auth login", "lovart_auth_login")
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
	assertNextAction(t, result.NextAction, "lovart project list", "lovart_project_list")

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-cookie") || strings.Contains(string(data), "secret-token") {
		t.Fatalf("self-test leaked credential values: %s", data)
	}
}

func TestRuntimeMissingNextActionPriority(t *testing.T) {
	resetRuntimeRoot(t)
	writeCreds(t, map[string]any{
		"cookie":     "secret-cookie",
		"token":      "secret-token",
		"csrf":       "secret-csrf",
		"project_id": "project-123",
		"cid":        "cid-123",
	})

	result := Run()

	assertNextAction(t, result.NextAction, "lovart update sync --all", "")
	if result.Checks.Signer.NextAction == nil {
		t.Fatal("signer next_action is nil")
	}
	if result.Checks.Metadata.NextAction == nil {
		t.Fatal("metadata next_action is nil")
	}
}

func TestMetadataMissingNextActionPriority(t *testing.T) {
	resetRuntimeRoot(t)
	writeCreds(t, map[string]any{
		"cookie":     "secret-cookie",
		"token":      "secret-token",
		"csrf":       "secret-csrf",
		"project_id": "project-123",
		"cid":        "cid-123",
	})
	writeSigner(t)

	result := Run()

	assertNextAction(t, result.NextAction, "lovart update sync --all", "")
	if result.Checks.Signer.NextAction != nil {
		t.Fatalf("signer next_action = %#v, want nil", result.Checks.Signer.NextAction)
	}
	if result.Checks.Metadata.NextAction == nil {
		t.Fatal("metadata next_action is nil")
	}
}

func TestNextActionJSONShapeStable(t *testing.T) {
	resetRuntimeRoot(t)

	data, err := json.Marshal(Run())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`"next_action":{"command":"lovart auth login","mcp_tool":"lovart_auth_login","reason":"credentials are missing","blocking":true,"recoverable":true}`,
		`"auth":{"ok":false,"status":"missing"`,
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("self-test JSON missing stable shape %s: %s", want, data)
		}
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
	if result.NextAction != nil {
		t.Fatalf("next_action = %#v, want nil", result.NextAction)
	}
	if got := result.Checks.Project.Details["project_context_ready"]; got != true {
		t.Fatalf("project_context_ready = %#v, want true", got)
	}
}

func assertNextAction(t *testing.T, action *Action, command, mcpTool string) {
	t.Helper()
	if action == nil {
		t.Fatal("next_action is nil")
	}
	if action.Command != command {
		t.Fatalf("next_action.command = %q, want %q", action.Command, command)
	}
	if action.MCPTool != mcpTool {
		t.Fatalf("next_action.mcp_tool = %q, want %q", action.MCPTool, mcpTool)
	}
	if action.Reason == "" {
		t.Fatal("next_action.reason is empty")
	}
	if !action.Blocking {
		t.Fatal("next_action.blocking = false, want true")
	}
	if !action.Recoverable {
		t.Fatal("next_action.recoverable = false, want true")
	}
}

func resetRuntimeRoot(t *testing.T) {
	t.Helper()
	t.Cleanup(paths.Reset)
	dir := t.TempDir()
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()
	if err := paths.EnsureRuntimeDirs(); err != nil {
		t.Fatal(err)
	}
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
