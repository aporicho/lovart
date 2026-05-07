// Package selfmanage implements Lovart binary upgrade and uninstall helpers.
package selfmanage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	lovarterrors "github.com/aporicho/lovart/internal/errors"
)

const (
	DefaultRepo      = "aporicho/lovart"
	defaultAPIBase   = "https://api.github.com"
	defaultUserAgent = "lovart-upgrade"
)

// ReleaseAsset describes one downloadable release artifact.
type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
}

// Release describes the GitHub release selected for an upgrade.
type Release struct {
	TagName string
	Assets  map[string]ReleaseAsset
}

type releasePayload struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type releaseClient struct {
	httpClient *http.Client
	apiBase    string
	token      string
}

func newReleaseClient(httpClient *http.Client, apiBase string) releaseClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	if strings.TrimSpace(apiBase) == "" {
		apiBase = defaultAPIBase
	}
	return releaseClient{httpClient: httpClient, apiBase: strings.TrimRight(apiBase, "/"), token: os.Getenv("GITHUB_TOKEN")}
}

func (c releaseClient) fetch(ctx context.Context, repo string, version string) (Release, error) {
	if strings.TrimSpace(repo) == "" {
		repo = DefaultRepo
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Release{}, inputError("invalid GitHub repo", map[string]any{"repo": repo, "expected": "OWNER/REPO"})
	}
	if strings.TrimSpace(version) == "" {
		version = "latest"
	}

	endpoint := c.apiBase + "/repos/" + parts[0] + "/" + parts[1] + "/releases/latest"
	if version != "latest" {
		endpoint = c.apiBase + "/repos/" + parts[0] + "/" + parts[1] + "/releases/tags/" + url.PathEscape(version)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Release{}, internalError("build GitHub release request", map[string]any{"error": err.Error()})
	}
	c.addHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Release{}, networkError("fetch GitHub release", map[string]any{"error": err.Error(), "repo": repo, "version": version})
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Release{}, networkError("fetch GitHub release failed", map[string]any{
			"status":  resp.Status,
			"repo":    repo,
			"version": version,
			"body":    string(body),
		})
	}
	var payload releasePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Release{}, networkError("parse GitHub release", map[string]any{"error": err.Error()})
	}
	release := Release{TagName: payload.TagName, Assets: map[string]ReleaseAsset{}}
	for _, asset := range payload.Assets {
		release.Assets[asset.Name] = ReleaseAsset{Name: asset.Name, DownloadURL: asset.BrowserDownloadURL}
	}
	return release, nil
}

func (c releaseClient) downloadFile(ctx context.Context, downloadURL string, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return internalError("build download request", map[string]any{"error": err.Error(), "url": downloadURL})
	}
	c.addHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return networkError("download release asset", map[string]any{"error": err.Error(), "url": downloadURL})
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return networkError("download release asset failed", map[string]any{
			"status": resp.Status,
			"url":    downloadURL,
			"body":   string(body),
		})
	}
	out, err := os.Create(dest)
	if err != nil {
		return internalError("create release asset file", map[string]any{"path": dest, "error": err.Error()})
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return networkError("write release asset file", map[string]any{"path": dest, "error": err.Error()})
	}
	return nil
}

func (c releaseClient) addHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", defaultUserAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func releaseAsset(release Release, name string) (ReleaseAsset, error) {
	asset, ok := release.Assets[name]
	if !ok || asset.DownloadURL == "" {
		return ReleaseAsset{}, inputError("release asset missing", map[string]any{
			"asset":            name,
			"release_version":  release.TagName,
			"available_assets": releaseAssetNames(release),
		})
	}
	return asset, nil
}

func releaseAssetNames(release Release) []string {
	names := make([]string, 0, len(release.Assets))
	for name := range release.Assets {
		names = append(names, name)
	}
	return names
}

func inputError(message string, details map[string]any) error {
	return lovarterrors.InputError(message, details)
}

func networkError(message string, details map[string]any) error {
	return lovarterrors.New(lovarterrors.CodeNetworkUnavailable, message, details)
}

func internalError(message string, details map[string]any) error {
	return lovarterrors.Internal(message, details)
}
