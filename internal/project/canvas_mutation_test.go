package project

import (
	"reflect"
	"testing"
)

func TestAddImagesToCanvasJSONMaintainsTldrawInvariants(t *testing.T) {
	mutated, err := addImagesToCanvasJSON(syntheticCanvasJSON(), []CanvasImage{
		{TaskID: "task-new", URL: "https://new/1.png", Width: 512, Height: 512},
		{TaskID: "task-new", URL: "https://new/2.png", Width: 1024, Height: 768},
	})
	if err != nil {
		t.Fatalf("addImagesToCanvasJSON returned error: %v", err)
	}

	store := decodeStore(t, mutated.JSON)
	assertStoreKeysMatchIDs(t, store)

	if mutated.PicCount != 5 {
		t.Fatalf("pic count = %d, want 5", mutated.PicCount)
	}

	wantCover := []string{
		"https://old/2.png",
		"https://old/3.png",
		"https://new/1.png",
		"https://new/2.png",
	}
	if !reflect.DeepEqual(mutated.CoverList, wantCover) {
		t.Fatalf("cover list = %#v, want %#v", mutated.CoverList, wantCover)
	}

	first := findImageByURL(t, store, "https://new/1.png")
	second := findImageByURL(t, store, "https://new/2.png")
	assertImageNode(t, first, "task-new", " Image 4", 512, 512)
	assertImageNode(t, second, "task-new", " Image 5", 1024, 768)
	assertCanonicalShapeIDs(t, store)

	firstIndex := first["index"].(string)
	secondIndex := second["index"].(string)
	if firstIndex != "a5" {
		t.Fatalf("first new index = %q, want a5", firstIndex)
	}
	if secondIndex != "a6" {
		t.Fatalf("second new index = %q, want a6", secondIndex)
	}
}

func TestAddImagesToCanvasJSONCreatesDefaultCanvas(t *testing.T) {
	mutated, err := addImagesToCanvasJSON("", []CanvasImage{
		{TaskID: "task-empty", URL: "https://new/empty.png", Width: 256, Height: 128},
	})
	if err != nil {
		t.Fatalf("addImagesToCanvasJSON returned error: %v", err)
	}

	store := decodeStore(t, mutated.JSON)
	assertStoreKeysMatchIDs(t, store)

	if _, ok := store["document:document"]; !ok {
		t.Fatalf("default canvas missing document record")
	}
	if _, ok := store["page:page"]; !ok {
		t.Fatalf("default canvas missing page record")
	}
	if mutated.PicCount != 1 {
		t.Fatalf("pic count = %d, want 1", mutated.PicCount)
	}
	wantCover := []string{"https://new/empty.png"}
	if !reflect.DeepEqual(mutated.CoverList, wantCover) {
		t.Fatalf("cover list = %#v, want %#v", mutated.CoverList, wantCover)
	}

	sequences := decodeSchemaSequences(t, mutated.JSON)
	assertSchemaSequence(t, sequences, "com.tldraw.store", 5)
	assertSchemaSequence(t, sequences, "com.tldraw.shape.frame", 1)
	assertSchemaSequence(t, sequences, "com.tldraw.shape.text", 4)
	assertSchemaSequence(t, sequences, "com.tldraw.shape.c-image", 0)
}
