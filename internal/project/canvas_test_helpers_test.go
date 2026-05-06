package project

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"
)

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
