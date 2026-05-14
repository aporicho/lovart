package registry

import (
	"fmt"
)

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
	result := r.ValidateRequest(model, body)
	issues := make([]string, 0, len(result.Issues))
	for _, issue := range result.Issues {
		issues = append(issues, issue.Message)
	}
	return issues
}

// ValidateRequest checks a request body and returns structured issues.
func ValidateRequest(model string, body map[string]any) ValidationResult {
	reg, err := Load()
	if err != nil {
		return ValidationResult{
			OK:    false,
			Model: model,
			Issues: []ValidationIssue{{
				Path:    "$",
				Code:    "metadata_missing",
				Message: err.Error(),
			}},
		}
	}
	return reg.ValidateRequest(model, body)
}

// ValidateRequest checks a request body against a loaded registry.
func (r *Registry) ValidateRequest(model string, body map[string]any) ValidationResult {
	record, ok := r.Model(model)
	if !ok {
		return ValidationResult{
			OK:    false,
			Model: model,
			Issues: []ValidationIssue{{
				Path:    "$",
				Code:    "unknown_model",
				Message: fmt.Sprintf("unknown model %q", model),
				Actual:  model,
			}},
		}
	}
	if record.RequestSchema == nil {
		return ValidationResult{
			OK:    false,
			Model: model,
			Issues: []ValidationIssue{{
				Path:    "$",
				Code:    "metadata_missing",
				Message: fmt.Sprintf("request schema for model %q is missing", model),
			}},
		}
	}
	normalized, err := r.NormalizeRequest(model, body)
	if err != nil {
		return ValidationResult{
			OK:    false,
			Model: model,
			Issues: []ValidationIssue{{
				Path:    "$",
				Code:    "normalize_failed",
				Message: err.Error(),
			}},
		}
	}
	issues := r.validateSchema("$", record.RequestSchema, normalized)
	sortIssues(issues)
	return ValidationResult{OK: len(issues) == 0, Model: model, Issues: issues}
}

func (r *Registry) validateSchema(path string, schema map[string]any, value any) []ValidationIssue {
	schema = r.resolveSchema(schema)
	if schema == nil {
		return nil
	}
	if issues, handled := r.validateCombinators(path, schema, value); handled {
		return issues
	}
	if typ := effectiveType(schema); typ != "" {
		if !matchesType(value, typ) {
			return []ValidationIssue{typeIssue(path, typ, value)}
		}
	}
	switch effectiveType(schema) {
	case "object":
		body, _ := value.(map[string]any)
		return r.validateObject(path, schema, body)
	case "array":
		values, _ := value.([]any)
		return r.validateArray(path, schema, values)
	case "string":
		return validateString(path, schema, value)
	case "integer", "number":
		return validateNumber(path, schema, value)
	default:
		return validateCommon(path, schema, value)
	}
}

func (r *Registry) validateCombinators(path string, schema map[string]any, value any) ([]ValidationIssue, bool) {
	if branches := schemaList(schema["allOf"]); len(branches) > 0 {
		issues := r.validateSchema(path, schemaWithout(schema, "allOf"), value)
		for _, branch := range branches {
			issues = append(issues, r.validateSchema(path, branch, value)...)
		}
		return issues, true
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		branches := schemaList(schema[key])
		if len(branches) == 0 {
			continue
		}
		matches := 0
		for _, branch := range branches {
			if len(r.validateSchema(path, branch, value)) == 0 {
				matches++
			}
		}
		if key == "anyOf" && matches > 0 {
			return nil, true
		}
		if key == "oneOf" && matches == 1 {
			return nil, true
		}
		return []ValidationIssue{{
			Path:     path,
			Code:     key,
			Message:  fmt.Sprintf("%s must match %s schema", path, key),
			Expected: key,
			Actual:   value,
		}}, true
	}
	return nil, false
}

func schemaWithout(schema map[string]any, key string) map[string]any {
	out := make(map[string]any, len(schema))
	for k, value := range schema {
		if k != key {
			out[k] = value
		}
	}
	return out
}

func (r *Registry) validateObject(path string, schema map[string]any, body map[string]any) []ValidationIssue {
	var issues []ValidationIssue
	props, _ := schema["properties"].(map[string]any)
	required := requiredSet(schema["required"])
	for key := range required {
		if _, ok := body[key]; !ok {
			issues = append(issues, ValidationIssue{
				Path:    joinPath(path, key),
				Code:    "required",
				Message: fmt.Sprintf("%s is required", joinPath(path, key)),
			})
		}
	}
	allowAdditional, _ := schema["additionalProperties"].(bool)
	for key, value := range body {
		prop := schemaMap(props[key])
		if prop == nil {
			if !allowAdditional {
				issues = append(issues, ValidationIssue{
					Path:    joinPath(path, key),
					Code:    "unknown_field",
					Message: fmt.Sprintf("%s is not allowed", joinPath(path, key)),
					Actual:  value,
				})
			}
			continue
		}
		issues = append(issues, r.validateSchema(joinPath(path, key), prop, value)...)
	}
	return issues
}

func (r *Registry) validateArray(path string, schema map[string]any, values []any) []ValidationIssue {
	var issues []ValidationIssue
	if min, ok := number(schema["minItems"]); ok && float64(len(values)) < min {
		issues = append(issues, minIssue(path, "min_items", min, values))
	}
	if max, ok := number(schema["maxItems"]); ok && float64(len(values)) > max {
		issues = append(issues, maxIssue(path, "max_items", max, values))
	}
	itemSchema := schemaMap(schema["items"])
	for i, value := range values {
		issues = append(issues, r.validateSchema(fmt.Sprintf("%s[%d]", path, i), itemSchema, value)...)
	}
	return append(issues, validateCommon(path, schema, values)...)
}

func validateString(path string, schema map[string]any, value any) []ValidationIssue {
	text, _ := value.(string)
	var issues []ValidationIssue
	if min, ok := number(schema["minLength"]); ok && float64(len(text)) < min {
		issues = append(issues, minIssue(path, "min_length", min, value))
	}
	if max, ok := number(schema["maxLength"]); ok && float64(len(text)) > max {
		issues = append(issues, maxIssue(path, "max_length", max, value))
	}
	return append(issues, validateCommon(path, schema, value)...)
}

func validateNumber(path string, schema map[string]any, value any) []ValidationIssue {
	got, _ := number(value)
	var issues []ValidationIssue
	if min, ok := number(schema["minimum"]); ok && got < min {
		issues = append(issues, minIssue(path, "minimum", min, value))
	}
	if max, ok := number(schema["maximum"]); ok && got > max {
		issues = append(issues, maxIssue(path, "maximum", max, value))
	}
	return append(issues, validateCommon(path, schema, value)...)
}

func validateCommon(path string, schema map[string]any, value any) []ValidationIssue {
	if values := enumStrings(schema["enum"]); len(values) > 0 && !containsEnum(values, value) {
		return []ValidationIssue{{
			Path:          path,
			Code:          "enum",
			Message:       fmt.Sprintf("%s has unsupported value %v", path, value),
			Actual:        value,
			AllowedValues: values,
		}}
	}
	return nil
}

func requiredSet(value any) map[string]bool {
	required := map[string]bool{}
	items, _ := value.([]any)
	for _, item := range items {
		if key, ok := item.(string); ok {
			required[key] = true
		}
	}
	return required
}
