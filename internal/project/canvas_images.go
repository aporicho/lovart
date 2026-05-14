package project

import "github.com/aporicho/lovart/internal/downloads"

const defaultCanvasImageSize = 1024

// CanvasImagesFromArtifacts converts generated artifacts into canvas image nodes.
func CanvasImagesFromArtifacts(taskID string, artifacts []downloads.Artifact) []CanvasImage {
	images := make([]CanvasImage, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.URL == "" {
			continue
		}
		width := artifact.Width
		if width == 0 {
			width = defaultCanvasImageSize
		}
		height := artifact.Height
		if height == 0 {
			height = defaultCanvasImageSize
		}
		images = append(images, CanvasImage{
			TaskID: taskID,
			URL:    artifact.URL,
			Width:  width,
			Height: height,
		})
	}
	return images
}

// CanvasImagesFromTask converts a completed generation task into canvas image nodes.
func CanvasImagesFromTask(taskID string, task map[string]any) []CanvasImage {
	return CanvasImagesFromArtifacts(taskID, downloads.ArtifactsFromTask(task))
}
