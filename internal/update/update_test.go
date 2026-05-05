package update

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
)

func TestSyncSignerWritesRuntimeCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOVART_REVERSE_ROOT", dir)
	paths.Reset()

	wasm := readTestWASM(t)
	httpClient := signerAssetClient(wasm)

	service := &Service{
		HTTP:      httpClient,
		CanvasURL: "https://example.test/canvas",
		Now:       func() time.Time { return time.Date(2026, 5, 5, 1, 2, 3, 0, time.UTC) },
	}
	result, err := service.SyncSigner(context.Background())
	if err != nil {
		t.Fatalf("SyncSigner: %v", err)
	}

	if result.Manifest.SHA256 != metadata.HashBytes(wasm) {
		t.Fatalf("sha = %s, want %s", result.Manifest.SHA256, metadata.HashBytes(wasm))
	}
	if _, err := os.Stat(paths.SignerWASMFile); err != nil {
		t.Fatalf("signer wasm not written: %v", err)
	}
	data, err := os.ReadFile(paths.SignerManifestFile)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	var manifest SignerManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if manifest.SourceURL == "" {
		t.Fatal("source url not recorded")
	}
}

func TestCheckReportsMissingRuntimeCaches(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOVART_REVERSE_ROOT", dir)
	paths.Reset()

	service := &Service{
		HTTP:      signerAssetClient(readTestWASM(t)),
		CanvasURL: "https://example.test/canvas",
	}
	result, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if !result.Detected {
		t.Fatal("expected drift")
	}
	if result.Signer == nil || !result.Signer.Missing {
		t.Fatalf("expected missing signer, got %#v", result.Signer)
	}
	if result.Metadata == nil || !result.Metadata.Missing {
		t.Fatalf("expected missing metadata, got %#v", result.Metadata)
	}
}

func readTestWASM(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../signing/testdata/26bd3a5bd74c3c92.wasm")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type fakeHTTP struct {
	responses map[string][]byte
}

func (f fakeHTTP) Do(req *http.Request) (*http.Response, error) {
	data, ok := f.responses[req.URL.String()]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("not found")),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(data))),
	}, nil
}

func signerAssetClient(wasm []byte) fakeHTTP {
	return fakeHTTP{responses: map[string][]byte{
		"https://example.test/canvas":                                            []byte(`<script src="/lovart_canvas_online/static/js/app.js"></script>`),
		"https://example.test/lovart_canvas_online/static/js/app.js":             []byte(`window.SENTRY_RELEASE={id:"test-release"};const wasm="static/26bd3a5bd74c3c92.wasm";`),
		"https://example.test/lovart_canvas_online/static/26bd3a5bd74c3c92.wasm": wasm,
	}}
}
