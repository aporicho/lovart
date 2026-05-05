package registry

import "strings"

func (r *Registry) requestSchemaFor(model string) map[string]any {
	pathsMap, _ := r.spec["paths"].(map[string]any)
	pathItem, _ := pathsMap["/"+model].(map[string]any)
	post, _ := pathItem["post"].(map[string]any)
	requestBody, _ := post["requestBody"].(map[string]any)
	content, _ := requestBody["content"].(map[string]any)
	jsonContent, _ := content["application/json"].(map[string]any)
	schema, _ := jsonContent["schema"].(map[string]any)
	return r.resolveSchema(schema)
}

func (r *Registry) resolveSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	if ref, _ := schema["$ref"].(string); ref != "" {
		if resolved := r.resolveRef(ref); resolved != nil {
			return resolved
		}
	}
	return schema
}

func (r *Registry) resolveRef(ref string) map[string]any {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return nil
	}
	components, _ := r.spec["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	schema, _ := schemas[strings.TrimPrefix(ref, prefix)].(map[string]any)
	return schema
}

func schemaMap(value any) map[string]any {
	schema, _ := value.(map[string]any)
	return schema
}

func schemaList(value any) []map[string]any {
	raw, _ := value.([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if schema, ok := item.(map[string]any); ok {
			out = append(out, schema)
		}
	}
	return out
}
