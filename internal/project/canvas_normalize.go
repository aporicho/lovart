package project

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type canvasShapeOrder struct {
	id    string
	index string
	x     float64
	y     float64
}

func normalizeCanvasJSON(jsonStr string) (*canvasMutation, CanvasRepairResult, error) {
	result := CanvasRepairResult{}
	if strings.TrimSpace(jsonStr) == "" {
		jsonStr = defaultCanvasJSON()
		result.Changed = true
	}
	if !json.Valid([]byte(jsonStr)) {
		return nil, CanvasRepairResult{}, fmt.Errorf("invalid canvas JSON")
	}

	var err error
	jsonStr, err = ensureCanvasStore(jsonStr)
	if err != nil {
		return nil, CanvasRepairResult{}, err
	}

	jsonStr, err = ensureBaseCanvasRecords(jsonStr, &result)
	if err != nil {
		return nil, CanvasRepairResult{}, err
	}
	jsonStr, err = normalizeCanvasTextNodes(jsonStr, &result)
	if err != nil {
		return nil, CanvasRepairResult{}, err
	}
	jsonStr, err = normalizeCanvasIndexes(jsonStr, &result)
	if err != nil {
		return nil, CanvasRepairResult{}, err
	}

	mutated := &canvasMutation{
		JSON:      jsonStr,
		PicCount:  countCImagesGJSON(jsonStr, canvasStorePath),
		CoverList: extractCoverListGJSON(jsonStr),
	}
	result.PicCount = mutated.PicCount
	result.CoverList = mutated.CoverList
	return mutated, result, nil
}

func ensureBaseCanvasRecords(jsonStr string, result *CanvasRepairResult) (string, error) {
	defaultStore := gjson.Get(defaultCanvasJSON(), canvasStorePath)
	for _, key := range []string{"document:document", "page:page"} {
		if gjson.Get(jsonStr, canvasStorePath+"."+key).Exists() {
			continue
		}
		record := defaultStore.Get(key).Raw
		var err error
		jsonStr, err = sjson.SetRaw(jsonStr, canvasStorePath+"."+key, record)
		if err != nil {
			return "", fmt.Errorf("normalize base record %s: %w", key, err)
		}
		result.Changed = true
	}

	sessionPath := "tldrawSnapshot.session"
	if !gjson.Get(jsonStr, sessionPath).Exists() {
		defaultSession := gjson.Get(defaultCanvasJSON(), sessionPath).Raw
		var err error
		jsonStr, err = sjson.SetRaw(jsonStr, sessionPath, defaultSession)
		if err != nil {
			return "", fmt.Errorf("normalize session: %w", err)
		}
		result.Changed = true
	}
	if gjson.Get(jsonStr, sessionPath+".currentPageId").String() == "" {
		var err error
		jsonStr, err = sjson.Set(jsonStr, sessionPath+".currentPageId", "page:page")
		if err != nil {
			return "", fmt.Errorf("normalize current page: %w", err)
		}
		result.Changed = true
	}
	return jsonStr, nil
}

func normalizeCanvasTextNodes(jsonStr string, result *CanvasRepairResult) (string, error) {
	store := gjson.Get(jsonStr, canvasStorePath)
	var textIDs []string
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("type").String() == "text" {
			textIDs = append(textIDs, key.String())
		}
		return true
	})

	for _, id := range textIDs {
		richTextPath := canvasStorePath + "." + id + ".props.richText"
		raw := gjson.Get(jsonStr, richTextPath).Raw
		if raw == "" {
			continue
		}

		var richText any
		if err := json.Unmarshal([]byte(raw), &richText); err != nil {
			return "", fmt.Errorf("normalize text %s: parse rich text: %w", id, err)
		}
		if !normalizeRichTextValue(richText) {
			continue
		}

		normalized, err := json.Marshal(richText)
		if err != nil {
			return "", fmt.Errorf("normalize text %s: marshal rich text: %w", id, err)
		}
		jsonStr, err = sjson.SetRaw(jsonStr, richTextPath, string(normalized))
		if err != nil {
			return "", fmt.Errorf("normalize text %s: %w", id, err)
		}
		result.Changed = true
		result.NormalizedTexts++
	}
	return jsonStr, nil
}

func normalizeRichTextValue(value any) bool {
	changed := false
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if key == "text" {
				if text, ok := item.(string); ok {
					normalized := sanitizeCanvasLabel(text)
					if normalized != text {
						typed[key] = normalized
						changed = true
					}
				}
				continue
			}
			if normalizeRichTextValue(item) {
				changed = true
			}
		}
	case []any:
		for _, item := range typed {
			if normalizeRichTextValue(item) {
				changed = true
			}
		}
	}
	return changed
}

func normalizeCanvasIndexes(jsonStr string, result *CanvasRepairResult) (string, error) {
	groups := map[string][]canvasShapeOrder{}
	store := gjson.Get(jsonStr, canvasStorePath)
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("typeName").String() != "shape" {
			return true
		}
		parent := value.Get("parentId").String()
		if parent == "" {
			return true
		}
		groups[parent] = append(groups[parent], canvasShapeOrder{
			id:    key.String(),
			index: value.Get("index").String(),
			x:     value.Get("x").Float(),
			y:     value.Get("y").Float(),
		})
		return true
	})

	parents := make([]string, 0, len(groups))
	for parent := range groups {
		parents = append(parents, parent)
	}
	sort.Strings(parents)

	for _, parent := range parents {
		shapes := groups[parent]
		sort.SliceStable(shapes, func(i, j int) bool {
			if shapes[i].y != shapes[j].y {
				return shapes[i].y < shapes[j].y
			}
			if shapes[i].x != shapes[j].x {
				return shapes[i].x < shapes[j].x
			}
			if shapes[i].index != shapes[j].index {
				return shapes[i].index < shapes[j].index
			}
			return shapes[i].id < shapes[j].id
		})
		for i, shape := range shapes {
			next := canvasIndexForPosition(i + 1)
			if shape.index == next {
				continue
			}
			var err error
			jsonStr, err = sjson.Set(jsonStr, canvasStorePath+"."+shape.id+".index", next)
			if err != nil {
				return "", fmt.Errorf("normalize index %s: %w", shape.id, err)
			}
			result.Changed = true
			result.NormalizedIndexes++
		}
	}
	return jsonStr, nil
}

func sanitizeCanvasLabel(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func canvasIndexForPosition(position int) string {
	if position <= 0 {
		position = 1
	}
	return fmt.Sprintf("a%d", position)
}
