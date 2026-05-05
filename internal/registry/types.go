// Package registry manages model records and validates request bodies against schemas.
package registry

// ModelRecord is a registered model with its request and response schemas.
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
	spec    map[string]any
}

// FieldSpec describes one user-facing request field.
type FieldSpec struct {
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

// OutputCap describes a model's multi-output generation capability.
type OutputCap struct {
	MultiField   string `json:"multi_field,omitempty"`
	MaxOutputs   int    `json:"max_outputs"`
	BatchSize    int    `json:"batch_size"`
	IsFixedBatch bool   `json:"is_fixed_batch"`
}

// ValidationResult is the structured outcome of validating a request body.
type ValidationResult struct {
	OK     bool              `json:"ok"`
	Model  string            `json:"model"`
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// ValidationIssue is a machine-readable schema validation failure.
type ValidationIssue struct {
	Path          string   `json:"path"`
	Code          string   `json:"code"`
	Message       string   `json:"message"`
	Expected      string   `json:"expected,omitempty"`
	Actual        any      `json:"actual,omitempty"`
	AllowedValues []string `json:"allowed_values,omitempty"`
	Minimum       *float64 `json:"minimum,omitempty"`
	Maximum       *float64 `json:"maximum,omitempty"`
}
