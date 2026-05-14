package project

import (
	"math"
	"strings"

	"github.com/tidwall/gjson"
)

// computeLayoutGJSON determines where to place new top-level image nodes.
func computeLayoutGJSON(jsonStr, storePath string) (int, int) {
	minX := math.MaxInt
	maxBottom := math.MinInt
	hasShape := false
	store := gjson.Get(jsonStr, storePath)
	if !store.Exists() {
		return defaultCanvasPadding, defaultCanvasPadding
	}

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
		return defaultCanvasPadding, defaultCanvasPadding
	}
	return minX, maxBottom + defaultCanvasFrameGap
}

func indicesForNewSiblingsGJSON(jsonStr, storePath, parentID string, n int) []string {
	if n <= 0 {
		return nil
	}
	highest := highestChildShapeIndexGJSON(jsonStr, storePath, parentID)
	if highest == "" {
		return indicesForPositions(n)
	}
	return canvasIndicesAbove(highest, n)
}

func highestChildShapeIndexGJSON(jsonStr, storePath, parentID string) string {
	highest := ""
	store := gjson.Get(jsonStr, storePath)
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("typeName").String() != "shape" || value.Get("parentId").String() != parentID {
			return true
		}
		index := value.Get("index").String()
		if index > highest {
			highest = index
		}
		return true
	})
	return highest
}

func canvasIndicesAbove(index string, n int) []string {
	indices := make([]string, n)
	next := index
	for i := range indices {
		next = canvasIndexAbove(next)
		indices[i] = next
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

const (
	canvasIndexPrefixAlphabet = "abcdefghijklmnopqrstuvwxyz"
	canvasIndexSuffixAlphabet = "123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

func canvasIndexAbove(index string) string {
	if index == "" {
		return canvasIndexForPosition(1)
	}
	if len(index) == 1 && strings.ContainsRune(canvasIndexPrefixAlphabet, rune(index[0])) {
		return index + "1"
	}
	for i := len(index) - 1; i >= 1; i-- {
		pos := strings.IndexByte(canvasIndexSuffixAlphabet, index[i])
		if pos < 0 {
			break
		}
		if pos+1 < len(canvasIndexSuffixAlphabet) {
			return index[:i] + string(canvasIndexSuffixAlphabet[pos+1])
		}
	}
	return index + "V"
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
