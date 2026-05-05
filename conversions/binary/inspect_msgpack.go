package binary

import (
	"encoding/json"
	"fmt"

	internalmsgpack "github.com/wingitman/atob/conversions/internal/msgpack"
)

func inspectMsgpack(data []byte) (string, error) {
	var v any
	if err := internalmsgpack.Unmarshal(data, &v); err != nil {
		return "", fmt.Errorf("invalid MessagePack data: %w", err)
	}
	// Normalise map keys to strings for JSON marshalling
	v = normaliseKeys(v)
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON marshal error: %w", err)
	}
	return string(out) + "\n", nil
}

// normaliseKeys recursively converts map[any]any (which msgpack may produce)
// to map[string]any so json.Marshal can handle it.
func normaliseKeys(v any) any {
	switch t := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprintf("%v", k)] = normaliseKeys(val)
		}
		return out
	case map[string]any:
		for k, val := range t {
			t[k] = normaliseKeys(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = normaliseKeys(val)
		}
		return t
	}
	return v
}
