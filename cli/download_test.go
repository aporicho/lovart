package cli

import (
	"strings"
	"testing"

	"github.com/aporicho/lovart/internal/downloads"
	"github.com/aporicho/lovart/internal/project"
)

func TestDownloadCommandExposesTaskAndCanvas(t *testing.T) {
	cmd := newDownloadCmd()
	for _, args := range [][]string{{"task"}, {"canvas"}} {
		found, _, err := cmd.Find(args)
		if err != nil {
			t.Fatalf("download command missing %v: %v", args, err)
		}
		if found == cmd {
			t.Fatalf("download command did not resolve %v", args)
		}
		for _, name := range []string{"download-dir", "download-dir-template", "download-file-template", "overwrite"} {
			if found.Flags().Lookup(name) == nil {
				t.Fatalf("%s missing --%s", found.CommandPath(), name)
			}
		}
	}
}

func TestDownloadTaskSelectorUsesOneBasedIndex(t *testing.T) {
	artifacts := []downloads.Artifact{
		{URL: "https://cdn.example/one.png", Index: 1},
		{URL: "https://cdn.example/two.png", Index: 2},
	}
	selected, env := selectDownloadArtifactIndex(artifacts, 2)
	if env != nil {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if len(selected) != 1 || selected[0].URL != "https://cdn.example/two.png" {
		t.Fatalf("selected = %#v", selected)
	}
	if _, env := selectDownloadArtifactIndex(artifacts, 3); env == nil || env.Error == nil || env.Error.Code != "input_error" {
		t.Fatalf("expected out-of-range input error, got %#v", env)
	}
}

func TestCanvasDownloadSelectorValidation(t *testing.T) {
	for _, tc := range []struct {
		name          string
		artifactID    string
		artifactIndex int
		taskID        string
		all           bool
		wantErr       string
	}{
		{name: "missing", wantErr: "choose exactly one"},
		{name: "conflict", artifactID: "a", all: true, wantErr: "choose exactly one"},
		{name: "negative index", artifactIndex: -1, wantErr: "greater than zero"},
		{name: "artifact", artifactID: "a"},
		{name: "index", artifactIndex: 1},
		{name: "task", taskID: "task-1"},
		{name: "all", all: true},
	} {
		err := validateCanvasDownloadSelector(tc.artifactID, tc.artifactIndex, tc.taskID, tc.all)
		if tc.wantErr == "" && err != nil {
			t.Fatalf("%s unexpected err: %v", tc.name, err)
		}
		if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
			t.Fatalf("%s err = %v, want containing %q", tc.name, err, tc.wantErr)
		}
	}
}

func TestSelectCanvasArtifacts(t *testing.T) {
	artifacts := []project.CanvasArtifact{
		{ArtifactID: "shape:one", Index: 1, URL: "https://cdn.example/one.png", TaskID: "task-a"},
		{ArtifactID: "shape:two", Index: 2, URL: "https://cdn.example/two.png", TaskID: "task-a"},
		{ArtifactID: "shape:three", Index: 3, URL: "https://cdn.example/three.png", TaskID: "task-b"},
	}
	selected, env := selectCanvasArtifacts(artifacts, "", 0, "task-a", false)
	if env != nil {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if len(selected) != 2 {
		t.Fatalf("task selection = %#v", selected)
	}
	selected, env = selectCanvasArtifacts(artifacts, "", 2, "", false)
	if env != nil || len(selected) != 1 || selected[0].ArtifactID != "shape:two" {
		t.Fatalf("index selection = %#v env=%#v", selected, env)
	}
	if _, env := selectCanvasArtifacts(artifacts, "missing", 0, "", false); env == nil || env.Error == nil || env.Error.Code != "input_error" {
		t.Fatalf("expected missing artifact input error, got %#v", env)
	}
}

func TestCanvasDownloadArtifactsPreferOriginalURL(t *testing.T) {
	artifacts := []project.CanvasArtifact{{
		ArtifactID:    "shape:one",
		Index:         4,
		URL:           "https://cdn.example/render.png",
		OriginalURL:   "https://cdn.example/original.png",
		DisplayWidth:  123,
		DisplayHeight: 456,
	}}
	selected := canvasDownloadArtifacts(artifacts, true)
	if len(selected) != 1 || selected[0].URL != "https://cdn.example/original.png" || selected[0].Index != 4 || selected[0].Width != 123 || selected[0].Height != 456 {
		t.Fatalf("download artifacts = %#v", selected)
	}
}
