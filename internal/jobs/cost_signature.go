package jobs

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// Pricing fields that affect credit cost — included in cost_signature.
var pricingFields = map[string]bool{
	"model":         true,
	"quality":       true,
	"size":          true,
	"resolution":    true,
	"aspect_ratio":  true,
	"mode":          true,
	"render_speed":  true,
	"outputs":       true,
	"n":             true,
	"max_images":    true,
	"num_images":    true,
}

// CostSignature computes a hash for pricing cache lookup.
// Jobs with identical signatures can reuse the same quote.
func CostSignature(job JobLine) string {
	parts := []string{}

	// Always include model.
	parts = append(parts, "model:"+job.Model)

	// Include price-affecting fields from body.
	sortedKeys := make([]string, 0, len(job.Body))
	for k := range job.Body {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, k := range sortedKeys {
		if !pricingFields[k] {
			continue
		}
		v := job.Body[k]
		// Count reference images separately.
		if k == "image" || k == "image_list" || k == "image_url" {
			switch arr := v.(type) {
			case []any:
				parts = append(parts, fmt.Sprintf("ref_images:%d", len(arr)))
			case string:
				if arr != "" {
					parts = append(parts, "ref_images:1")
				}
			}
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%v", k, v))
	}

	// Include outputs count.
	parts = append(parts, fmt.Sprintf("outputs:%d", job.Outputs))
	parts = append(parts, fmt.Sprintf("mode:%s", job.Mode))

	sig := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(sig))
	return fmt.Sprintf("%x", hash[:8])
}
