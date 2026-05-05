package downloads

import (
	"context"
	"fmt"
	"io"
	nethttp "net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/aporicho/lovart/internal/paths"
)

// DownloadArtifacts downloads artifacts, embeds effect metadata into supported
// image formats, and appends a runtime index under the download root.
func DownloadArtifacts(ctx context.Context, artifacts []Artifact, opts Options) (*Result, error) {
	root := opts.RootDir
	if root == "" {
		root = paths.DownloadsDir
	}
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("downloads: create root: %w", err)
	}

	client := opts.HTTPClient
	if client == nil {
		client = &nethttp.Client{Timeout: 5 * time.Minute}
	}

	result := &Result{RootDir: root}
	for i, artifact := range artifacts {
		if artifact.Index <= 0 {
			artifact.Index = i + 1
		}
		result.Files = append(result.Files, downloadOne(ctx, client, root, artifact, opts))
	}
	if err := appendIndex(root, result, opts); err != nil {
		result.IndexError = err.Error()
	}
	return result, nil
}

func downloadOne(ctx context.Context, client *nethttp.Client, root string, artifact Artifact, opts Options) FileResult {
	file := FileResult{
		ArtifactIndex: artifact.Index,
		URL:           artifact.URL,
	}
	if err := validateArtifactURL(artifact.URL); err != nil {
		file.Error = err.Error()
		return file
	}

	urlExt := extensionFromURL(artifact.URL)
	ext := urlExt
	if ext == "" {
		ext = "bin"
	}
	dir, filename, err := resolvePath(root, opts, artifact, ext)
	if err != nil {
		file.Error = err.Error()
		return file
	}
	file.Directory = dir
	file.Filename = filename
	file.Path = filepath.Join(dir, filename)

	if urlExt != "" && !opts.Overwrite {
		if stat, ok := existingFile(file.Path); ok {
			file.Bytes = stat.Size()
			file.Existing = true
			return file
		}
	}

	resp, err := getArtifact(ctx, client, artifact.URL)
	if err != nil {
		file.Error = err.Error()
		return file
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	file.ContentType = contentType
	if urlExt == "" {
		if ctExt := extensionFromContentType(contentType); ctExt != "" {
			ext = ctExt
			dir, filename, err = resolvePath(root, opts, artifact, ext)
			if err != nil {
				file.Error = err.Error()
				return file
			}
			file.Directory = dir
			file.Filename = filename
			file.Path = filepath.Join(dir, filename)
		}
	}

	if !opts.Overwrite {
		if stat, ok := existingFile(file.Path); ok {
			file.Bytes = stat.Size()
			file.Existing = true
			return file
		}
	}

	if err := os.MkdirAll(file.Directory, 0755); err != nil {
		file.Error = fmt.Sprintf("downloads: create directory: %v", err)
		return file
	}
	bytes, err := writeResponseAtomic(file.Path, resp.Body)
	if err != nil {
		file.Error = err.Error()
		return file
	}
	file.Bytes = bytes

	format, embedded, err := embedEffectMetadata(file.Path, buildEffectMetadata(opts.Context, artifact))
	file.MetadataFormat = format
	file.EmbeddedMetadata = embedded
	if err != nil {
		file.MetadataError = err.Error()
	}
	return file
}

func validateArtifactURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("downloads: artifact URL is empty")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("downloads: parse artifact URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("downloads: unsupported artifact URL scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("downloads: artifact URL host is empty")
	}
	return nil
}

func getArtifact(ctx context.Context, client *nethttp.Client, rawURL string) (*nethttp.Response, error) {
	req, err := nethttp.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("downloads: create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloads: GET artifact: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("downloads: GET artifact returned %d", resp.StatusCode)
	}
	return resp, nil
}

func existingFile(path string) (os.FileInfo, bool) {
	stat, err := os.Stat(path)
	if err != nil || stat.IsDir() || stat.Size() == 0 {
		return nil, false
	}
	return stat, true
}

func writeResponseAtomic(path string, body io.Reader) (int64, error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return 0, fmt.Errorf("downloads: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	bytes, err := io.Copy(tmp, body)
	if err != nil {
		tmp.Close()
		return 0, fmt.Errorf("downloads: write artifact: %w", err)
	}
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		return 0, fmt.Errorf("downloads: chmod artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("downloads: close artifact: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return 0, fmt.Errorf("downloads: replace artifact: %w", err)
	}
	return bytes, nil
}
