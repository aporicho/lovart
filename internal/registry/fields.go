package registry

import (
	"fmt"
	"sort"
)

// RequestFields returns the request fields for a model.
func RequestFields(model string) ([]FieldSpec, error) {
	reg, err := Load()
	if err != nil {
		return nil, err
	}
	return reg.RequestFields(model)
}

// RequestFields returns the request fields for a model from a loaded registry.
func (r *Registry) RequestFields(model string) ([]FieldSpec, error) {
	record, ok := r.Model(model)
	if !ok {
		return nil, fmt.Errorf("registry: model %q not found", model)
	}
	if record.RequestSchema == nil {
		return nil, fmt.Errorf("registry: request schema for model %q is missing", model)
	}
	return fieldsFromSchema(r, record.RequestSchema), nil
}

func fieldsFromSchema(reg *Registry, schema map[string]any) []FieldSpec {
	schema = reg.resolveSchema(schema)
	props, _ := schema["properties"].(map[string]any)
	required := requiredSet(schema["required"])

	fields := make([]FieldSpec, 0, len(props))
	for key, rawProp := range props {
		prop := reg.resolveSchema(schemaMap(rawProp))
		field := FieldSpec{
			Key:         key,
			Type:        displayType(prop),
			Required:    required[key],
			Default:     prop["default"],
			Description: stringField(prop, "description"),
		}
		if values := enumValues(prop); len(values) > 0 {
			field.Enumerated = true
			field.Values = values
		}
		if min, ok := number(prop["minimum"]); ok {
			field.Minimum = &min
		}
		if max, ok := number(prop["maximum"]); ok {
			field.Maximum = &max
		}
		fields = append(fields, field)
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Key < fields[j].Key })
	return fields
}

func displayType(schema map[string]any) string {
	if typ, _ := schema["type"].(string); typ != "" {
		return typ
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		for _, branch := range schemaList(schema[key]) {
			if typ, _ := branch["type"].(string); typ != "" && typ != "null" {
				return typ
			}
		}
	}
	if _, ok := schema["properties"].(map[string]any); ok {
		return "object"
	}
	return ""
}

func stringField(schema map[string]any, key string) string {
	value, _ := schema[key].(string)
	return value
}

func enumValues(schema map[string]any) []string {
	if values := enumStrings(schema["enum"]); len(values) > 0 {
		return values
	}
	items := schemaMap(schema["items"])
	return enumStrings(items["enum"])
}

func enumStrings(value any) []string {
	raw, _ := value.([]any)
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		values = append(values, fmt.Sprint(item))
	}
	sort.Strings(values)
	return values
}
