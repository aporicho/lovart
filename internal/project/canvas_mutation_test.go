package project

import (
	"reflect"
	"testing"

	"github.com/tidwall/sjson"
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

func TestAddImagesToCanvasJSONPlacesImagesBelowExistingWideCanvas(t *testing.T) {
	jsonStr := wideCanvasJSON(t)
	mutated, err := addImagesToCanvasJSON(jsonStr, []CanvasImage{
		{TaskID: "task-new", URL: "https://new/wide-1.png", Width: 1024, Height: 1024},
		{TaskID: "task-new", URL: "https://new/wide-2.png", Width: 1024, Height: 1024},
	})
	if err != nil {
		t.Fatalf("addImagesToCanvasJSON returned error: %v", err)
	}

	store := decodeStore(t, mutated.JSON)
	first := findImageByURL(t, store, "https://new/wide-1.png")
	second := findImageByURL(t, store, "https://new/wide-2.png")

	x := int(first["x"].(float64))
	y := int(first["y"].(float64))
	if x != 100 {
		t.Fatalf("first new x = %d, want existing min x 100", x)
	}
	if y <= 21950 {
		t.Fatalf("first new y = %d, want below existing bottom 21950", y)
	}
	if x == 33400 || y == 0 {
		t.Fatalf("first new position = (%d,%d), still matches the old far-right append bug", x, y)
	}

	firstIndex := first["index"].(string)
	secondIndex := second["index"].(string)
	if firstIndex <= "b1ZO2A3bV" {
		t.Fatalf("first new index = %q, want above existing max index", firstIndex)
	}
	if secondIndex <= firstIndex {
		t.Fatalf("second new index = %q, want above first new index %q", secondIndex, firstIndex)
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

func wideCanvasJSON(t *testing.T) string {
	t.Helper()

	jsonStr := defaultCanvasJSON()
	oldLeft := `{"x":100,"y":100,"rotation":0,"isLocked":false,"opacity":1,"meta":{"source":"ai"},"id":"shape:aaaaaaaaaaaaaaaaaaaaa","type":"c-image","props":{"w":100,"h":100,"url":"https://old/left.png","originalUrl":"https://old/left.png","radius":0,"name":" Image 1","genType":1,"generatorTaskId":"task-old"},"parentId":"page:page","index":"a1","typeName":"shape"}`
	oldRight := `{"x":32312,"y":20926,"rotation":0,"isLocked":false,"opacity":1,"meta":{"source":"ai"},"id":"shape:bbbbbbbbbbbbbbbbbbbbb","type":"c-image","props":{"w":1024,"h":1024,"url":"https://old/right.png","originalUrl":"https://old/right.png","radius":0,"name":" Image 2","genType":1,"generatorTaskId":"task-old"},"parentId":"page:page","index":"b1ZO2A3bV","typeName":"shape"}`
	var err error
	jsonStr, err = sjson.SetRaw(jsonStr, canvasStorePath+".shape:aaaaaaaaaaaaaaaaaaaaa", oldLeft)
	if err != nil {
		t.Fatalf("insert left shape: %v", err)
	}
	jsonStr, err = sjson.SetRaw(jsonStr, canvasStorePath+".shape:bbbbbbbbbbbbbbbbbbbbb", oldRight)
	if err != nil {
		t.Fatalf("insert right shape: %v", err)
	}
	return jsonStr
}
