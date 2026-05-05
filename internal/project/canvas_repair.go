package project

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// RepairCanvas normalizes a project's canvas and saves it when repairs are needed.
func RepairCanvas(ctx context.Context, client *http.Client, projectID, cid string) (*CanvasRepairResult, error) {
	fullCanvas, err := queryCanvas(ctx, client, projectID, cid)
	if err != nil {
		return nil, fmt.Errorf("canvas repair: query: %w", err)
	}

	jsonStr, err := decodeCanvasJSON(fullCanvas.Canvas)
	if err != nil {
		return nil, fmt.Errorf("canvas repair: decode: %w", err)
	}

	mutated, result, err := normalizeCanvasJSON(jsonStr)
	if err != nil {
		return nil, fmt.Errorf("canvas repair: normalize: %w", err)
	}
	if !result.Changed {
		return &result, nil
	}

	newCanvas, err := encodeCanvasJSON(mutated.JSON)
	if err != nil {
		return nil, fmt.Errorf("canvas repair: encode: %w", err)
	}

	fullCanvas.Canvas = newCanvas
	fullCanvas.PicCount = mutated.PicCount
	fullCanvas.CoverList = mutated.CoverList
	if fullCanvas.Name == "" {
		fullCanvas.Name = "Untitled"
	}

	if err := saveCanvas(ctx, client, projectID, cid, fullCanvas); err != nil {
		return nil, fmt.Errorf("canvas repair: save: %w", err)
	}
	return &result, nil
}
