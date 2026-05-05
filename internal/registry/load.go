package registry

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aporicho/lovart/internal/metadata"
)

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

	reg := &Registry{
		records: make(map[string]ModelRecord),
		spec:    spec,
	}
	for _, item := range list.Items {
		if item.Name == "" {
			continue
		}
		reg.records[item.Name] = ModelRecord{
			Model:         item.Name,
			DisplayName:   item.DisplayName,
			Type:          item.Type,
			RequestSchema: reg.requestSchemaFor(item.Name),
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
