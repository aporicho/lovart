package project

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestNormalizeCanvasJSONRepairsTextAndIndexes(t *testing.T) {
	mutated, result, err := normalizeCanvasJSON(corruptCanvasJSON())
	if err != nil {
		t.Fatalf("normalizeCanvasJSON returned error: %v", err)
	}
	if !result.Changed {
		t.Fatalf("normalizeCanvasJSON did not report changes")
	}
	if result.NormalizedTexts != 1 {
		t.Fatalf("normalized texts = %d, want 1", result.NormalizedTexts)
	}
	if result.NormalizedIndexes != 3 {
		t.Fatalf("normalized indexes = %d, want 3", result.NormalizedIndexes)
	}
	if result.NormalizedSchemaSequences == 0 {
		t.Fatalf("normalized schema sequences = 0, want repairs")
	}
	if result.NormalizedShapeIDs != 3 {
		t.Fatalf("normalized shape ids = %d, want 3", result.NormalizedShapeIDs)
	}
	if result.PicCount != 1 {
		t.Fatalf("pic count = %d, want 1", result.PicCount)
	}

	store := decodeStore(t, mutated.JSON)
	assertCanonicalShapeIDs(t, store)
	frame := findShapeByType(t, store, "frame")
	frameID := frame["id"].(string)
	if frame["index"] != "a1" {
		t.Fatalf("frame index = %v, want a1", frame["index"])
	}
	text := findChildByType(t, store, frameID, "text")
	if text["index"] != "a1" {
		t.Fatalf("text index = %v, want a1", text["index"])
	}
	if got := richTextPlainText(t, text); got != "Broken Title vertex/nano-banana · 1 image" {
		t.Fatalf("text = %q", got)
	}
	image := findChildByType(t, store, frameID, "c-image")
	if image["index"] != "a2" {
		t.Fatalf("image index = %v, want a2", image["index"])
	}

	sequences := decodeSchemaSequences(t, mutated.JSON)
	assertSchemaSequence(t, sequences, "com.tldraw.store", 5)
	assertSchemaSequence(t, sequences, "com.tldraw.shape.frame", 1)
	assertSchemaSequence(t, sequences, "com.tldraw.shape.text", 4)
	assertSchemaSequence(t, sequences, "com.tldraw.shape.c-image", 0)
}

func TestCanvasShapeIDsUseLovartLength(t *testing.T) {
	for i := 0; i < 20; i++ {
		id, err := newShapeID()
		if err != nil {
			t.Fatalf("newShapeID returned error: %v", err)
		}
		if !canonicalCanvasShapeID(id) {
			t.Fatalf("shape id %q is not canonical", id)
		}
	}
}

func TestCanvasIndexForPositionAvoidsUnsafeSequentialKeys(t *testing.T) {
	indices := indicesForPositions(13)
	want := []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9", "aA", "aB", "aC", "aD"}
	if !reflect.DeepEqual(indices, want) {
		t.Fatalf("indices = %#v, want %#v", indices, want)
	}
	sorted := append([]string(nil), indices...)
	sort.Strings(sorted)
	if !reflect.DeepEqual(sorted, indices) {
		t.Fatalf("indices do not sort in position order: sorted=%#v original=%#v", sorted, indices)
	}
	for _, index := range indices {
		if unsafeCanvasIndex(index) {
			t.Fatalf("index %q is unsafe", index)
		}
	}
}

func TestNormalizeCanvasJSONRepairsUnsafeSequentialIndexes(t *testing.T) {
	mutated, result, err := normalizeCanvasJSON(unsafeSequentialIndexCanvasJSON())
	if err != nil {
		t.Fatalf("normalizeCanvasJSON returned error: %v", err)
	}
	if result.NormalizedIndexKeys != 4 {
		t.Fatalf("normalized index keys = %d, want 4", result.NormalizedIndexKeys)
	}

	store := decodeStore(t, mutated.JSON)
	frame := findShapeByType(t, store, "frame")
	children := childShapeIndexes(t, store, frame["id"].(string))
	want := []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9", "aA", "aB", "aC", "aD"}
	if !reflect.DeepEqual(children, want) {
		t.Fatalf("child indexes = %#v, want %#v", children, want)
	}
}

func TestNormalizeCanvasJSONPreservesFutureSchemaSequences(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(syntheticCanvasJSON()), &doc); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	snapshot := doc["tldrawSnapshot"].(map[string]any)
	document := snapshot["document"].(map[string]any)
	document["schema"] = map[string]any{
		"schemaVersion": 3,
		"sequences": map[string]any{
			"com.tldraw.store":      99,
			"com.tldraw.shape.text": 1,
			"com.example.future":    7,
		},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	mutated, result, err := normalizeCanvasJSON(string(b))
	if err != nil {
		t.Fatalf("normalizeCanvasJSON returned error: %v", err)
	}
	if result.NormalizedSchemaSequences == 0 {
		t.Fatalf("normalized schema sequences = 0, want repairs")
	}

	var normalized map[string]any
	if err := json.Unmarshal([]byte(mutated.JSON), &normalized); err != nil {
		t.Fatalf("parse normalized canvas: %v", err)
	}
	normalizedSnapshot := normalized["tldrawSnapshot"].(map[string]any)
	normalizedDocument := normalizedSnapshot["document"].(map[string]any)
	schema := normalizedDocument["schema"].(map[string]any)
	if int(schema["schemaVersion"].(float64)) != 3 {
		t.Fatalf("schema version = %v, want 3", schema["schemaVersion"])
	}

	sequences := schema["sequences"].(map[string]any)
	assertSchemaSequence(t, sequences, "com.tldraw.store", 99)
	assertSchemaSequence(t, sequences, "com.tldraw.shape.text", 4)
	assertSchemaSequence(t, sequences, "com.example.future", 7)
	assertSchemaSequence(t, sequences, "com.tldraw.shape.frame", 1)
}
