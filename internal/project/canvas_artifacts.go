package project

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aporicho/lovart/internal/http"
	"github.com/tidwall/gjson"
)

// CanvasArtifact is one downloadable image-like product on a Lovart canvas.
type CanvasArtifact struct {
	ArtifactID    string          `json:"artifact_id"`
	Index         int             `json:"index"`
	Type          string          `json:"type"`
	URL           string          `json:"url"`
	OriginalURL   string          `json:"original_url,omitempty"`
	TaskID        string          `json:"task_id,omitempty"`
	Name          string          `json:"name,omitempty"`
	DisplayWidth  int             `json:"display_width,omitempty"`
	DisplayHeight int             `json:"display_height,omitempty"`
	X             float64         `json:"x,omitempty"`
	Y             float64         `json:"y,omitempty"`
	ParentID      string          `json:"parent_id,omitempty"`
	FrameID       string          `json:"frame_id,omitempty"`
	Raw           json.RawMessage `json:"raw,omitempty"`
}

// CanvasArtifactsOptions controls canvas artifact extraction and filtering.
type CanvasArtifactsOptions struct {
	TaskID     string
	Limit      int
	Offset     int
	IncludeRaw bool
}

// CanvasArtifactsResult is the project-facing canvas artifact list response.
type CanvasArtifactsResult struct {
	ProjectID       string           `json:"project_id"`
	ProjectName     string           `json:"project_name,omitempty"`
	CanvasURL       string           `json:"canvas_url"`
	Count           int              `json:"count"`
	Total           int              `json:"total"`
	Offset          int              `json:"offset,omitempty"`
	Limit           int              `json:"limit,omitempty"`
	Artifacts       []CanvasArtifact `json:"artifacts"`
	ShapeTypeCounts map[string]int   `json:"shape_type_counts,omitempty"`
}

// ListCanvasArtifacts reads a project canvas and returns downloadable c-image artifacts.
func ListCanvasArtifacts(ctx context.Context, client *http.Client, projectID, cid string, opts CanvasArtifactsOptions) (*CanvasArtifactsResult, error) {
	fullCanvas, err := queryCanvas(ctx, client, projectID, cid)
	if err != nil {
		return nil, fmt.Errorf("canvas artifacts: query: %w", err)
	}
	jsonStr, err := decodeCanvasJSON(fullCanvas.Canvas)
	if err != nil {
		return nil, fmt.Errorf("canvas artifacts: decode: %w", err)
	}
	result, err := extractCanvasArtifacts(jsonStr, opts)
	if err != nil {
		return nil, err
	}
	result.ProjectID = projectID
	result.ProjectName = fullCanvas.Name
	result.CanvasURL = CanvasURL(projectID)
	return result, nil
}

// GetCanvasArtifact returns one canvas artifact by artifact_id.
func GetCanvasArtifact(ctx context.Context, client *http.Client, projectID, cid, artifactID string, includeRaw bool) (*CanvasArtifactsResult, error) {
	if strings.TrimSpace(artifactID) == "" {
		return nil, fmt.Errorf("canvas artifacts: artifact id is required")
	}
	result, err := ListCanvasArtifacts(ctx, client, projectID, cid, CanvasArtifactsOptions{IncludeRaw: includeRaw})
	if err != nil {
		return nil, err
	}
	for _, artifact := range result.Artifacts {
		if artifact.ArtifactID == artifactID {
			result.Artifacts = []CanvasArtifact{artifact}
			result.Count = 1
			result.Total = 1
			result.Offset = 0
			result.Limit = 0
			return result, nil
		}
	}
	return nil, fmt.Errorf("canvas artifacts: artifact %q not found", artifactID)
}

func extractCanvasArtifacts(jsonStr string, opts CanvasArtifactsOptions) (*CanvasArtifactsResult, error) {
	if jsonStr == "" {
		jsonStr = defaultCanvasJSON()
	}
	if !json.Valid([]byte(jsonStr)) {
		return nil, fmt.Errorf("canvas artifacts: invalid canvas JSON")
	}
	store := gjson.Get(jsonStr, canvasStorePath)
	if !store.Exists() {
		return &CanvasArtifactsResult{Artifacts: []CanvasArtifact{}, ShapeTypeCounts: map[string]int{}}, nil
	}

	shapes := map[string]canvasArtifactShape{}
	shapeTypeCounts := map[string]int{}
	store.ForEach(func(key, value gjson.Result) bool {
		id := key.String()
		shapeType := value.Get("type").String()
		if shapeType != "" {
			shapeTypeCounts[shapeType]++
		}
		shapes[id] = canvasArtifactShape{
			ID:       id,
			Type:     shapeType,
			ParentID: value.Get("parentId").String(),
			Index:    value.Get("index").String(),
			X:        value.Get("x").Float(),
			Y:        value.Get("y").Float(),
		}
		return true
	})

	var artifacts []canvasArtifactWithOrder
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("type").String() != "c-image" {
			return true
		}
		url := value.Get("props.url").String()
		originalURL := value.Get("props.originalUrl").String()
		if url == "" && originalURL == "" {
			return true
		}
		id := key.String()
		taskID := value.Get("props.generatorTaskId").String()
		if opts.TaskID != "" && taskID != opts.TaskID {
			return true
		}
		artifact := CanvasArtifact{
			ArtifactID:    id,
			Type:          "image",
			URL:           url,
			OriginalURL:   originalURL,
			TaskID:        taskID,
			Name:          value.Get("props.name").String(),
			DisplayWidth:  int(value.Get("props.w").Int()),
			DisplayHeight: int(value.Get("props.h").Int()),
			X:             value.Get("x").Float(),
			Y:             value.Get("y").Float(),
			ParentID:      value.Get("parentId").String(),
			FrameID:       canvasArtifactFrameID(shapes, value.Get("parentId").String()),
		}
		if artifact.URL == "" {
			artifact.URL = artifact.OriginalURL
		}
		if opts.IncludeRaw && value.Raw != "" {
			artifact.Raw = json.RawMessage(value.Raw)
		}
		artifacts = append(artifacts, canvasArtifactWithOrder{
			Artifact: artifact,
			Order:    canvasArtifactOrder(shapes, artifact),
		})
		return true
	})

	sort.SliceStable(artifacts, func(i, j int) bool {
		return artifacts[i].Order.Less(artifacts[j].Order)
	})

	all := make([]CanvasArtifact, 0, len(artifacts))
	for i, item := range artifacts {
		item.Artifact.Index = i + 1
		all = append(all, item.Artifact)
	}

	total := len(all)
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	limit := opts.Limit
	if limit < 0 {
		limit = 0
	}
	end := total
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	selected := append([]CanvasArtifact(nil), all[offset:end]...)

	return &CanvasArtifactsResult{
		Count:           len(selected),
		Total:           total,
		Offset:          offset,
		Limit:           limit,
		Artifacts:       selected,
		ShapeTypeCounts: shapeTypeCounts,
	}, nil
}

type canvasArtifactShape struct {
	ID       string
	Type     string
	ParentID string
	Index    string
	X        float64
	Y        float64
}

type canvasArtifactWithOrder struct {
	Artifact CanvasArtifact
	Order    canvasArtifactSortKey
}

type canvasArtifactSortKey struct {
	FrameY     float64
	FrameX     float64
	FrameIndex string
	FrameID    string
	ImageY     float64
	ImageX     float64
	ImageIndex string
	ArtifactID string
}

func (k canvasArtifactSortKey) Less(other canvasArtifactSortKey) bool {
	if k.FrameY != other.FrameY {
		return k.FrameY < other.FrameY
	}
	if k.FrameX != other.FrameX {
		return k.FrameX < other.FrameX
	}
	if k.FrameIndex != other.FrameIndex {
		return k.FrameIndex < other.FrameIndex
	}
	if k.FrameID != other.FrameID {
		return k.FrameID < other.FrameID
	}
	if k.ImageY != other.ImageY {
		return k.ImageY < other.ImageY
	}
	if k.ImageX != other.ImageX {
		return k.ImageX < other.ImageX
	}
	if k.ImageIndex != other.ImageIndex {
		return k.ImageIndex < other.ImageIndex
	}
	return k.ArtifactID < other.ArtifactID
}

func canvasArtifactFrameID(shapes map[string]canvasArtifactShape, parentID string) string {
	for parentID != "" && parentID != "page:page" {
		parent, ok := shapes[parentID]
		if !ok {
			return ""
		}
		if parent.Type == "frame" {
			return parent.ID
		}
		parentID = parent.ParentID
	}
	return ""
}

func canvasArtifactOrder(shapes map[string]canvasArtifactShape, artifact CanvasArtifact) canvasArtifactSortKey {
	key := canvasArtifactSortKey{
		ImageY:     artifact.Y,
		ImageX:     artifact.X,
		ArtifactID: artifact.ArtifactID,
	}
	if self, ok := shapes[artifact.ArtifactID]; ok {
		key.ImageIndex = self.Index
	}
	if artifact.FrameID == "" {
		key.FrameY = artifact.Y
		key.FrameX = artifact.X
		return key
	}
	frame := shapes[artifact.FrameID]
	key.FrameY = frame.Y
	key.FrameX = frame.X
	key.FrameIndex = frame.Index
	key.FrameID = frame.ID
	return key
}
