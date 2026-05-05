package project

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// AddToCanvas adds generated images as nodes on the project canvas.
func AddToCanvas(ctx context.Context, client *http.Client, projectID, cid string, images []CanvasImage) error {
	if len(images) == 0 {
		return nil
	}

	fullCanvas, err := queryCanvas(ctx, client, projectID, cid)
	if err != nil {
		return fmt.Errorf("canvas: query: %w", err)
	}

	jsonStr, err := decodeCanvasJSON(fullCanvas.Canvas)
	if err != nil {
		return fmt.Errorf("canvas: decode: %w", err)
	}

	mutated, err := addImagesToCanvasJSON(jsonStr, images)
	if err != nil {
		return fmt.Errorf("canvas: add images: %w", err)
	}

	newCanvas, err := encodeCanvasJSON(mutated.JSON)
	if err != nil {
		return fmt.Errorf("canvas: encode: %w", err)
	}

	fullCanvas.Canvas = newCanvas
	fullCanvas.PicCount = mutated.PicCount
	fullCanvas.CoverList = mutated.CoverList
	if fullCanvas.Name == "" {
		fullCanvas.Name = "Untitled"
	}

	if err := saveCanvas(ctx, client, projectID, cid, fullCanvas); err != nil {
		return fmt.Errorf("canvas: save: %w", err)
	}

	return nil
}
