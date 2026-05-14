package registry

import "fmt"

// NormalizeRequest returns a request body copy with top-level schema defaults set.
func NormalizeRequest(model string, body map[string]any) (map[string]any, error) {
	reg, err := Load()
	if err != nil {
		return nil, err
	}
	return reg.NormalizeRequest(model, body)
}

// NormalizeRequest returns a request body copy with top-level schema defaults set.
func (r *Registry) NormalizeRequest(model string, body map[string]any) (map[string]any, error) {
	record, ok := r.Model(model)
	if !ok {
		return nil, fmt.Errorf("unknown model %q", model)
	}
	if record.RequestSchema == nil {
		return nil, fmt.Errorf("request schema for model %q is missing", model)
	}
	out := make(map[string]any, len(body))
	for key, value := range body {
		out[key] = cloneJSONValue(value)
	}
	schema := r.resolveSchema(record.RequestSchema)
	props, _ := schema["properties"].(map[string]any)
	for key, rawProp := range props {
		if _, ok := out[key]; ok {
			continue
		}
		prop := r.resolveSchema(schemaMap(rawProp))
		if defaultValue, ok := prop["default"]; ok {
			out[key] = cloneJSONValue(defaultValue)
		}
	}
	return out, nil
}

func cloneJSONValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = cloneJSONValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cloneJSONValue(item)
		}
		return out
	default:
		return value
	}
}
