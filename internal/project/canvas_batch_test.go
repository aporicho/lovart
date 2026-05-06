package project

import (
	"reflect"
	"testing"
)

func TestAddBatchToCanvasJSONCreatesFrameSection(t *testing.T) {
	mutated, err := addBatchToCanvasJSON(syntheticCanvasJSON(), CanvasBatch{Sections: []CanvasSection{{
		ID:       "cat",
		Title:    "Cat",
		Subtitle: "openai/gpt-image-2 · 4 images",
		Images: []CanvasImage{
			{TaskID: "task-cat", URL: "https://new/cat-1.png", Width: 1024, Height: 1024},
			{TaskID: "task-cat", URL: "https://new/cat-2.png", Width: 1024, Height: 1024},
			{TaskID: "task-cat", URL: "https://new/cat-3.png", Width: 1024, Height: 1024},
			{TaskID: "task-cat", URL: "https://new/cat-4.png", Width: 1024, Height: 1024},
		},
	}}})
	if err != nil {
		t.Fatalf("addBatchToCanvasJSON returned error: %v", err)
	}

	store := decodeStore(t, mutated.JSON)
	assertStoreKeysMatchIDs(t, store)
	assertCanonicalShapeIDs(t, store)

	frame := findShapeByType(t, store, "frame")
	frameID := frame["id"].(string)
	if frame["parentId"] != "page:page" {
		t.Fatalf("frame parentId = %v, want page:page", frame["parentId"])
	}
	if int(frame["x"].(float64)) != 0 || int(frame["y"].(float64)) != 340 {
		t.Fatalf("frame position = (%v,%v), want (0,340)", frame["x"], frame["y"])
	}
	frameProps := frame["props"].(map[string]any)
	if frameProps["name"] != "Cat" || frameProps["color"] != "black" || frameProps["isAutoLayout"] != true {
		t.Fatalf("frame props = %#v", frameProps)
	}
	if int(frameProps["w"].(float64)) != 4596 || int(frameProps["h"].(float64)) != 1424 {
		t.Fatalf("frame size = %vx%v, want 4596x1424", frameProps["w"], frameProps["h"])
	}

	text := findChildByType(t, store, frameID, "text")
	if int(text["x"].(float64)) != 100 || int(text["y"].(float64)) != 100 {
		t.Fatalf("text position = (%v,%v), want (100,100)", text["x"], text["y"])
	}
	if got := richTextPlainText(t, text); got != "Cat · openai/gpt-image-2 · 4 images" {
		t.Fatalf("text = %q", got)
	}

	wantPositions := map[string][2]int{
		"https://new/cat-1.png": {100, 300},
		"https://new/cat-2.png": {1224, 300},
		"https://new/cat-3.png": {2348, 300},
		"https://new/cat-4.png": {3472, 300},
	}
	for url, want := range wantPositions {
		img := findImageByURL(t, store, url)
		if img["parentId"] != frameID {
			t.Fatalf("%s parentId = %v, want %s", url, img["parentId"], frameID)
		}
		if int(img["x"].(float64)) != want[0] || int(img["y"].(float64)) != want[1] {
			t.Fatalf("%s position = (%v,%v), want %v", url, img["x"], img["y"], want)
		}
	}
	if mutated.PicCount != 7 {
		t.Fatalf("pic count = %d, want 7", mutated.PicCount)
	}
	wantCover := []string{
		"https://new/cat-1.png",
		"https://new/cat-2.png",
		"https://new/cat-3.png",
		"https://new/cat-4.png",
	}
	if !reflect.DeepEqual(mutated.CoverList, wantCover) {
		t.Fatalf("cover list = %#v, want %#v", mutated.CoverList, wantCover)
	}
}

func TestAddBatchToCanvasJSONWrapsAndScalesLargeImages(t *testing.T) {
	mutated, err := addBatchToCanvasJSON("", CanvasBatch{Sections: []CanvasSection{{
		ID:    "large",
		Title: "Large",
		Images: []CanvasImage{
			{TaskID: "task-large", URL: "https://new/1.png", Width: 4096, Height: 2048},
			{TaskID: "task-large", URL: "https://new/2.png", Width: 4096, Height: 2048},
			{TaskID: "task-large", URL: "https://new/3.png", Width: 4096, Height: 2048},
			{TaskID: "task-large", URL: "https://new/4.png", Width: 4096, Height: 2048},
			{TaskID: "task-large", URL: "https://new/5.png", Width: 4096, Height: 2048},
		},
	}}})
	if err != nil {
		t.Fatalf("addBatchToCanvasJSON returned error: %v", err)
	}

	store := decodeStore(t, mutated.JSON)
	frame := findShapeByType(t, store, "frame")
	frameID := frame["id"].(string)
	frameProps := frame["props"].(map[string]any)
	if int(frameProps["w"].(float64)) != 4596 || int(frameProps["h"].(float64)) != 1524 {
		t.Fatalf("frame size = %vx%v, want 4596x1524", frameProps["w"], frameProps["h"])
	}

	fifth := findImageByURL(t, store, "https://new/5.png")
	if fifth["parentId"] != frameID {
		t.Fatalf("fifth parentId = %v, want %s", fifth["parentId"], frameID)
	}
	if int(fifth["x"].(float64)) != 100 || int(fifth["y"].(float64)) != 912 {
		t.Fatalf("fifth position = (%v,%v), want (100,912)", fifth["x"], fifth["y"])
	}
	props := fifth["props"].(map[string]any)
	if int(props["w"].(float64)) != 1024 || int(props["h"].(float64)) != 512 {
		t.Fatalf("fifth size = %vx%v, want 1024x512", props["w"], props["h"])
	}
}
