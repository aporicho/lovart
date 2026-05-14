package project

import (
	"context"
	"fmt"
	"sort"

	"github.com/aporicho/lovart/internal/http"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// CanvasRecoverOptions controls a developer-only canvas recovery operation.
type CanvasRecoverOptions struct {
	TaskID       string
	ArtifactURLs []string
	Apply        bool
}

// CanvasRecoverNode describes one canvas node considered by recovery.
type CanvasRecoverNode struct {
	ID       string `json:"id"`
	Type     string `json:"type,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	TaskID   string `json:"task_id,omitempty"`
	URL      string `json:"url,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// CanvasRecoverResult reports a developer-only recovery dry-run or write.
type CanvasRecoverResult struct {
	ProjectID      string              `json:"project_id"`
	Applied        bool                `json:"applied"`
	BackupPath     string              `json:"backup_path,omitempty"`
	MatchedImages  []CanvasRecoverNode `json:"matched_images"`
	DeletedNodes   []CanvasRecoverNode `json:"deleted_nodes"`
	PicCountBefore int                 `json:"pic_count_before"`
	PicCountAfter  int                 `json:"pic_count_after"`
	CoverListAfter []string            `json:"cover_list_after,omitempty"`
}

type recoverNode struct {
	id       string
	typ      string
	parentID string
	taskID   string
	url      string
	reason   string
}

func RecoverCanvasByTask(ctx context.Context, client *http.Client, projectID, cid string, opts CanvasRecoverOptions) (*CanvasRecoverResult, error) {
	if opts.TaskID == "" && len(opts.ArtifactURLs) == 0 {
		return nil, fmt.Errorf("canvas recover: task id or artifact url required")
	}
	fullCanvas, err := queryCanvas(ctx, client, projectID, cid)
	if err != nil {
		return nil, fmt.Errorf("canvas recover: query: %w", err)
	}
	originalCanvas := *fullCanvas

	jsonStr, err := decodeCanvasJSON(fullCanvas.Canvas)
	if err != nil {
		return nil, fmt.Errorf("canvas recover: decode: %w", err)
	}

	nextJSON, result, err := recoverCanvasJSON(jsonStr, projectID, opts)
	if err != nil {
		return nil, err
	}
	if len(result.MatchedImages) == 0 {
		return result, nil
	}
	if !opts.Apply {
		return result, nil
	}

	newCanvas, err := encodeCanvasJSON(nextJSON)
	if err != nil {
		return nil, fmt.Errorf("canvas recover: encode: %w", err)
	}
	fullCanvas.Canvas = newCanvas
	fullCanvas.PicCount = result.PicCountAfter
	fullCanvas.CoverList = result.CoverListAfter
	if fullCanvas.Name == "" {
		fullCanvas.Name = "Untitled"
	}

	backupPath, err := saveCanvasWithBackup(ctx, client, projectID, cid, &originalCanvas, fullCanvas)
	if err != nil {
		return nil, fmt.Errorf("canvas recover: save: %w", err)
	}
	result.Applied = true
	result.BackupPath = backupPath
	return result, nil
}

func recoverCanvasJSON(jsonStr, projectID string, opts CanvasRecoverOptions) (string, *CanvasRecoverResult, error) {
	result := &CanvasRecoverResult{
		ProjectID:      projectID,
		PicCountBefore: countCImagesGJSON(jsonStr, canvasStorePath),
	}
	urls := map[string]bool{}
	for _, url := range opts.ArtifactURLs {
		if url != "" {
			urls[url] = true
		}
	}

	nodes := canvasRecoverNodes(jsonStr)
	matched := map[string]bool{}
	for id, node := range nodes {
		if node.typ != "c-image" {
			continue
		}
		if opts.TaskID != "" && node.taskID == opts.TaskID {
			node.reason = "task_id"
			nodes[id] = node
			matched[id] = true
			continue
		}
		if urls[node.url] {
			node.reason = "artifact_url"
			nodes[id] = node
			matched[id] = true
		}
	}

	deleteIDs := map[string]bool{}
	for id := range matched {
		deleteIDs[id] = true
		result.MatchedImages = append(result.MatchedImages, exportRecoverNode(nodes[id]))
	}
	for _, frameID := range removableRecoverFrames(nodes, matched) {
		deleteIDs[frameID] = true
		frame := nodes[frameID]
		frame.reason = "matched_frame"
		nodes[frameID] = frame
		for id, node := range nodes {
			if node.parentID == frameID && node.typ == "text" {
				deleteIDs[id] = true
				node.reason = "matched_frame_text"
				nodes[id] = node
			}
		}
	}

	ids := make([]string, 0, len(deleteIDs))
	for id := range deleteIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		var err error
		jsonStr, err = sjson.Delete(jsonStr, canvasStorePath+"."+id)
		if err != nil {
			return "", nil, fmt.Errorf("canvas recover: delete %s: %w", id, err)
		}
		result.DeletedNodes = append(result.DeletedNodes, exportRecoverNode(nodes[id]))
	}
	result.PicCountAfter = countCImagesGJSON(jsonStr, canvasStorePath)
	result.CoverListAfter = extractCoverListGJSON(jsonStr)
	return jsonStr, result, nil
}

func canvasRecoverNodes(jsonStr string) map[string]recoverNode {
	nodes := map[string]recoverNode{}
	store := gjson.Get(jsonStr, canvasStorePath)
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("typeName").String() != "shape" {
			return true
		}
		id := key.String()
		nodes[id] = recoverNode{
			id:       id,
			typ:      value.Get("type").String(),
			parentID: value.Get("parentId").String(),
			taskID:   value.Get("props.generatorTaskId").String(),
			url:      value.Get("props.url").String(),
		}
		return true
	})
	return nodes
}

func removableRecoverFrames(nodes map[string]recoverNode, matched map[string]bool) []string {
	var frames []string
	for id, node := range nodes {
		if node.typ != "frame" {
			continue
		}
		hasMatchedImage := false
		hasOther := false
		for childID, child := range nodes {
			if child.parentID != id {
				continue
			}
			if matched[childID] && child.typ == "c-image" {
				hasMatchedImage = true
				continue
			}
			if child.typ == "text" {
				continue
			}
			hasOther = true
		}
		if hasMatchedImage && !hasOther {
			frames = append(frames, id)
		}
	}
	sort.Strings(frames)
	return frames
}

func exportRecoverNode(node recoverNode) CanvasRecoverNode {
	return CanvasRecoverNode{
		ID:       node.id,
		Type:     node.typ,
		ParentID: node.parentID,
		TaskID:   node.taskID,
		URL:      node.url,
		Reason:   node.reason,
	}
}
