package project

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func addImagesToCanvasJSON(jsonStr string, images []CanvasImage) (*canvasMutation, error) {
	if jsonStr == "" {
		jsonStr = defaultCanvasJSON()
	}
	if !json.Valid([]byte(jsonStr)) {
		return nil, fmt.Errorf("invalid canvas JSON")
	}
	var err error
	jsonStr, err = ensureCanvasStore(jsonStr)
	if err != nil {
		return nil, err
	}

	startX, startY := computeLayoutGJSON(jsonStr, canvasStorePath)
	imageCount := countCImagesGJSON(jsonStr, canvasStorePath)
	indices := indicesAfter(maxShapeIndexGJSON(jsonStr, canvasStorePath), len(images))

	const columns = 4
	const gap = 64
	for i, img := range images {
		col := i % columns
		row := i / columns
		x := startX + col*(img.Width+gap)
		y := startY + row*(img.Height+gap)

		idPart, err := randomString(22)
		if err != nil {
			return nil, fmt.Errorf("shape id: %w", err)
		}
		id := "shape:" + idPart
		name := fmt.Sprintf(" Image %d", imageCount+i+1)

		nodeJSON, err := buildNodeJSON(img, id, indices[i], name, x, y)
		if err != nil {
			return nil, fmt.Errorf("build node: %w", err)
		}
		jsonStr, err = sjson.SetRaw(jsonStr, canvasStorePath+"."+id, nodeJSON)
		if err != nil {
			return nil, fmt.Errorf("insert node: %w", err)
		}
	}

	return &canvasMutation{
		JSON:      jsonStr,
		PicCount:  countCImagesGJSON(jsonStr, canvasStorePath),
		CoverList: extractCoverListGJSON(jsonStr),
	}, nil
}

// buildNodeJSON constructs a tldraw c-image shape node as a JSON string.
func buildNodeJSON(img CanvasImage, id, index, name string, x, y int) (string, error) {
	return buildImageNodeJSON(img, id, index, name, "page:page", x, y, img.Width, img.Height)
}

// countCImagesGJSON counts c-image nodes in the store.
func countCImagesGJSON(jsonStr, storePath string) int {
	n := 0
	store := gjson.Get(jsonStr, storePath)
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("type").String() == "c-image" {
			n++
		}
		return true
	})
	return n
}
