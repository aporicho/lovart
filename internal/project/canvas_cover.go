package project

import (
	"sort"

	"github.com/tidwall/gjson"
)

func coverList(canvas string) ([]string, error) {
	jsonStr, err := decodeCanvasJSON(canvas)
	if err != nil {
		return nil, err
	}
	return extractCoverListGJSON(jsonStr), nil
}

// extractCoverListGJSON gets the 4 topmost c-image URLs for the cover list.
func extractCoverListGJSON(jsonStr string) []string {
	if jsonStr == "" {
		return nil
	}
	type coverImage struct {
		index string
		url   string
	}
	var images []coverImage
	store := gjson.Get(jsonStr, "tldrawSnapshot.document.store")
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("type").String() == "c-image" {
			if url := value.Get("props.url").String(); url != "" {
				images = append(images, coverImage{
					index: value.Get("index").String(),
					url:   url,
				})
			}
		}
		return true
	})

	sort.SliceStable(images, func(i, j int) bool {
		return images[i].index < images[j].index
	})

	start := 0
	if len(images) > 4 {
		start = len(images) - 4
	}
	urls := make([]string, 0, len(images)-start)
	for _, img := range images[start:] {
		urls = append(urls, img.url)
	}
	return urls
}
