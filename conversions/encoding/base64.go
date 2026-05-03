package encoding

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(base64Encode{})
	conversions.Register(base64Decode{})
}

// --- encode ---

type base64Encode struct{}

func (base64Encode) Name() string        { return "base64-encode" }
func (base64Encode) Category() string    { return "encoding" }
func (base64Encode) Description() string { return "Encode text to Base64" }

func (base64Encode) Convert(input string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(strings.TrimRight(input, "\n"))), nil
}

// --- decode ---

type base64Decode struct{}

func (base64Decode) Name() string        { return "base64-decode" }
func (base64Decode) Category() string    { return "encoding" }
func (base64Decode) Description() string { return "Decode Base64 to text" }

func (base64Decode) Convert(input string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input))
	if err != nil {
		// try URL encoding variant
		b, err = base64.URLEncoding.DecodeString(strings.TrimSpace(input))
		if err != nil {
			return "", fmt.Errorf("invalid base64 input: %w", err)
		}
	}
	return string(b), nil
}
