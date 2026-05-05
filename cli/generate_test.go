package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestGenerateCommandExposesProjectOverrides(t *testing.T) {
	cmd := newGenerateCmd()

	for _, name := range []string{"project-id", "cid"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("generate command missing --%s flag", name)
		}
	}

	if got := cmd.Use; got != "generate <model> --body-file <file> [--project-id <id>] [--mode auto|fast|relax]" {
		t.Fatalf("generate use = %q", got)
	}
}

func TestGenerateValidatesSchemaBeforeSignedClient(t *testing.T) {
	setupCLIRuntimeMetadata(t)
	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "body.json")
	if err := os.WriteFile(bodyPath, []byte(`{"n":20}`), 0644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		cmd := newGenerateCmd()
		cmd.SetArgs([]string{"openai/gpt-image-2", "--body-file", bodyPath, "--dry-run"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if !strings.Contains(output, `"code":"schema_invalid"`) {
		t.Fatalf("expected schema_invalid before auth/signing, got %s", output)
	}
}

func setupCLIRuntimeMetadata(t *testing.T) {
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = write
	defer func() { os.Stdout = original }()

	fn()
	write.Close()
	data, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
