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

func TestValidateRequestStructuredSchemaFeatures(t *testing.T) {
	setupRegistryMetadata(t)

	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	result := reg.ValidateRequest("openai/gpt-image-2", map[string]any{
		"prompt": "test",
		"images": []any{"a.png", float64(3), "c.png"},
		"options": map[string]any{
			"seed":  "bad",
			"extra": true,
		},
		"mask":  float64(2),
		"style": "drawing",
	})
	if result.OK {
		t.Fatal("expected validation failure")
	}
	wantCodes := map[string]bool{
		"max_items":     false,
		"type":          false,
		"unknown_field": false,
		"enum":          false,
	}
	for _, issue := range result.Issues {
		if _, ok := wantCodes[issue.Code]; ok {
			wantCodes[issue.Code] = true
		}
	}
	for code, seen := range wantCodes {
		if !seen {
			t.Fatalf("missing code %q in %#v", code, result.Issues)
		}
	}
}

func TestRequestFieldsAndOutputCapability(t *testing.T) {
	setupRegistryMetadata(t)

	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fields, err := reg.RequestFields("openai/gpt-image-2")
	if err != nil {
		t.Fatalf("RequestFields: %v", err)
	}
	if len(fields) == 0 || fields[0].Key != "images" {
		t.Fatalf("fields not sorted or missing: %#v", fields)
	}
	cap := reg.OutputCapability("openai/gpt-image-2")
	if cap.MultiField != "n" || cap.MaxOutputs != 10 {
		t.Fatalf("cap = %#v", cap)
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
          "n": {"type": "integer", "minimum": 1, "maximum": 10},
          "images": {"type": "array", "minItems": 1, "maxItems": 2, "items": {"type": "string"}},
          "options": {
            "type": "object",
            "properties": {
              "seed": {"type": "integer"}
            }
          },
          "mask": {"anyOf": [{"type": "string"}, {"type": "null"}]},
          "style": {"allOf": [{"type": "string"}, {"enum": ["photo"]}]}
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
