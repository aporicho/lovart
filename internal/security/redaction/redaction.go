// Package redaction removes secrets from values before they reach logs or JSON output.
package redaction

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

const Replacement = "[REDACTED]"

var stringPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(authorization\s*[:=]\s*bearer\s+)([^\s,;\}"']+)`),
	regexp.MustCompile(`(?i)\b((?:set-)?cookie\s*[:=]\s*)([^\r\n]+)`),
	regexp.MustCompile(`(?i)([?&](?:user)?token=)([^&\s]+)`),
	regexp.MustCompile(`(?i)\b([A-Za-z0-9_.-]*(?:token|csrf|xsrf)[A-Za-z0-9_.-]*\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;&\}]+)`),
	regexp.MustCompile(`(?i)\b(session(?:id|[_-]?id)?\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;&\}]+)`),
	regexp.MustCompile(`(?i)\b((?:session|sid)=)([^;\s,&]+)`),
}

var sensitiveKeyFragments = []string{
	"authorization",
	"cookie",
	"set-cookie",
	"token",
	"csrf",
	"xsrf",
	"session",
}

// RedactString replaces recognized secret-bearing fragments in free-form text.
func RedactString(value string) string {
	redacted := value
	for _, pattern := range stringPatterns {
		redacted = pattern.ReplaceAllString(redacted, `${1}`+Replacement)
	}
	return redacted
}

// RedactError returns an error with the same message after string redaction.
func RedactError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(RedactString(err.Error()))
}

// RedactMap returns a recursively redacted copy of a string-keyed map.
func RedactMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	redacted := make(map[string]any, len(values))
	for key, value := range values {
		if IsSensitiveKey(key) {
			redacted[key] = Replacement
			continue
		}
		redacted[key] = RedactValue(value)
	}
	return redacted
}

// RedactValue returns a recursively redacted copy of strings, maps, and slices.
func RedactValue(value any) any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		return RedactString(typed)
	case error:
		return RedactError(typed)
	case map[string]any:
		return RedactMap(typed)
	case []any:
		redacted := make([]any, len(typed))
		for i, item := range typed {
			redacted[i] = RedactValue(item)
		}
		return redacted
	}
	return redactReflect(reflect.ValueOf(value))
}

// IsSensitiveKey reports whether a structured field name should have its value removed.
func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func redactReflect(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.String:
		return RedactString(value.String())
	case reflect.Map:
		redacted := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			key := fmt.Sprint(iter.Key().Interface())
			if IsSensitiveKey(key) {
				redacted[key] = Replacement
				continue
			}
			redacted[key] = RedactValue(iter.Value().Interface())
		}
		return redacted
	case reflect.Slice, reflect.Array:
		redacted := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			redacted[i] = RedactValue(value.Index(i).Interface())
		}
		return redacted
	default:
		return value.Interface()
	}
}
