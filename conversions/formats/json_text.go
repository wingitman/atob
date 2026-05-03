package formats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(jsonPretty{})
	conversions.Register(jsonMinify{})
}

type jsonPretty struct{}

func (jsonPretty) Name() string        { return "json-pretty" }
func (jsonPretty) Category() string    { return "formats" }
func (jsonPretty) Description() string { return "Pretty-print / format JSON with indentation" }

func (jsonPretty) Convert(input string) (string, error) {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type jsonMinify struct{}

func (jsonMinify) Name() string        { return "json-minify" }
func (jsonMinify) Category() string    { return "formats" }
func (jsonMinify) Description() string { return "Minify / compact JSON (remove whitespace)" }

func (jsonMinify) Convert(input string) (string, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(strings.TrimSpace(input))); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	return buf.String(), nil
}
