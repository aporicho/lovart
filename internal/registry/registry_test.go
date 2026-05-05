package registry

import (
	"os"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestLoadAndValidate(t *testing.T) {
	setupRegistryMetadata(t)

	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := reg.Model("openai/gpt-image-2"); !ok {
		t.Fatal("model not loaded")
	}

	issues := reg.Validate("openai/gpt-image-2", map[string]any{
		"prompt":  "test",
		"quality": "low",
		"n":       float64(2),
	})
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}

	issues = reg.Validate("openai/gpt-image-2", map[string]any{
		"quality": "extreme",
		"n":       float64(20),
		"extra":   true,
	})
	if len(issues) != 4 {
		t.Fatalf("issues = %v, want 4", issues)
	}
}

func setupRegistryMetadata(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LOVART_REVERSE_ROOT", dir)
	paths.Reset()

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
          "prompt": {"type": "string"},
          "quality": {"type": "string", "enum": ["low", "high"]},
          "n": {"type": "integer", "minimum": 1, "maximum": 10}
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
