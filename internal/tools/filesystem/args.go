package filesystem

import "fmt"

// optionalInt reads an optional integer argument. Tool-call arguments
// arrive as map[string]any decoded from JSON by whichever provider SDK
// parsed the model's response — numbers most commonly land as float64,
// but this accepts int and int64 too rather than assuming one shape.
func optionalInt(args map[string]any, key string) (int, bool, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return 0, false, nil
	}

	switch v := raw.(type) {
	case float64:
		return int(v), true, nil
	case int:
		return v, true, nil
	case int64:
		return int(v), true, nil
	default:
		return 0, false, fmt.Errorf("%q must be a number", key)
	}
}

func requiredString(args map[string]any, key string) (string, error) {
	raw, exists := args[key]
	if !exists {
		return "", fmt.Errorf("%q is required", key)
	}

	s, ok := raw.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("%q is required", key)
	}

	return s, nil
}

func optionalBool(args map[string]any, key string) bool {
	raw, exists := args[key]
	if !exists || raw == nil {
		return false
	}
	b, _ := raw.(bool)
	return b
}
