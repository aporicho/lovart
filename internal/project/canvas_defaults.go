package project

import "encoding/json"

func defaultCanvasJSON() string {
	canvas := map[string]any{
		"tldrawSnapshot": map[string]any{
			"document": map[string]any{
				"store": map[string]any{
					"document:document": map[string]any{
						"gridSize": 10,
						"name":     "",
						"meta":     map[string]any{},
						"id":       "document:document",
						"typeName": "document",
					},
					"page:page": map[string]any{
						"meta":     map[string]any{},
						"id":       "page:page",
						"name":     "Page 1",
						"index":    "a1",
						"typeName": "page",
					},
				},
				"schema": map[string]any{
					"schemaVersion": 2,
					"sequences":     lovartCanvasSchemaSequences(),
				},
			},
			"session": map[string]any{
				"version":          0,
				"currentPageId":    "page:page",
				"exportBackground": true,
				"isFocusMode":      false,
				"isDebugMode":      false,
				"isToolLocked":     false,
				"isGridMode":       false,
				"pageStates":       []any{},
			},
		},
	}
	b, err := json.Marshal(canvas)
	if err != nil {
		panic(err)
	}
	return string(b)
}
