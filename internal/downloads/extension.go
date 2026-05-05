package downloads

import (
	"mime"
	"net/url"
	"path"
	"strings"
)

func extensionFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.TrimPrefix(path.Ext(parsed.Path), ".")
	return normalizeExt(ext)
}

func extensionFromContentType(contentType string) string {
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = contentType[:i]
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	switch contentType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "video/mp4":
		return "mp4"
	}
	exts, _ := mime.ExtensionsByType(contentType)
	if len(exts) == 0 {
		return ""
	}
	return normalizeExt(strings.TrimPrefix(exts[0], "."))
}

func normalizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return ""
	}
	for _, r := range ext {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	switch ext {
	case "jpeg", "jpe":
		return "jpg"
	default:
		return ext
	}
}
