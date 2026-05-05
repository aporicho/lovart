// Package registry manages model records and request validation.
package registry

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aporicho/lovart/internal/metadata"
)

// ModelRecord is a registered model with its schema.
type ModelRecord struct {
	Model          string         `json:"model"`
	DisplayName    string         `json:"display_name"`
	Type           string         `json:"type"`
	RequestSchema  map[string]any `json:"request_schema,omitempty"`
	ResponseSchema map[string]any `json:"response_schema,omitempty"`
}

// Registry is the runtime model registry loaded from cached metadata.
type Registry struct {
	records map[string]ModelRecord
}

// Load reads the runtime metadata cache and builds a registry.
func Load() (*Registry, error) {
	listData, err := metadata.ReadGeneratorList()
	if err != nil {
		return nil, err
	}
	schemaData, err := metadata.ReadGeneratorSchema()
	if err != nil {
		return nil, err
	}

	var list struct {
		Items []struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Type        string `json:"type"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listData, &list); err != nil {
		return nil, fmt.Errorf("registry: parse generator list: %w", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(schemaData, &spec); err != nil {
		return nil, fmt.Errorf("registry: parse generator schema: %w", err)
	}

	reg := &Registry{records: map[string]ModelRecord{}}
	for _, item := range list.Items {
		if item.Name == "" {
			continue
		}
		reg.records[item.Name] = ModelRecord{
			Model:         item.Name,
			DisplayName:   item.DisplayName,
			Type:          item.Type,
			RequestSchema: requestSchemaFor(spec, item.Name),
		}
	}
	return reg, nil
}

// Models returns all known records sorted by model name.
func (r *Registry) Models() []ModelRecord {
	models := make([]ModelRecord, 0, len(r.records))
	for _, record := range r.records {
		models = append(models, record)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Model < models[j].Model })
	return models
}

// Model returns a single model record.
func (r *Registry) Model(model string) (ModelRecord, bool) {
	record, ok := r.records[model]
	return record, ok
}

// Validate checks a request body against the model schema.
func Validate(model string, body map[string]any) []string {
	reg, err := Load()
	if err != nil {
		return []string{err.Error()}
	}
	return reg.Validate(model, body)
}

// Validate checks a request body against a loaded registry.
func (r *Registry) Validate(model string, body map[string]any) []string {
	record, ok := r.Model(model)
	if !ok {
		return []string{fmt.Sprintf("unknown model %q", model)}
	}
	if len(record.RequestSchema) == 0 {
		return nil
	}
	return validateObject(record.RequestSchema, body)
}

func requestSchemaFor(spec map[string]any, model string) map[string]any {
	pathsMap, _ := spec["paths"].(map[string]any)
	pathItem, _ := pathsMap["/"+model].(map[string]any)
	post, _ := pathItem["post"].(map[string]any)
	requestBody, _ := post["requestBody"].(map[string]any)
	content, _ := requestBody["content"].(map[string]any)
	jsonContent, _ := content["application/json"].(map[string]any)
	schema, _ := jsonContent["schema"].(map[string]any)
	ref, _ := schema["$ref"].(string)
	if ref == "" {
		return schema
	}
	return resolveRef(spec, ref)
}

func resolveRef(spec map[string]any, ref string) map[string]any {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return nil
	}
	components, _ := spec["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	schema, _ := schemas[strings.TrimPrefix(ref, prefix)].(map[string]any)
	return schema
}

func validateObject(schema map[string]any, body map[string]any) []string {
	var issues []string
	props, _ := schema["properties"].(map[string]any)
	required := requiredSet(schema["required"])
	for key := range required {
		if _, ok := body[key]; !ok {
			issues = append(issues, fmt.Sprintf("missing required field %q", key))
		}
	}
	for key, value := range body {
		rawProp, ok := props[key]
		if !ok {
			issues = append(issues, fmt.Sprintf("unknown field %q", key))
			continue
		}
		prop, _ := rawProp.(map[string]any)
		issues = append(issues, validateField(key, value, prop)...)
	}
	sort.Strings(issues)
	return issues
}

func validateField(key string, value any, schema map[string]any) []string {
	var issues []string
	if typ, _ := schema["type"].(string); typ != "" && !matchesType(value, typ) {
		issues = append(issues, fmt.Sprintf("field %q must be %s", key, typ))
	}
	if values, ok := schema["enum"].([]any); ok && len(values) > 0 && !containsEnum(values, value) {
		issues = append(issues, fmt.Sprintf("field %q has unsupported value %v", key, value))
	}
	if min, ok := number(schema["minimum"]); ok {
		if got, ok := number(value); ok && got < min {
			issues = append(issues, fmt.Sprintf("field %q must be >= %v", key, min))
		}
	}
	if max, ok := number(schema["maximum"]); ok {
		if got, ok := number(value); ok && got > max {
			issues = append(issues, fmt.Sprintf("field %q must be <= %v", key, max))
		}
	}
	return issues
}

func requiredSet(value any) map[string]struct{} {
	required := map[string]struct{}{}
	items, _ := value.([]any)
	for _, item := range items {
		if key, ok := item.(string); ok {
			required[key] = struct{}{}
		}
	}
	return required
}

func matchesType(value any, typ string) bool {
	switch typ {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := number(value)
		return ok
	case "integer":
		n, ok := number(value)
		return ok && n == float64(int64(n))
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		return true
	}
}

func containsEnum(values []any, value any) bool {
	for _, candidate := range values {
		if fmt.Sprint(candidate) == fmt.Sprint(value) {
			return true
		}
	}
	return false
}

func number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
