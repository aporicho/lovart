package jobs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

func TestParseJobsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	data := `{"job_id":"001","title":"test job","model":"openai/gpt-image-2","mode":"relax","outputs":10,"body":{"prompt":"a red cube","quality":"low","size":"1024*1024"}}
{"job_id":"002","model":"seedream/seedream-5-0","mode":"relax","outputs":4,"body":{"prompt":"blue sphere","aspect_ratio":"4:3","resolution":"2K"}}
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	jobs, err := ParseJobsFile(path)
	if err != nil {
		t.Fatalf("ParseJobsFile: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	if jobs[0].JobID != "001" {
		t.Errorf("job[0].JobID = %q, want %q", jobs[0].JobID, "001")
	}
	if jobs[0].Outputs != 10 {
		t.Errorf("job[0].Outputs = %d, want 10", jobs[0].Outputs)
	}
	if jobs[1].Model != "seedream/seedream-5-0" {
		t.Errorf("job[1].Model = %q", jobs[1].Model)
	}
}

func TestParseJobsFile_ValidationErrors(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing model", func(t *testing.T) {
		path := filepath.Join(dir, "no_model.jsonl")
		os.WriteFile(path, []byte(`{"job_id":"001","outputs":1,"body":{}}`), 0644)
		_, err := ParseJobsFile(path)
		if err == nil {
			t.Error("expected error for missing model")
		}
	})

	t.Run("zero outputs", func(t *testing.T) {
		path := filepath.Join(dir, "zero_outputs.jsonl")
		os.WriteFile(path, []byte(`{"job_id":"001","model":"openai/gpt-image-2","outputs":0,"body":{}}`), 0644)
		_, err := ParseJobsFile(path)
		if err == nil {
			t.Error("expected error for zero outputs")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		path := filepath.Join(dir, "invalid.jsonl")
		os.WriteFile(path, []byte(`not json`), 0644)
		_, err := ParseJobsFile(path)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(dir, "empty.jsonl")
		os.WriteFile(path, []byte{}, 0644)
		jobs, err := ParseJobsFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(jobs) != 0 {
			t.Errorf("expected 0 jobs, got %d", len(jobs))
		}
	})
}

func TestValidateJobLines(t *testing.T) {
	setupJobsRegistryMetadata(t)

	lines := []JobLine{{
		Line:    7,
		JobID:   "bad",
		Model:   "openai/gpt-image-2",
		Outputs: 1,
		Body:    map[string]any{"n": float64(99)},
	}}
	err := ValidateJobLines(lines)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(err.Issues) != 1 {
		t.Fatalf("issues = %#v", err.Issues)
	}
	if err.Issues[0].Line != 7 || err.Issues[0].JobID != "bad" {
		t.Fatalf("issue context = %#v", err.Issues[0])
	}
}

func setupJobsRegistryMetadata(t *testing.T) {
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
