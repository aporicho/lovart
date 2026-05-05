package downloads

func buildEffectMetadata(ctx JobContext, artifact Artifact) EffectMetadata {
	return EffectMetadata{
		SchemaVersion: 1,
		Model:         ctx.Model,
		Mode:          ctx.Mode,
		InputArgs:     copyMap(ctx.Body),
		Artifact: EffectArtifactRef{
			Index:  artifact.Index,
			Width:  artifact.Width,
			Height: artifact.Height,
		},
	}
}

func copyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = copyValue(v)
	}
	return out
}

func copyValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return copyMap(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = copyValue(item)
		}
		return out
	default:
		return value
	}
}
