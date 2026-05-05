package update

import (
	"context"
	"fmt"
	"io"
	nethttp "net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/signing"
)

const maxFetchBytes = 32 << 20

var (
	staticJSRe = regexp.MustCompile(`(?:https?:)?//[^"')<> ]+lovart_canvas_online/static/js/[^"')<> ]+\.js|/?lovart_canvas_online/static/js/[^"')<> ]+\.js`)
	wasmRe     = regexp.MustCompile(`(?:https?:)?//[^"')<> ]+(?:lovart_canvas_online/static/|lovart_prd/static/)[^"')<> ]+\.wasm|/?(?:lovart_canvas_online/static/|lovart_prd/static/)[^"')<> ]+\.wasm|\bstatic/[^"')<> ]+\.wasm`)
	releaseRe  = regexp.MustCompile(`(?:SENTRY_RELEASE\s*[:=]\s*\{?\s*id|release)\s*[:=]\s*['"]([^'"]+)['"]`)
)

func (s *Service) discoverSignerCandidate(ctx context.Context) (*signerCandidate, error) {
	canvasURL := s.canvasURL()
	html, err := s.fetchText(ctx, canvasURL)
	if err != nil {
		return nil, fmt.Errorf("update: fetch canvas html: %w", err)
	}

	jsURLs := uniqueAbsoluteURLs(canvasURL, staticJSRe.FindAllString(html, -1))
	wasmURLs := uniqueAbsoluteURLs(canvasURL, wasmRe.FindAllString(html, -1))
	var jsTexts []string
	for _, jsURL := range jsURLs {
		text, err := s.fetchText(ctx, jsURL)
		if err != nil {
			continue
		}
		jsTexts = append(jsTexts, text)
		wasmURLs = append(wasmURLs, uniqueAbsoluteURLs(jsURL, wasmRe.FindAllString(text, -1))...)
	}
	wasmURLs = uniqueStrings(wasmURLs)
	if len(wasmURLs) == 0 {
		return nil, fmt.Errorf("update: no signer wasm URL found in frontend assets")
	}

	var invalid []string
	for _, wasmURL := range wasmURLs {
		data, err := s.fetchBytes(ctx, wasmURL)
		if err != nil {
			invalid = append(invalid, fmt.Sprintf("%s: %v", wasmURL, err))
			continue
		}
		if err := signing.ValidateWASMBytes(ctx, data); err != nil {
			invalid = append(invalid, fmt.Sprintf("%s: %v", wasmURL, err))
			continue
		}
		return &signerCandidate{
			URL:             wasmURL,
			Bytes:           data,
			SHA256:          metadata.HashBytes(data),
			CanvasHTMLHash:  metadata.HashBytes([]byte(strings.Join(staticJSRe.FindAllString(html, -1), "\n"))),
			StaticJSHash:    metadata.HashBytes([]byte(strings.Join(jsURLs, "\n"))),
			SentryReleaseID: sentryRelease(jsTexts),
		}, nil
	}
	return nil, fmt.Errorf("update: no valid signer wasm found (%s)", strings.Join(invalid, "; "))
}

func (s *Service) fetchText(ctx context.Context, value string) (string, error) {
	data, err := s.fetchBytes(ctx, value)
	return string(data), err
}

func (s *Service) fetchBytes(ctx context.Context, value string) ([]byte, error) {
	req, err := nethttp.NewRequestWithContext(ctx, "GET", value, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned %d", value, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFetchBytes {
		return nil, fmt.Errorf("response too large")
	}
	return data, nil
}

func (s *Service) httpClient() HTTPDoer {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &nethttp.Client{Timeout: 30 * time.Second}
}

func (s *Service) canvasURL() string {
	if s.CanvasURL != "" {
		return s.CanvasURL
	}
	return defaultCanvasURL
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func uniqueAbsoluteURLs(base string, values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		abs, err := absoluteAssetURL(base, value)
		if err == nil && abs != "" {
			out = append(out, abs)
		}
	}
	return uniqueStrings(out)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func absoluteAssetURL(base, value string) (string, error) {
	if strings.HasPrefix(value, "//") {
		return "https:" + value, nil
	}
	if strings.HasPrefix(value, "http://") {
		return "", fmt.Errorf("insecure asset url %s", value)
	}
	if strings.HasPrefix(value, "https://") {
		return value, nil
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	refValue := "/" + strings.TrimLeft(value, "/")
	if strings.HasPrefix(value, "static/") {
		if suffix := pathAfterStaticJS(baseURL.Path); suffix != "" {
			prefix, _ := strings.CutSuffix(baseURL.Path, suffix)
			baseURL.Path = prefix
			refValue = value
		}
	}
	ref, err := url.Parse(refValue)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(ref).String(), nil
}

func pathAfterStaticJS(value string) string {
	const marker = "/static/js/"
	idx := strings.Index(value, marker)
	if idx < 0 {
		return ""
	}
	return value[idx+1:]
}

func sentryRelease(texts []string) string {
	for _, text := range texts {
		if match := releaseRe.FindStringSubmatch(text); len(match) == 2 {
			return match[1]
		}
	}
	return ""
}
