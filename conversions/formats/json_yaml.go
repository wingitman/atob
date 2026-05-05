package formats

import (
	"encoding/json"
	"fmt"

	internalyaml "github.com/wingitman/atob/conversions/internal/yaml"
	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(jsonToYAML{})
	conversions.Register(yamlToJSON{})
}

type jsonToYAML struct{}

func (jsonToYAML) Name() string        { return "json-yaml" }
func (jsonToYAML) Category() string    { return "formats" }
func (jsonToYAML) Description() string { return "Convert JSON to YAML" }

func (jsonToYAML) Convert(input string) (string, error) {
	var data any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	out, err := internalyaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("YAML marshal error: %w", err)
	}
	return string(out), nil
}

type yamlToJSON struct{}

func (yamlToJSON) Name() string        { return "yaml-json" }
func (yamlToJSON) Category() string    { return "formats" }
func (yamlToJSON) Description() string { return "Convert YAML to JSON" }

func (yamlToJSON) Convert(input string) (string, error) {
	var data any
	if err := internalyaml.Unmarshal([]byte(input), &data); err != nil {
		return "", fmt.Errorf("invalid YAML: %w", err)
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON marshal error: %w", err)
	}
	return string(out), nil
}
