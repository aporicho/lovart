package config

import (
	"github.com/aporicho/lovart/internal/registry"
)

// ConfigField describes one configurable parameter.
type ConfigField = registry.FieldSpec

// ConfigFields holds all legal fields for a model.
type ConfigFields struct {
	Model  string        `json:"model"`
	Fields []ConfigField `json:"fields"`
}

// ForModel returns legal config for a model from the runtime registry.
func ForModel(model string) (*ConfigFields, error) {
	fields, err := registry.RequestFields(model)
	if err != nil {
		return nil, err
	}
	return &ConfigFields{Model: model, Fields: fields}, nil
}

// GlobalConfig returns global configuration (cross-model settings).
func GlobalConfig() (map[string]any, error) {
	return nil, nil
}
