package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aporicho/lovart/internal/paths"
)

type canvasBackupManifest struct {
	ProjectID     string   `json:"project_id"`
	ProjectName   string   `json:"project_name,omitempty"`
	Version       string   `json:"version,omitempty"`
	PicCount      int      `json:"pic_count"`
	CoverList     []string `json:"cover_list,omitempty"`
	BackedUpAt    string   `json:"backed_up_at"`
	CanvasFile    string   `json:"canvas_file"`
	DecodedFile   string   `json:"decoded_file,omitempty"`
	DecodeWarning string   `json:"decode_warning,omitempty"`
}

func backupCanvasBeforeWrite(projectID string, original *canvasState) (string, error) {
	if original == nil {
		return "", fmt.Errorf("canvas backup: missing original canvas")
	}
	safeProjectID := safeBackupPathPart(projectID)
	if safeProjectID == "" {
		safeProjectID = "unknown-project"
	}

	timestamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	dir := filepath.Join(paths.Root, "backups", "canvas", safeProjectID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("canvas backup: create dir: %w", err)
	}

	canvasPath := filepath.Join(dir, timestamp+".shakkerdata")
	if err := os.WriteFile(canvasPath, []byte(original.Canvas), 0600); err != nil {
		return "", fmt.Errorf("canvas backup: write canvas: %w", err)
	}

	manifest := canvasBackupManifest{
		ProjectID:   projectID,
		ProjectName: original.Name,
		Version:     original.Version,
		PicCount:    original.PicCount,
		CoverList:   append([]string(nil), original.CoverList...),
		BackedUpAt:  time.Now().UTC().Format(time.RFC3339Nano),
		CanvasFile:  canvasPath,
	}
	if jsonStr, err := decodeCanvasJSON(original.Canvas); err == nil {
		if manifest.PicCount == 0 {
			manifest.PicCount = countCImagesGJSON(jsonStr, canvasStorePath)
		}
		if manifest.CoverList == nil {
			manifest.CoverList = extractCoverListGJSON(jsonStr)
		}
		decodedPath := filepath.Join(dir, timestamp+".json")
		if err := os.WriteFile(decodedPath, []byte(jsonStr), 0600); err != nil {
			return "", fmt.Errorf("canvas backup: write decoded canvas: %w", err)
		}
		manifest.DecodedFile = decodedPath
	} else if strings.TrimSpace(original.Canvas) != "" {
		manifest.DecodeWarning = err.Error()
	}

	manifestPath := filepath.Join(dir, timestamp+".manifest.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("canvas backup: marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		return "", fmt.Errorf("canvas backup: write manifest: %w", err)
	}
	return manifestPath, nil
}

func safeBackupPathPart(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
