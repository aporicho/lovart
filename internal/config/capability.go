package config

import (
	"encoding/json"
	"fmt"
)

// OutputCap describes a model's multi-output generation capability.
type OutputCap struct {
	MultiField   string // schema field name: "n", "max_images", "num_images", or empty
	MaxOutputs   int    // max value of the field (0 if no limit)
	BatchSize    int    // fixed batch output per API call (Midjourney=4, others=1)
	IsFixedBatch bool   // true if model outputs a fixed batch per API call
}

// Fixed batch models — one API call produces this many images.
var fixedBatchModels = map[string]int{
	"youchuan/midjourney": 4,
}

// OutputCapability returns the output capability for a model by reading the schema.
func OutputCapability(model string) *OutputCap {
	cap := &OutputCap{
		BatchSize: 1,
		MaxOutputs: 1,
	}

	// Check fixed batch models first.
	if batch, ok := fixedBatchModels[model]; ok {
		cap.IsFixedBatch = true
		cap.BatchSize = batch
		return cap
	}

	// Load schema and find the model's request component.
	fieldName, maxVal := findMultiField(model)
	if fieldName != "" {
		cap.MultiField = fieldName
		if maxVal > 0 {
			cap.MaxOutputs = maxVal
		}
	}
	return cap
}

// findMultiField looks up the multi-output field from the embedded schema.
func findMultiField(model string) (fieldName string, maxVal int) {
	schemaData, err := refAssets.ReadFile("assets/lovart_generator_schema.json")
	if err != nil {
		return "", 0
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
					Minimum *float64 `json:"minimum"`
					Maximum *float64 `json:"maximum"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}

	if err := json.Unmarshal(schemaData, &spec); err != nil {
		return "", 0
	}

	// Find the component for this model.
	modelPath := fmt.Sprintf("/%s", model)
	path, ok := spec.Paths[modelPath]
	if !ok {
		return "", 0
	}

	schemaRef := path.Post.RequestBody.Content.JSON.Schema.Ref
	if schemaRef == "" {
		return "", 0
	}

	var schemaName string
	fmt.Sscanf(schemaRef, "#/components/schemas/%s", &schemaName)
	comp, ok := spec.Components.Schemas[schemaName]
	if !ok {
		return "", 0
	}

	// Check each known multi-output field name.
	for _, name := range []string{"n", "max_images", "num_images"} {
		if prop, ok := comp.Properties[name]; ok {
			max := 0
			if prop.Maximum != nil {
				max = int(*prop.Maximum)
			}
			return name, max
		}
	}
	return "", 0
}
