package downloads

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxPathSegmentRunes = 120

func sanitizeSegment(input string) string {
	input = strings.TrimSpace(input)
	var b strings.Builder
	lastSpace := false
	for _, r := range input {
		if invalidPathRune(r) {
			if !lastSpace {
				b.WriteByte('_')
				lastSpace = false
			}
			continue
		}
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	out := strings.Trim(b.String(), " ._")
	if out == "" {
		return ""
	}
	if utf8.RuneCountInString(out) <= maxPathSegmentRunes {
		return out
	}
	runes := []rune(out)
	return string(runes[:maxPathSegmentRunes])
}

func invalidPathRune(r rune) bool {
	if r < 32 || r == 127 {
		return true
	}
	switch r {
	case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
		return true
	default:
		return false
	}
}

func safeFilename(input, fallback string) string {
	out := sanitizeSegment(input)
	if out == "" {
		return fallback
	}
	return out
}
