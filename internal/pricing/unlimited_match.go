package pricing

import (
	"fmt"
	"strings"
)

type requestFacts struct {
	quality    string
	resolution string
}

func matchingUnlimited(model string, facts requestFacts, status *unlimitedStatus) (unlimitedItem, bool) {
	if status == nil || !status.UnlimitedEnable {
		return unlimitedItem{}, false
	}
	for _, item := range status.UnlimitedList {
		if item.Status != 1 {
			continue
		}
		if !modelMatches(item, model) {
			continue
		}
		if !unlimitedExtraMatches(item.ExtraItem, facts) {
			continue
		}
		return item, true
	}
	return unlimitedItem{}, false
}

func modelMatches(item unlimitedItem, model string) bool {
	if strings.EqualFold(strings.TrimSpace(item.ModelDisplayName), strings.TrimSpace(model)) {
		return true
	}
	for _, alias := range item.AliasList {
		if strings.EqualFold(strings.TrimSpace(alias), strings.TrimSpace(model)) {
			return true
		}
	}
	return false
}

func unlimitedExtraMatches(extra *string, facts requestFacts) bool {
	if extra == nil || strings.TrimSpace(*extra) == "" {
		return true
	}
	extraText := normalizeText(*extra)

	extraQuality := qualityFromText(extraText)
	if extraQuality != "" {
		if facts.quality == "" || extraQuality != facts.quality {
			return false
		}
	}

	extraResolution := resolutionFromText(extraText)
	if extraResolution != "" {
		if facts.resolution == "" || !resolutionWithinLimit(facts.resolution, extraResolution) {
			return false
		}
	}

	return true
}

func requestFactsFor(detail PriceDetail, body map[string]any) requestFacts {
	args := make(map[string]any, len(body)+len(detail.InputArgs))
	for key, value := range body {
		args[key] = value
	}
	for key, value := range detail.InputArgs {
		args[key] = value
	}
	return requestFacts{
		quality:    qualityFromText(valueString(args["quality"])),
		resolution: resolutionFromArgs(args),
	}
}

func resolutionFromArgs(args map[string]any) string {
	for _, key := range []string{"resolution", "size"} {
		if value, ok := args[key]; ok {
			if resolution := resolutionFromText(valueString(value)); resolution != "" {
				return resolution
			}
		}
	}
	return ""
}

func qualityFromText(value string) string {
	text := normalizeText(value)
	for _, token := range []string{"low", "medium", "high"} {
		if strings.Contains(text, token) {
			return token
		}
	}
	return ""
}

func resolutionFromText(value string) string {
	text := normalizeText(value)
	if strings.Contains(text, "4k") {
		return "4k"
	}
	if strings.Contains(text, "2k") {
		return "2k"
	}
	if strings.Contains(text, "1k") {
		return "1k"
	}
	if strings.Contains(text, "3840") || strings.Contains(text, "2160") {
		return "4k"
	}
	if strings.Contains(text, "2048") || strings.Contains(text, "1152") {
		return "2k"
	}
	if strings.Contains(text, "1024") || strings.Contains(text, "1536") {
		return "1k"
	}
	if text == "512" || text == "512*512" {
		return "512"
	}
	return ""
}

func resolutionWithinLimit(requested string, limit string) bool {
	requestedRank := resolutionRank(requested)
	limitRank := resolutionRank(limit)
	if requestedRank == 0 || limitRank == 0 {
		return requested == limit
	}
	return requestedRank <= limitRank
}

func resolutionRank(resolution string) int {
	switch resolutionFromText(resolution) {
	case "512":
		return 1
	case "1k":
		return 2
	case "2k":
		return 3
	case "4k":
		return 4
	default:
		return 0
	}
}

func normalizeText(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	text = strings.ReplaceAll(text, " ", "")
	text = strings.ReplaceAll(text, "_", "")
	text = strings.ReplaceAll(text, "-", "")
	text = strings.ReplaceAll(text, "x", "*")
	text = strings.ReplaceAll(text, "×", "*")
	return text
}

func valueString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
