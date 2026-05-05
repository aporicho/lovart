package project

import (
	"fmt"

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

func maxShapeIndexGJSON(jsonStr, storePath string) string {
	maxIndex := ""
	store := gjson.Get(jsonStr, storePath)
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("typeName").String() != "shape" {
			return true
		}
		if parent := value.Get("parentId").String(); parent != "" && parent != "page:page" {
			return true
		}
		index := value.Get("index").String()
		if index > maxIndex {
			maxIndex = index
		}
		return true
	})
	return maxIndex
}

func indicesAfter(maxIndex string, n int) []string {
	if n <= 0 {
		return nil
	}
	indices := make([]string, n)
	if maxIndex == "" {
		for i := range indices {
			indices[i] = fmt.Sprintf("a%04d", i+1)
		}
		return indices
	}
	for i := range indices {
		indices[i] = fmt.Sprintf("%s%04d", maxIndex, i+1)
	}
	return indices
}
