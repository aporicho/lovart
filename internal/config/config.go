// Package config resolves legal model parameters from schemas.
package config

// ConfigFields describes all legal fields for a model.
type ConfigFields struct {
	Model  string              `json:"model"`
	Fields []ConfigField       `json:"fields"`
}

// ConfigField describes one parameter (prompt, size, quality, etc).
type ConfigField struct {
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Visible     bool     `json:"visible"`
	Default     any      `json:"default,omitempty"`
	Description string   `json:"description,omitempty"`
	Enumerable  *bool    `json:"enumerable,omitempty"`
	Values      []string `json:"values,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
	MinItems    *int     `json:"minItems,omitempty"`
	MaxItems    *int     `json:"maxItems,omitempty"`
}

// ForModel returns legal config for a model.
func ForModel(model string, includeAll bool) (*ConfigFields, error) {
	// TODO: load schema from ref data or live API
	return nil, nil
}

// GlobalConfig returns global (cross-model) config.
func GlobalConfig() (map[string]any, error) {
	return nil, nil
}
