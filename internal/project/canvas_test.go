package project

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
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

func TestCanvasEncodeDecodeRoundTrip(t *testing.T) {
	fixture := syntheticCanvasJSON()
	encoded, err := encodeCanvasJSON(fixture)
	if err != nil {
		t.Fatalf("encodeCanvasJSON returned error: %v", err)
	}
	decoded, err := decodeCanvasJSON(encoded)
	if err != nil {
		t.Fatalf("decodeCanvasJSON returned error: %v", err)
	}
	if decoded != fixture {
		t.Fatalf("decoded canvas does not match original")
	}
}

func TestDecodeCanvasJSONRejectsBadPrefix(t *testing.T) {
	if _, err := decodeCanvasJSON("not-shakker-data"); err == nil {
		t.Fatalf("decodeCanvasJSON accepted data without SHAKKERDATA prefix")
	}
}

func syntheticCanvasJSON() string {
	return `{"tldrawSnapshot":{"document":{"store":{"document:document":{"gridSize":10,"name":"","meta":{},"id":"document:document","typeName":"document"},"page:page":{"meta":{},"id":"page:page","name":"Page 1","index":"a1","typeName":"page"},"shape:oldOneShape0000000001":{"x":0,"y":0,"rotation":0,"isLocked":false,"opacity":1,"meta":{"source":"ai"},"id":"shape:oldOneShape0000000001","type":"c-image","props":{"w":100,"h":100,"url":"https://old/1.png","originalUrl":"https://old/1.png","radius":0,"name":" Image 1","genType":1,"generatorTaskId":"task-old"},"parentId":"page:page","index":"a1","typeName":"shape"},"shape:oldTwoShape0000000002":{"x":120,"y":0,"rotation":0,"isLocked":false,"opacity":1,"meta":{"source":"ai"},"id":"shape:oldTwoShape0000000002","type":"c-image","props":{"w":100,"h":100,"url":"https://old/2.png","originalUrl":"https://old/2.png","radius":0,"name":" Image 2","genType":1,"generatorTaskId":"task-old"},"parentId":"page:page","index":"a2","typeName":"shape"},"shape:oldTriShape0000000003":{"x":240,"y":0,"rotation":0,"isLocked":false,"opacity":1,"meta":{"source":"ai"},"id":"shape:oldTriShape0000000003","type":"c-image","props":{"w":100,"h":100,"url":"https://old/3.png","originalUrl":"https://old/3.png","radius":0,"name":" Image 3","genType":1,"generatorTaskId":"task-old"},"parentId":"page:page","index":"a3","typeName":"shape"},"shape:genrShapeId0000000001":{"x":360,"y":0,"rotation":0,"isLocked":false,"opacity":1,"meta":{"source":"ai"},"id":"shape:genrShapeId0000000001","type":"c-generator","props":{"w":100,"h":100,"name":"Image Generator"},"parentId":"page:page","index":"a4","typeName":"shape"}},"schema":{"schemaVersion":2,"sequences":{}}},"session":{"version":0,"currentPageId":"page:page","exportBackground":true,"isFocusMode":false,"isDebugMode":false,"isToolLocked":false,"isGridMode":false,"pageStates":[]}}}`
}

func corruptCanvasJSON() string {
	return `{"tldrawSnapshot":{"document":{"store":{"document:document":{"gridSize":10,"name":"","meta":{},"id":"document:document","typeName":"document"},"page:page":{"meta":{},"id":"page:page","name":"Page 1","index":"a1","typeName":"page"},"shape:frame":{"x":100,"y":100,"rotation":0,"isLocked":false,"opacity":1,"meta":{},"id":"shape:frame","type":"frame","props":{"w":1224,"h":1424,"name":"Broken Title","color":"black","isAutoLayout":true},"parentId":"page:page","index":"a000300020001","typeName":"shape"},"shape:text":{"x":100,"y":100,"rotation":0,"isLocked":false,"opacity":1,"meta":{},"id":"shape:text","type":"text","props":{"color":"black","size":"m","w":20,"font":"draw","textAlign":"start","autoSize":true,"scale":1,"richText":{"type":"doc","attrs":{"dir":"auto"},"content":[{"type":"paragraph","attrs":{"dir":"auto","textAlign":"left"},"content":[{"type":"text","marks":[{"type":"textStyle","attrs":{"fontFamily":"Inter","fontSize":"80px","color":"#000000","fontStyle":null,"fontWeight":"400","letterSpacing":null,"lineHeight":null,"textBoxTrim":null,"textBoxEdge":null,"textCase":null,"fillPaint":"{\"type\":\"SOLID\",\"color\":{\"r\":0,\"g\":0,\"b\":0,\"a\":1},\"opacity\":1,\"visible\":true,\"blendMode\":\"NORMAL\"}","stroke":"{\"type\":\"SOLID\",\"color\":{\"r\":0,\"g\":0,\"b\":0,\"a\":1},\"opacity\":1,\"visible\":false,\"blendMode\":\"NORMAL\"}"}}],"text":"Broken Title\nvertex/nano-banana · 1 image"}]}]}},"parentId":"shape:frame","index":"a0001","typeName":"shape"},"shape:image":{"x":100,"y":300,"rotation":0,"isLocked":false,"opacity":1,"meta":{"source":"ai"},"id":"shape:image","type":"c-image","props":{"w":1024,"h":1024,"url":"https://new/1.png","originalUrl":"https://new/1.png","radius":0,"name":" Image 1","genType":1,"generatorTaskId":"task"},"parentId":"shape:frame","index":"a000300020001","typeName":"shape"}},"schema":{"schemaVersion":2,"sequences":{}}},"session":{"version":0,"currentPageId":"page:page","exportBackground":true,"isFocusMode":false,"isDebugMode":false,"isToolLocked":false,"isGridMode":false,"pageStates":[]}}}`
}

func unsafeSequentialIndexCanvasJSON() string {
	var doc map[string]any
	if err := json.Unmarshal([]byte(defaultCanvasJSON()), &doc); err != nil {
		panic(err)
	}
	snapshot := doc["tldrawSnapshot"].(map[string]any)
	document := snapshot["document"].(map[string]any)
	store := document["store"].(map[string]any)

	frameID := "shape:frameShape00000000001"
	store[frameID] = map[string]any{
		"x":        0,
		"y":        0,
		"rotation": 0,
		"isLocked": false,
		"opacity":  1,
		"meta":     map[string]any{},
		"id":       frameID,
		"type":     "frame",
		"props": map[string]any{
			"w":            1500,
			"h":            1500,
			"name":         "Unsafe Indexes",
			"color":        "black",
			"isAutoLayout": true,
		},
		"parentId": "page:page",
		"index":    "a1",
		"typeName": "shape",
	}
	for i := 1; i <= 13; i++ {
		id := fmt.Sprintf("shape:childShape%011d", i)
		store[id] = map[string]any{
			"x":        float64(i * 100),
			"y":        300.0,
			"rotation": 0,
			"isLocked": false,
			"opacity":  1,
			"meta":     map[string]any{"source": "ai"},
			"id":       id,
			"type":     "c-image",
			"props": map[string]any{
				"w":               100,
				"h":               100,
				"url":             fmt.Sprintf("https://new/%d.png", i),
				"originalUrl":     fmt.Sprintf("https://new/%d.png", i),
				"radius":          0,
				"name":            fmt.Sprintf(" Image %d", i),
				"genType":         1,
				"generatorTaskId": "task",
			},
			"parentId": frameID,
			"index":    fmt.Sprintf("a%d", i),
			"typeName": "shape",
		}
	}

	b, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func decodeStore(t *testing.T, jsonStr string) map[string]any {
	t.Helper()

	var doc map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &doc); err != nil {
		t.Fatalf("parse canvas JSON: %v", err)
	}
	snapshot := doc["tldrawSnapshot"].(map[string]any)
	document := snapshot["document"].(map[string]any)
	return document["store"].(map[string]any)
}

func decodeSchemaSequences(t *testing.T, jsonStr string) map[string]any {
	t.Helper()

	var doc map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &doc); err != nil {
		t.Fatalf("parse canvas JSON: %v", err)
	}
	snapshot := doc["tldrawSnapshot"].(map[string]any)
	document := snapshot["document"].(map[string]any)
	schema := document["schema"].(map[string]any)
	return schema["sequences"].(map[string]any)
}

func assertSchemaSequence(t *testing.T, sequences map[string]any, key string, want int) {
	t.Helper()

	got, ok := intFromJSONValue(sequences[key])
	if !ok {
		t.Fatalf("schema sequence %s = %#v, want numeric %d", key, sequences[key], want)
	}
	if got != want {
		t.Fatalf("schema sequence %s = %d, want %d", key, got, want)
	}
}

func assertStoreKeysMatchIDs(t *testing.T, store map[string]any) {
	t.Helper()

	for key, raw := range store {
		record := raw.(map[string]any)
		id, _ := record["id"].(string)
		if id != "" && id != key {
			t.Fatalf("store key %q does not match record id %q", key, id)
		}
	}
}

func assertCanonicalShapeIDs(t *testing.T, store map[string]any) {
	t.Helper()

	for key, raw := range store {
		record := raw.(map[string]any)
		if record["typeName"] != "shape" {
			continue
		}
		if !canonicalCanvasShapeID(key) {
			t.Fatalf("shape key %q is not canonical", key)
		}
		if record["id"] != key {
			t.Fatalf("shape id = %v, want %s", record["id"], key)
		}
	}
}

func childShapeIndexes(t *testing.T, store map[string]any, parentID string) []string {
	t.Helper()

	var shapes []canvasShapeOrder
	for key, raw := range store {
		record := raw.(map[string]any)
		if record["typeName"] != "shape" || record["parentId"] != parentID {
			continue
		}
		shapes = append(shapes, canvasShapeOrder{
			id:    key,
			index: record["index"].(string),
			x:     record["x"].(float64),
			y:     record["y"].(float64),
		})
	}
	sort.SliceStable(shapes, func(i, j int) bool {
		if shapes[i].y != shapes[j].y {
			return shapes[i].y < shapes[j].y
		}
		if shapes[i].x != shapes[j].x {
			return shapes[i].x < shapes[j].x
		}
		return shapes[i].id < shapes[j].id
	})
	out := make([]string, 0, len(shapes))
	for _, shape := range shapes {
		out = append(out, shape.index)
	}
	return out
}

func findImageByURL(t *testing.T, store map[string]any, url string) map[string]any {
	t.Helper()

	for _, raw := range store {
		record := raw.(map[string]any)
		if record["type"] != "c-image" {
			continue
		}
		props := record["props"].(map[string]any)
		if props["url"] == url {
			return record
		}
	}
	t.Fatalf("image %q not found", url)
	return nil
}

func assertImageNode(t *testing.T, node map[string]any, taskID, name string, width, height int) {
	t.Helper()

	if node["type"] != "c-image" {
		t.Fatalf("node type = %v, want c-image", node["type"])
	}
	if node["typeName"] != "shape" {
		t.Fatalf("node typeName = %v, want shape", node["typeName"])
	}
	if node["parentId"] != "page:page" {
		t.Fatalf("node parentId = %v, want page:page", node["parentId"])
	}
	meta := node["meta"].(map[string]any)
	if meta["source"] != "ai" {
		t.Fatalf("node meta.source = %v, want ai", meta["source"])
	}

	props := node["props"].(map[string]any)
	if props["generatorTaskId"] != taskID {
		t.Fatalf("generatorTaskId = %v, want %s", props["generatorTaskId"], taskID)
	}
	if props["name"] != name {
		t.Fatalf("name = %v, want %s", props["name"], name)
	}
	if int(props["w"].(float64)) != width {
		t.Fatalf("width = %v, want %d", props["w"], width)
	}
	if int(props["h"].(float64)) != height {
		t.Fatalf("height = %v, want %d", props["h"], height)
	}
	if props["url"] != props["originalUrl"] {
		t.Fatalf("url and originalUrl differ: %v vs %v", props["url"], props["originalUrl"])
	}
}

func findShapeByType(t *testing.T, store map[string]any, shapeType string) map[string]any {
	t.Helper()

	for _, raw := range store {
		record := raw.(map[string]any)
		if record["type"] == shapeType {
			return record
		}
	}
	t.Fatalf("shape type %q not found", shapeType)
	return nil
}

func findChildByType(t *testing.T, store map[string]any, parentID, shapeType string) map[string]any {
	t.Helper()

	for _, raw := range store {
		record := raw.(map[string]any)
		if record["parentId"] == parentID && record["type"] == shapeType {
			return record
		}
	}
	t.Fatalf("child type %q under %q not found", shapeType, parentID)
	return nil
}

func richTextPlainText(t *testing.T, node map[string]any) string {
	t.Helper()

	props := node["props"].(map[string]any)
	richText := props["richText"].(map[string]any)
	content := richText["content"].([]any)
	paragraph := content[0].(map[string]any)
	spans := paragraph["content"].([]any)
	return spans[0].(map[string]any)["text"].(string)
}
