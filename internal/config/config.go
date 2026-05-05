package config

import (
	"encoding/json"
	"fmt"

	"github.com/aporicho/lovart/internal/metadata"
)

// ConfigField describes one configurable parameter.
type ConfigField struct {
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Description string   `json:"description,omitempty"`
	Enumerated  bool     `json:"enumerated"`
	Values      []string `json:"values,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
}

// ConfigFields holds all legal fields for a model.
type ConfigFields struct {
	Model  string        `json:"model"`
	Fields []ConfigField `json:"fields"`
}

// ForModel returns legal config for a model by parsing the runtime OpenAPI schema cache.
func ForModel(model string) (*ConfigFields, error) {
	schemaData, err := metadata.ReadGeneratorSchema()
	if err != nil {
		return nil, fmt.Errorf("config: read schema: %w", err)
	}

	var spec struct {
		Paths map[string]struct {
			Post struct {
				RequestBody struct {
					Content struct {
						JSON struct {
							Schema struct {
								Ref string `json:"$ref"`
							} `json:"schema"`
						} `json:"application/json"`
					} `json:"content"`
				} `json:"requestBody"`
			} `json:"post"`
		} `json:"paths"`
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Type        string           `json:"type"`
					Description string           `json:"description"`
					Default     any              `json:"default"`
					Enum        []any            `json:"enum"`
					Items       *json.RawMessage `json:"items"`
				} `json:"properties"`
				Required []string `json:"required"`
			} `json:"schemas"`
		} `json:"components"`
	}

	if err := json.Unmarshal(schemaData, &spec); err != nil {
		return nil, fmt.Errorf("config: parse schema: %w", err)
	}

	modelPath := fmt.Sprintf("/%s", model)
	path, ok := spec.Paths[modelPath]
	if !ok {
		return nil, fmt.Errorf("config: model %q not found", model)
	}

	schemaRef := path.Post.RequestBody.Content.JSON.Schema.Ref
	if schemaRef == "" {
		return nil, fmt.Errorf("config: no schema ref for model %q", model)
	}

	// Extract component name from $ref: "#/components/schemas/FooRequest"
	var schemaName string
	if n, err := fmt.Sscanf(schemaRef, "#/components/schemas/%s", &schemaName); n != 1 || err != nil {
		return nil, fmt.Errorf("config: bad $ref %q", schemaRef)
	}

	compSchema, ok := spec.Components.Schemas[schemaName]
	if !ok {
		return nil, fmt.Errorf("config: component %q not found", schemaName)
	}

	requiredSet := make(map[string]bool)
	for _, r := range compSchema.Required {
		requiredSet[r] = true
	}

	result := &ConfigFields{Model: model}
	for name, prop := range compSchema.Properties {
		field := ConfigField{
			Key:         name,
			Type:        prop.Type,
			Required:    requiredSet[name],
			Default:     prop.Default,
			Description: prop.Description,
		}

		if len(prop.Enum) > 0 {
			field.Enumerated = true
			for _, e := range prop.Enum {
				field.Values = append(field.Values, fmt.Sprintf("%v", e))
			}
		}

		if prop.Items != nil {
			var itemsSchema struct {
				Enum []any `json:"enum"`
			}
			json.Unmarshal(*prop.Items, &itemsSchema)
			if len(itemsSchema.Enum) > 0 {
				field.Enumerated = true
				for _, e := range itemsSchema.Enum {
					field.Values = append(field.Values, fmt.Sprintf("%v", e))
				}
			}
		}

		result.Fields = append(result.Fields, field)
	}

	return result, nil
}

// GlobalConfig returns global configuration (cross-model settings).
func GlobalConfig() (map[string]any, error) {
	return nil, nil
}
