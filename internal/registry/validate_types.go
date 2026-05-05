package registry

import "fmt"

func effectiveType(schema map[string]any) string {
	if typ, _ := schema["type"].(string); typ != "" {
		return typ
	}
	if _, ok := schema["properties"].(map[string]any); ok {
		return "object"
	}
	if _, ok := schema["items"].(map[string]any); ok {
		return "array"
	}
	return ""
}

func matchesType(value any, typ string) bool {
	if value == nil {
		return typ == "null"
	}
	switch typ {
	case "null":
		return value == nil
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := number(value)
		return ok
	case "integer":
		return isInteger(value)
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

func typeIssue(path string, expected string, actual any) ValidationIssue {
	return ValidationIssue{
		Path:     path,
		Code:     "type",
		Message:  fmt.Sprintf("%s must be %s", path, expected),
		Expected: expected,
		Actual:   actual,
	}
}

func minIssue(path, code string, min float64, actual any) ValidationIssue {
	return ValidationIssue{
		Path:    path,
		Code:    code,
		Message: fmt.Sprintf("%s must be >= %v", path, min),
		Actual:  actual,
		Minimum: &min,
	}
}

func maxIssue(path, code string, max float64, actual any) ValidationIssue {
	return ValidationIssue{
		Path:    path,
		Code:    code,
		Message: fmt.Sprintf("%s must be <= %v", path, max),
		Actual:  actual,
		Maximum: &max,
	}
}

func containsEnum(values []string, value any) bool {
	for _, candidate := range values {
		if candidate == fmt.Sprint(value) {
			return true
		}
	}
	return false
}

func joinPath(base, key string) string {
	if base == "" || base == "$" {
		return "$." + key
	}
	return base + "." + key
}

func sortIssues(issues []ValidationIssue) {
	for i := range issues {
		for j := i + 1; j < len(issues); j++ {
			if issues[j].Path < issues[i].Path ||
				(issues[j].Path == issues[i].Path && issues[j].Code < issues[i].Code) {
				issues[i], issues[j] = issues[j], issues[i]
			}
		}
	}
}
