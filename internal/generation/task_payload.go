package generation

import "github.com/aporicho/lovart/internal/registry"

func buildNormalizedTaskPayload(model string, body map[string]any, opts Options) (map[string]any, map[string]any, error) {
	normalizedBody, err := registry.NormalizeRequest(model, body)
	if err != nil {
		return nil, nil, err
	}
	return buildTaskPayload(model, normalizedBody, opts), normalizedBody, nil
}

func buildTaskPayload(model string, body map[string]any, opts Options) map[string]any {
	payload := map[string]any{
		"generator_name": model,
	}
	if len(body) > 0 {
		payload["input_args"] = body
	}
	if opts.CID != "" {
		payload["cid"] = opts.CID
	}
	if opts.ProjectID != "" {
		payload["project_id"] = opts.ProjectID
	}
	return payload
}
