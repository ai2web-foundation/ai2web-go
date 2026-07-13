package ai2web

import (
	"fmt"
	"math"
	"reflect"
)

// ValidateSchema validates a value against a JSON-Schema-subset (port of @ai2web/core
// validateSchema): pragmatic (object with typed/required properties, primitives, arrays,
// enum) rather than the whole of JSON Schema. Returns (valid, errors). An empty or absent
// schema accepts anything.
func ValidateSchema(value any, schema any, path string) (bool, []string) {
	errors := []string{}
	sm := toMap(schema)
	if len(sm) == 0 {
		return true, errors
	}

	if declared, _ := sm["type"].(string); declared != "" {
		ok := false
		if declared == "integer" {
			ok = isInteger(value)
		} else {
			ok = schemaTypeOf(value) == declared
		}
		if !ok {
			return false, append(errors, path+": expected "+declared+", got "+schemaTypeOf(value))
		}
	}

	if enum, ok := sm["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			if reflect.DeepEqual(e, value) {
				found = true
				break
			}
		}
		if !found {
			errors = append(errors, path+": value is not one of the allowed options")
		}
	}

	declared, _ := sm["type"].(string)
	if (declared == "object" || (declared == "" && schemaTypeOf(value) == "object")) {
		if obj, ok := value.(map[string]any); ok {
			for _, k := range asStringSlice(sm["required"]) {
				if _, exists := obj[k]; !exists {
					errors = append(errors, path+"."+k+": required")
				}
			}
			for k, sub := range toMap(sm["properties"]) {
				if v, exists := obj[k]; exists {
					_, errs := ValidateSchema(v, sub, path+"."+k)
					errors = append(errors, errs...)
				}
			}
		}
	}

	if (declared == "array" || (declared == "" && schemaTypeOf(value) == "array")) && sm["items"] != nil {
		if arr, ok := value.([]any); ok {
			for i, item := range arr {
				_, errs := ValidateSchema(item, sm["items"], fmt.Sprintf("%s[%d]", path, i))
				errors = append(errors, errs...)
			}
		}
	}

	return len(errors) == 0, errors
}

func schemaTypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64, float32, int, int64, int32:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

func isInteger(v any) bool {
	switch x := v.(type) {
	case int, int64, int32:
		return true
	case float64:
		return x == math.Trunc(x)
	case float32:
		return float64(x) == math.Trunc(float64(x))
	default:
		return false
	}
}

func asStringSlice(v any) []string {
	out := []string{}
	switch xs := v.(type) {
	case []string:
		return xs
	case []any:
		for _, x := range xs {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// actionInputSchema returns the input_schema declared for the named action, or nil.
func actionInputSchema(m Manifest, name string) any {
	acts, ok := m["actions"].([]any)
	if !ok {
		return nil
	}
	for _, a := range acts {
		am := toMap(a)
		if am == nil {
			continue
		}
		if n, _ := am["name"].(string); n == name {
			return am["input_schema"]
		}
	}
	return nil
}
