package project

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// queryCanvas loads the current canvas and metadata from the API.
func queryCanvas(ctx context.Context, client *http.Client, projectID, cid string) (*canvasState, error) {
	path := "/api/canva/project/queryProject"
	body := map[string]any{"projectId": projectID, "cid": cid}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Canvas      string `json:"canvas"`
			Version     string `json:"version"`
			ProjectName string `json:"projectName"`
		} `json:"data"`
	}

	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return nil, fmt.Errorf("canvas: query project: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("canvas: query project returned code %d", resp.Code)
	}

	return &canvasState{
		Canvas:  resp.Data.Canvas,
		Version: resp.Data.Version,
		Name:    resp.Data.ProjectName,
	}, nil
}

// saveCanvas uploads the updated canvas matching the browser's saveProject format.
func saveCanvas(ctx context.Context, client *http.Client, projectID, cid string, cs *canvasState) error {
	path := "/api/canva/project/saveProject"

	sessionID, err := newSessionID()
	if err != nil {
		return fmt.Errorf("canvas: session id: %w", err)
	}

	cover := cs.CoverList
	if cover == nil {
		cover, err = coverList(cs.Canvas)
		if err != nil {
			return fmt.Errorf("canvas: cover list: %w", err)
		}
	}

	body := saveCanvasBody(projectID, cid, sessionID, cs, cover)

	var resp struct {
		Code int `json:"code"`
	}
	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return fmt.Errorf("canvas: save project: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("canvas: save project returned code %d", resp.Code)
	}
	return nil
}

func saveCanvasBody(projectID, cid, sessionID string, cs *canvasState, cover []string) map[string]any {
	body := map[string]any{
		"canvas":           cs.Canvas,
		"projectId":        projectID,
		"projectName":      cs.Name,
		"picCount":         cs.PicCount,
		"version":          cs.Version,
		"sessionId":        sessionID,
		"projectCoverList": cover,
	}
	if cid != "" {
		body["cid"] = cid
	}
	return body
}

func saveCanvasWithBackup(ctx context.Context, client *http.Client, projectID, cid string, original, updated *canvasState) (string, error) {
	backupPath, err := backupCanvasBeforeWrite(projectID, original)
	if err != nil {
		return "", err
	}
	if err := saveCanvas(ctx, client, projectID, cid, updated); err != nil {
		return backupPath, fmt.Errorf("%w (backup: %s)", err, backupPath)
	}
	return backupPath, nil
}
