package project

import (
	"github.com/tidwall/gjson"
)

func coverList(canvas string) ([]string, error) {
	jsonStr, err := decodeCanvasJSON(canvas)
	if err != nil {
		return nil, err
	}
	return extractCoverListGJSON(jsonStr), nil
}

// extractCoverListGJSON gets the 4 newest c-image URLs for the cover list.
func extractCoverListGJSON(jsonStr string) []string {
	if jsonStr == "" {
		return nil
	}
	var images []string
	store := gjson.Get(jsonStr, "tldrawSnapshot.document.store")
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("type").String() == "c-image" {
			if url := value.Get("props.url").String(); url != "" {
				images = append(images, url)
			}
		}
		return true
	})

	start := 0
	if len(images) > 4 {
		start = len(images) - 4
	}
	return append([]string(nil), images[start:]...)
}
