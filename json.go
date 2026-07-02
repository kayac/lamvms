package lamvms

// omitEmptyValues is based on fujiwara/lambroll's omitEmptyValues.
// https://github.com/fujiwara/lambroll/blob/v1/json.go
func omitEmptyValues(data any) any {
	switch v := data.(type) {
	case map[string]any:
		m := make(map[string]any)
		for key, value := range v {
			cleaned := omitEmptyValues(value)
			if !isEmptyValue(cleaned) {
				m[key] = cleaned
			}
		}
		if len(m) != 0 {
			return m
		}
	case []any:
		a := make([]any, 0, len(v))
		for _, value := range v {
			cleaned := omitEmptyValues(value)
			if !isEmptyValue(cleaned) {
				a = append(a, cleaned)
			}
		}
		if len(a) != 0 {
			return a
		}
	default:
		if !isEmptyValue(v) {
			return v
		}
	}
	return nil
}

func isEmptyValue(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case bool:
		return !v
	case map[string]any:
		return len(v) == 0
	case []any:
		return len(v) == 0
	default:
		return false
	}
}
