package formats

import (
	"bytes"
	"encoding/json"
	"fmt"

	internaltoml "github.com/wingitman/atob/conversions/internal/toml"
	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(jsonToTOML{})
	conversions.Register(tomlToJSON{})
}

type jsonToTOML struct{}

func (jsonToTOML) Name() string        { return "json-toml" }
func (jsonToTOML) Category() string    { return "formats" }
func (jsonToTOML) Description() string { return "Convert JSON to TOML" }

func (jsonToTOML) Convert(input string) (string, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	var buf bytes.Buffer
	if err := internaltoml.Encode(&buf, data); err != nil {
		return "", fmt.Errorf("TOML encode error: %w", err)
	}
	return buf.String(), nil
}

type tomlToJSON struct{}

func (tomlToJSON) Name() string        { return "toml-json" }
func (tomlToJSON) Category() string    { return "formats" }
func (tomlToJSON) Description() string { return "Convert TOML to JSON" }

func (tomlToJSON) Convert(input string) (string, error) {
	var data map[string]any
	if err := internaltoml.Decode(input, &data); err != nil {
		return "", fmt.Errorf("invalid TOML: %w", err)
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON marshal error: %w", err)
	}
	return string(out), nil
}
