package downloads

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestDownloadArtifactsRoutesByFieldsAndEmbedsPNGMetadata(t *testing.T) {
	pngBytes := testPNG(t)
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	root := t.TempDir()
	result, err := DownloadArtifacts(context.Background(), []Artifact{{
		URL:    server.URL + "/artifact.png",
		Width:  2048,
		Height: 1152,
		Index:  1,
	}}, Options{
		RootDir:      root,
		DirTemplate:  "{{fields.series}}/{{fields.scene_no}} {{fields.scene_name}}",
		FileTemplate: "artifact-{{artifact.index:02}}.{{ext}}",
		TaskID:       "task-secret",
		Context: JobContext{
			Model: "openai/gpt-image-2",
			Mode:  "relax",
			Fields: map[string]any{
				"series":     "fanrenxiuxian",
				"scene_no":   "001",
				"scene_name": "凡人少年初入仙途",
			},
			Body: map[string]any{
				"prompt":  "a cinematic scene",
				"quality": "medium",
				"size":    "2048*1152",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Error != "" {
		t.Fatalf("download result = %#v", result.Files)
	}
	wantPath := filepath.Join(root, "fanrenxiuxian", "001 凡人少年初入仙途", "artifact-01.png")
	if result.Files[0].Path != wantPath {
		t.Fatalf("path = %q, want %q", result.Files[0].Path, wantPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("lovart.effect.v1")) || !bytes.Contains(data, []byte("a cinematic scene")) {
		t.Fatalf("PNG metadata not embedded")
	}
	if bytes.Contains(data, []byte("task-secret")) || bytes.Contains(data, []byte("fanrenxiuxian")) {
		t.Fatalf("runtime or routing metadata leaked into image metadata")
	}

	second, err := DownloadArtifacts(context.Background(), []Artifact{{URL: server.URL + "/artifact.png", Index: 1}}, Options{
		RootDir:      root,
		DirTemplate:  "{{fields.series}}/{{fields.scene_no}} {{fields.scene_name}}",
		FileTemplate: "artifact-{{artifact.index:02}}.{{ext}}",
		Context: JobContext{Fields: map[string]any{
			"series":     "fanrenxiuxian",
			"scene_no":   "001",
			"scene_name": "凡人少年初入仙途",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Files[0].Existing {
		t.Fatalf("expected existing result: %#v", second.Files[0])
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("download was repeated, hits=%d", got)
	}
}

func TestDefaultDirectoryFallsBackToTitlePrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(testPNG(t))
	}))
	defer server.Close()

	root := t.TempDir()
	result, err := DownloadArtifacts(context.Background(), []Artifact{{URL: server.URL + "/a.png", Index: 1}}, Options{
		RootDir: root,
		Context: JobContext{
			Title: "001 凡人少年初入仙途 / 电影写实 / openai/gpt-image-2",
			Body:  map[string]any{"prompt": "x"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "001 凡人少年初入仙途", "artifact-01.png")
	if result.Files[0].Path != want {
		t.Fatalf("path = %q, want %q", result.Files[0].Path, want)
	}
}

func TestEmbedEffectMetadataForMajorImageFormats(t *testing.T) {
	metadata := EffectMetadata{
		SchemaVersion: 1,
		Model:         "model",
		InputArgs:     map[string]any{"prompt": "visible prompt"},
		Artifact:      EffectArtifactRef{Index: 1},
	}
	cases := []struct {
		name     string
		data     []byte
		contains []byte
	}{
		{name: "jpg", data: []byte{0xff, 0xd8, 0xff, 0xd9}, contains: []byte("lovart:Effect")},
		{name: "webp", data: testWebP(), contains: []byte("lovart:Effect")},
		{name: "gif", data: []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), contains: []byte("LOVART_EFFECT_V1")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "image."+tc.name)
			if err := os.WriteFile(path, tc.data, 0644); err != nil {
				t.Fatal(err)
			}
			format, embedded, err := embedEffectMetadata(path, metadata)
			if err != nil {
				t.Fatal(err)
			}
			if !embedded || format == "" {
				t.Fatalf("not embedded: format=%q embedded=%v", format, embedded)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(data, tc.contains) || !bytes.Contains(data, []byte("visible prompt")) {
				t.Fatalf("metadata missing for %s", tc.name)
			}
		})
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testWebP() []byte {
	data := []byte{
		'R', 'I', 'F', 'F', 0, 0, 0, 0,
		'W', 'E', 'B', 'P',
		'V', 'P', '8', 'X', 10, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	return data
}
