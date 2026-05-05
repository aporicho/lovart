package project

import (
	"math"

	"github.com/tidwall/gjson"
)

// computeLayoutGJSON determines where to place new nodes using gjson.
func computeLayoutGJSON(jsonStr, storePath string) (int, int) {
	maxRight := 0
	store := gjson.Get(jsonStr, storePath)
	if !store.Exists() {
		return 100, 100
	}

	store.ForEach(func(key, value gjson.Result) bool {
		x := value.Get("x").Float()
		w := value.Get("props.w").Float()
		right := int(x + w)
		if right > maxRight {
			maxRight = right
		}
		return true
	})

	if maxRight == 0 {
		return 100, 100
	}
	return maxRight + 64, 0
}

func countChildShapesGJSON(jsonStr, storePath, parentID string) int {
	count := 0
	store := gjson.Get(jsonStr, storePath)
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("typeName").String() == "shape" && value.Get("parentId").String() == parentID {
			count++
		}
		return true
	})
	return count
}

func indicesForNewSiblingsGJSON(jsonStr, storePath, parentID string, n int) []string {
	if n <= 0 {
		return nil
	}
	start := countChildShapesGJSON(jsonStr, storePath, parentID) + 1
	indices := make([]string, n)
	for i := range indices {
		indices[i] = canvasIndexForPosition(start + i)
	}
	return indices
}

func indicesForPositions(n int) []string {
	if n <= 0 {
		return nil
	}
	indices := make([]string, n)
	for i := range indices {
		indices[i] = canvasIndexForPosition(i + 1)
	}
	return indices
}

func computeSectionLayoutStartGJSON(jsonStr, storePath string, options CanvasLayoutOptions) (int, int) {
	minX := math.MaxInt
	maxBottom := math.MinInt
	hasShape := false
	store := gjson.Get(jsonStr, storePath)
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("typeName").String() != "shape" || value.Get("parentId").String() != "page:page" {
			return true
		}
		x := int(math.Round(value.Get("x").Float()))
		y := int(math.Round(value.Get("y").Float()))
		w := int(math.Round(value.Get("props.w").Float()))
		h := int(math.Round(value.Get("props.h").Float()))
		if w <= 0 && h <= 0 {
			return true
		}
		if x < minX {
			minX = x
		}
		if bottom := y + h; bottom > maxBottom {
			maxBottom = bottom
		}
		hasShape = true
		return true
	})
	if !hasShape {
		return options.Padding, options.Padding
	}
	return minX, maxBottom + options.FrameGap
}
