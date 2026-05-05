package binary

import (
	"encoding/json"
	"fmt"

	internalcbor "github.com/wingitman/atob/conversions/internal/cbor"
)

func inspectCBOR(data []byte) (string, error) {
	var v any
	if err := internalcbor.Unmarshal(data, &v); err != nil {
		return "", fmt.Errorf("invalid CBOR data: %w", err)
	}
	v = normaliseKeys(v) // reuse from inspect_msgpack.go
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON marshal error: %w", err)
	}
	return string(out) + "\n", nil
}
