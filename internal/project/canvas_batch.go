package project

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// AddBatchToCanvas adds generated images to the project canvas as organized sections.
func AddBatchToCanvas(ctx context.Context, client *http.Client, projectID, cid string, batch CanvasBatch) error {
	if canvasBatchImageCount(batch) == 0 {
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

	mutated, err := addBatchToCanvasJSON(jsonStr, batch)
	if err != nil {
		return fmt.Errorf("canvas: add batch: %w", err)
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

func canvasBatchImageCount(batch CanvasBatch) int {
	count := 0
	for _, section := range batch.Sections {
		count += len(section.Images)
	}
	return count
}
