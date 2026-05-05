package project

import "encoding/json"

func buildImageNodeJSON(img CanvasImage, id, index, name, parentID string, x, y, width, height int) (string, error) {
	node := map[string]any{
		"x":        float64(x),
		"y":        float64(y),
		"rotation": 0,
		"isLocked": false,
		"opacity":  1,
		"meta":     map[string]any{"source": "ai"},
		"id":       id,
		"type":     "c-image",
		"props": map[string]any{
			"w":               width,
			"h":               height,
			"url":             img.URL,
			"originalUrl":     img.URL,
			"radius":          0,
			"name":            name,
			"genType":         1,
			"generatorTaskId": img.TaskID,
		},
		"parentId": parentID,
		"index":    index,
		"typeName": "shape",
	}

	b, err := json.Marshal(node)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func buildFrameNodeJSON(id, index, name string, x, y, width, height int) (string, error) {
	node := map[string]any{
		"x":        float64(x),
		"y":        float64(y),
		"rotation": 0,
		"isLocked": false,
		"opacity":  1,
		"meta":     map[string]any{},
		"id":       id,
		"type":     "frame",
		"props": map[string]any{
			"w":            width,
			"h":            height,
			"name":         name,
			"color":        "black",
			"isAutoLayout": true,
		},
		"parentId": "page:page",
		"index":    index,
		"typeName": "shape",
	}

	b, err := json.Marshal(node)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func buildTextNodeJSON(id, parentID, index, text string, x, y int) (string, error) {
	node := map[string]any{
		"x":        float64(x),
		"y":        float64(y),
		"rotation": 0,
		"isLocked": false,
		"opacity":  1,
		"meta":     map[string]any{},
		"id":       id,
		"type":     "text",
		"props": map[string]any{
			"color":     "black",
			"size":      "m",
			"w":         20,
			"font":      "draw",
			"textAlign": "start",
			"autoSize":  true,
			"scale":     1,
			"richText":  richTextDoc(text),
		},
		"parentId": parentID,
		"index":    index,
		"typeName": "shape",
	}

	b, err := json.Marshal(node)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func richTextDoc(text string) map[string]any {
	return map[string]any{
		"type": "doc",
		"attrs": map[string]any{
			"dir": "auto",
		},
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"attrs": map[string]any{
					"dir":       "auto",
					"textAlign": "left",
				},
				"content": []any{
					map[string]any{
						"type": "text",
						"marks": []any{
							map[string]any{
								"type": "textStyle",
								"attrs": map[string]any{
									"fontFamily":    "Inter",
									"fontSize":      "80px",
									"color":         "#000000",
									"fontStyle":     nil,
									"fontWeight":    "400",
									"letterSpacing": nil,
									"lineHeight":    nil,
									"textBoxTrim":   nil,
									"textBoxEdge":   nil,
									"textCase":      nil,
									"fillPaint":     "{\"type\":\"SOLID\",\"color\":{\"r\":0,\"g\":0,\"b\":0,\"a\":1},\"opacity\":1,\"visible\":true,\"blendMode\":\"NORMAL\"}",
									"stroke":        "{\"type\":\"SOLID\",\"color\":{\"r\":0,\"g\":0,\"b\":0,\"a\":1},\"opacity\":1,\"visible\":false,\"blendMode\":\"NORMAL\"}",
								},
							},
						},
						"text": text,
					},
				},
			},
		},
	}
}
