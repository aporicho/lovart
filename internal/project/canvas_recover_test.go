package project

import "testing"

func TestRecoverCanvasJSONRemovesMatchedFrameSection(t *testing.T) {
	mutated, err := addBatchToCanvasJSON(syntheticCanvasJSON(), CanvasBatch{Sections: []CanvasSection{{
		Title: "Recover",
		Images: []CanvasImage{
			{TaskID: "task-recover", URL: "https://new/1.png", Width: 1024, Height: 1024},
			{TaskID: "task-recover", URL: "https://new/2.png", Width: 1024, Height: 1024},
		},
	}}})
	if err != nil {
		t.Fatalf("addBatchToCanvasJSON: %v", err)
	}

	recovered, result, err := recoverCanvasJSON(mutated.JSON, "project-123", CanvasRecoverOptions{TaskID: "task-recover"})
	if err != nil {
		t.Fatalf("recoverCanvasJSON: %v", err)
	}
	if len(result.MatchedImages) != 2 {
		t.Fatalf("matched images = %d, want 2", len(result.MatchedImages))
	}
	if len(result.DeletedNodes) != 4 {
		t.Fatalf("deleted nodes = %#v, want 4 nodes", result.DeletedNodes)
	}
	if result.PicCountBefore != 5 || result.PicCountAfter != 3 {
		t.Fatalf("pic count before/after = %d/%d, want 5/3", result.PicCountBefore, result.PicCountAfter)
	}

	store := decodeStore(t, recovered)
	if _, ok := store[result.MatchedImages[0].ID]; ok {
		t.Fatalf("matched image %s still present", result.MatchedImages[0].ID)
	}
	if _, ok := store["shape:oldOneShape0000000001"]; !ok {
		t.Fatalf("old image was removed")
	}
}

func TestRecoverCanvasJSONDoesNotRemoveMixedFrame(t *testing.T) {
	mutated, err := addBatchToCanvasJSON(syntheticCanvasJSON(), CanvasBatch{Sections: []CanvasSection{{
		Title: "Recover",
		Images: []CanvasImage{
			{TaskID: "task-recover", URL: "https://new/1.png", Width: 1024, Height: 1024},
			{TaskID: "task-other", URL: "https://new/2.png", Width: 1024, Height: 1024},
		},
	}}})
	if err != nil {
		t.Fatalf("addBatchToCanvasJSON: %v", err)
	}

	recovered, result, err := recoverCanvasJSON(mutated.JSON, "project-123", CanvasRecoverOptions{TaskID: "task-recover"})
	if err != nil {
		t.Fatalf("recoverCanvasJSON: %v", err)
	}
	if len(result.MatchedImages) != 1 || len(result.DeletedNodes) != 1 {
		t.Fatalf("result = %#v, want only matched image deletion", result)
	}

	store := decodeStore(t, recovered)
	frame := findShapeByType(t, store, "frame")
	if frame == nil {
		t.Fatalf("mixed frame was removed")
	}
	other := findImageByURL(t, store, "https://new/2.png")
	if other == nil {
		t.Fatalf("non-target image was removed")
	}
}
