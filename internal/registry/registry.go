// Package registry manages model records and request validation.
package registry

// ModelRecord is a registered model with its schema.
type ModelRecord struct {
	Model          string `json:"model"`
	DisplayName    string `json:"display_name"`
	Type           string `json:"type"`
	RequestSchema  any    `json:"request_schema,omitempty"`
	ResponseSchema any    `json:"response_schema,omitempty"`
}

// Validate checks a request body against the model schema.
func Validate(model string, body map[string]any) []string {
	// TODO: implement schema validation
	return nil
}
