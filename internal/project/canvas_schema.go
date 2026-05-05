package project

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const canvasSchemaPath = "tldrawSnapshot.document.schema"

func lovartCanvasSchemaSequences() map[string]int {
	return map[string]int{
		"com.tldraw.asset":                   1,
		"com.tldraw.asset.bookmark":          2,
		"com.tldraw.asset.image":             6,
		"com.tldraw.asset.video":             5,
		"com.tldraw.binding.arrow":           1,
		"com.tldraw.camera":                  1,
		"com.tldraw.document":                2,
		"com.tldraw.instance":                26,
		"com.tldraw.instance_page_state":     5,
		"com.tldraw.instance_presence":       6,
		"com.tldraw.page":                    1,
		"com.tldraw.pointer":                 1,
		"com.tldraw.shape":                   4,
		"com.tldraw.shape.arrow":             8,
		"com.tldraw.shape.c-3d":              0,
		"com.tldraw.shape.c-audio":           0,
		"com.tldraw.shape.c-color":           0,
		"com.tldraw.shape.c-context-widget":  0,
		"com.tldraw.shape.c-decal":           0,
		"com.tldraw.shape.c-draw":            0,
		"com.tldraw.shape.c-ellipse":         0,
		"com.tldraw.shape.c-generator":       0,
		"com.tldraw.shape.c-html-widget":     0,
		"com.tldraw.shape.c-image":           0,
		"com.tldraw.shape.c-mockup":          0,
		"com.tldraw.shape.c-pdf":             0,
		"com.tldraw.shape.c-polygon":         0,
		"com.tldraw.shape.c-rectangle":       0,
		"com.tldraw.shape.c-star":            0,
		"com.tldraw.shape.c-style":           0,
		"com.tldraw.shape.c-svg":             0,
		"com.tldraw.shape.c-task":            0,
		"com.tldraw.shape.c-vector":          0,
		"com.tldraw.shape.c-video":           0,
		"com.tldraw.shape.c-video-generator": 0,
		"com.tldraw.shape.canvas-text":       0,
		"com.tldraw.shape.draw":              4,
		"com.tldraw.shape.frame":             1,
		"com.tldraw.shape.geo":               11,
		"com.tldraw.shape.group":             0,
		"com.tldraw.shape.line":              5,
		"com.tldraw.shape.text":              4,
		"com.tldraw.store":                   5,
	}
}

func ensureCanvasSchemaSequences(jsonStr string, result *CanvasRepairResult) (string, error) {
	schema := map[string]any{}
	changed := false
	schemaResult := gjson.Get(jsonStr, canvasSchemaPath)
	if schemaResult.Exists() {
		if err := json.Unmarshal([]byte(schemaResult.Raw), &schema); err != nil {
			changed = true
		}
	} else {
		changed = true
	}
	if schema == nil {
		schema = map[string]any{}
		changed = true
	}

	if version, ok := intFromJSONValue(schema["schemaVersion"]); !ok || version < 2 {
		schema["schemaVersion"] = 2
		changed = true
	}

	sequences, ok := schema["sequences"].(map[string]any)
	if !ok {
		sequences = map[string]any{}
		schema["sequences"] = sequences
		changed = true
	}
	for key, baseline := range lovartCanvasSchemaSequences() {
		current, ok := intFromJSONValue(sequences[key])
		if ok && current >= baseline {
			continue
		}
		sequences[key] = baseline
		changed = true
		result.NormalizedSchemaSequences++
	}

	if !changed {
		return jsonStr, nil
	}
	updatedSchema, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("marshal canvas schema: %w", err)
	}
	updated, err := sjson.SetRaw(jsonStr, canvasSchemaPath, string(updatedSchema))
	if err != nil {
		return "", fmt.Errorf("write canvas schema: %w", err)
	}
	result.Changed = true
	return updated, nil
}

func intFromJSONValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		return int(typed), true
	case float64:
		asInt := int(typed)
		return asInt, float64(asInt) == typed
	case float32:
		asInt := int(typed)
		return asInt, float32(asInt) == typed
	case json.Number:
		asInt, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(asInt), true
	default:
		return 0, false
	}
}
