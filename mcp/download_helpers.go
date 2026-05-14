package mcp

import (
	"github.com/aporicho/lovart/internal/downloads"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/project"
)

func selectDownloadArtifactIndex(artifacts []downloads.Artifact, index int) ([]downloads.Artifact, *envelope.Envelope) {
	if len(artifacts) == 0 {
		env := envelope.Err(errors.CodeInputError, "no downloadable artifacts found", nil)
		return nil, &env
	}
	if index == 0 {
		return artifacts, nil
	}
	if index < 1 || index > len(artifacts) {
		env := envelope.Err(errors.CodeInputError, "artifact index out of range", map[string]any{
			"index": index,
			"count": len(artifacts),
		})
		return nil, &env
	}
	return []downloads.Artifact{artifacts[index-1]}, nil
}

func selectCanvasArtifacts(artifacts []project.CanvasArtifact, artifactID string, artifactIndex int, taskID string, all bool) ([]project.CanvasArtifact, *envelope.Envelope) {
	if all {
		if len(artifacts) == 0 {
			env := envelope.Err(errors.CodeInputError, "no downloadable canvas artifacts found", nil)
			return nil, &env
		}
		return artifacts, nil
	}
	if artifactID != "" {
		for _, artifact := range artifacts {
			if artifact.ArtifactID == artifactID {
				return []project.CanvasArtifact{artifact}, nil
			}
		}
		env := envelope.Err(errors.CodeInputError, "canvas artifact not found", map[string]any{"artifact_id": artifactID, "count": len(artifacts)})
		return nil, &env
	}
	if artifactIndex != 0 {
		if artifactIndex < 1 || artifactIndex > len(artifacts) {
			env := envelope.Err(errors.CodeInputError, "canvas artifact index out of range", map[string]any{"index": artifactIndex, "count": len(artifacts)})
			return nil, &env
		}
		return []project.CanvasArtifact{artifacts[artifactIndex-1]}, nil
	}
	var selected []project.CanvasArtifact
	for _, artifact := range artifacts {
		if artifact.TaskID == taskID {
			selected = append(selected, artifact)
		}
	}
	if len(selected) == 0 {
		env := envelope.Err(errors.CodeInputError, "no canvas artifacts found for task", map[string]any{"task_id": taskID, "count": len(artifacts)})
		return nil, &env
	}
	return selected, nil
}

func canvasDownloadArtifacts(artifacts []project.CanvasArtifact, original bool) []downloads.Artifact {
	out := make([]downloads.Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		url := artifact.URL
		if original && artifact.OriginalURL != "" {
			url = artifact.OriginalURL
		}
		out = append(out, downloads.Artifact{
			URL:    url,
			Width:  artifact.DisplayWidth,
			Height: artifact.DisplayHeight,
			Index:  artifact.Index,
		})
	}
	return out
}
