package encoding

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(hexEncode{})
	conversions.Register(hexDecode{})
}

type hexEncode struct{}

func (hexEncode) Name() string        { return "hex-encode" }
func (hexEncode) Category() string    { return "encoding" }
func (hexEncode) Description() string { return "Encode text to hexadecimal" }

func (hexEncode) Convert(input string) (string, error) {
	return hex.EncodeToString([]byte(strings.TrimRight(input, "\n"))), nil
}

type hexDecode struct{}

func (hexDecode) Name() string        { return "hex-decode" }
func (hexDecode) Category() string    { return "encoding" }
func (hexDecode) Description() string { return "Decode hexadecimal to text" }

func (hexDecode) Convert(input string) (string, error) {
	b, err := hex.DecodeString(strings.TrimSpace(input))
	if err != nil {
		return "", fmt.Errorf("invalid hex input: %w", err)
	}
	return string(b), nil
}
