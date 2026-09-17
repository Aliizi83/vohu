package tools

import "fmt"

// AccessLevelWrite is what actually doing something over a stored
// connection requires — running a command, deploying a binary — even
// something read-only in intent still changes state on the remote
// system (a process runs, output is produced), so every tool that acts
// against a connection gates at Write, not the bare Read that merely
// listing connections requires.
const AccessLevelWrite = "write"

// ParseUintArg handles the float64 shape encoding/json produces for
// numbers decoded into map[string]any — which is exactly how a tool
// call's arguments arrive from every provider in this codebase.
func ParseUintArg(v any) (uint, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint(n), true
	default:
		return 0, false
	}
}

func ParseStringArrayArg(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}

	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("args must be an array")
	}

	result := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("all command arguments must be strings")
		}
		result = append(result, s)
	}

	return result, nil
}
