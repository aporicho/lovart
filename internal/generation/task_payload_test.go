package generation

import (
	"os"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestBuildNormalizedTaskPayloadUsesSchemaDefaults(t *testing.T) {
	setupGenerationRegistryMetadata(t)

	body := map[string]any{
		"prompt": "test",
	}
	payload, normalized, err := buildNormalizedTaskPayload("openai/gpt-image-2", body, Options{
		ProjectID: "project-123",
		CID:       "cid-123",
	})
	if err != nil {
		t.Fatalf("buildNormalizedTaskPayload: %v", err)
	}
	if normalized["resolution"] != "1K" {
		t.Fatalf("normalized resolution = %#v, want 1K", normalized["resolution"])
	}
	if _, ok := body["resolution"]; ok {
		t.Fatalf("normalization mutated input body: %#v", body)
	}
	inputArgs, ok := payload["input_args"].(map[string]any)
	if !ok {
		t.Fatalf("input_args = %#v, want map", payload["input_args"])
	}
	if inputArgs["resolution"] != "1K" {
		t.Fatalf("payload resolution = %#v, want 1K", inputArgs["resolution"])
	}
	if payload["project_id"] != "project-123" || payload["cid"] != "cid-123" {
		t.Fatalf("payload context = %#v", payload)
	}
}

func setupGenerationRegistryMetadata(t *testing.T) {
	t.Helper()
	t.Cleanup(paths.Reset)
	dir := t.TempDir()
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()
	if err := os.MkdirAll(paths.MetadataDir, 0755); err != nil {
		t.Fatal(err)
	}

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
        "properties": {
          "prompt": {"type": "string"},
          "resolution": {"type": "string", "default": "1K"}
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
}
