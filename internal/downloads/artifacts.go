package downloads

// ArtifactsFromTask extracts artifact details from a generation task response.
func ArtifactsFromTask(task map[string]any) []Artifact {
	if task == nil {
		return nil
	}
	return ArtifactsFromDetails(artifactDetails(task))
}

// ArtifactsFromDetails converts loosely typed artifact details into the public
// download artifact model.
func ArtifactsFromDetails(details []map[string]any) []Artifact {
	artifacts := make([]Artifact, 0, len(details))
	for i, detail := range details {
		url, _ := detail["url"].(string)
		if url == "" {
			url, _ = detail["content"].(string)
		}
		if url == "" {
			continue
		}
		artifacts = append(artifacts, Artifact{
			URL:    url,
			Width:  numericInt(detail["width"]),
			Height: numericInt(detail["height"]),
			Index:  i + 1,
		})
	}
	return artifacts
}

func artifactDetails(task map[string]any) []map[string]any {
	if raw, _ := task["artifact_details"].([]map[string]any); raw != nil {
		return raw
	}
	if values, _ := task["artifact_details"].([]any); len(values) > 0 {
		out := make([]map[string]any, 0, len(values))
		for _, value := range values {
			if item, ok := value.(map[string]any); ok {
				out = append(out, item)
			}
		}
		return out
	}
	if values, _ := task["artifacts"].([]string); len(values) > 0 {
		out := make([]map[string]any, 0, len(values))
		for _, url := range values {
			out = append(out, map[string]any{"url": url})
		}
		return out
	}
	if values, _ := task["artifacts"].([]any); len(values) > 0 {
		out := make([]map[string]any, 0, len(values))
		for _, value := range values {
			switch item := value.(type) {
			case string:
				out = append(out, map[string]any{"url": item})
			case map[string]any:
				out = append(out, item)
			}
		}
		return out
	}
	return nil
}

func numericInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case jsonNumber:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

type jsonNumber interface {
	Int64() (int64, error)
}
