package project

import "testing"

func TestSaveCanvasBodyIncludesCID(t *testing.T) {
	body := saveCanvasBody("project-123", "cid-456", "session-789", &canvasState{
		Canvas:   "SHAKKERDATA://canvas",
		Version:  "version-1",
		Name:     "Project",
		PicCount: 2,
	}, []string{"https://cover/1.png"})

	if body["cid"] != "cid-456" {
		t.Fatalf("cid = %v, want cid-456", body["cid"])
	}
	if body["projectId"] != "project-123" || body["sessionId"] != "session-789" {
		t.Fatalf("body project/session fields = %#v", body)
	}
}

func TestSaveCanvasBodyOmitsEmptyCID(t *testing.T) {
	body := saveCanvasBody("project-123", "", "session-789", &canvasState{}, nil)
	if _, ok := body["cid"]; ok {
		t.Fatalf("body includes empty cid: %#v", body)
	}
}
