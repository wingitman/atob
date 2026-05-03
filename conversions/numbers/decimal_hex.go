package numbers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(decToHex{})
	conversions.Register(hexToDec{})
}

type decToHex struct{}

func (decToHex) Name() string        { return "dec-hex" }
func (decToHex) Category() string    { return "numbers" }
func (decToHex) Description() string { return "Convert decimal integer to hexadecimal" }

func (decToHex) Convert(input string) (string, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid decimal integer: %w", err)
	}
	return strconv.FormatInt(n, 16), nil
}

type hexToDec struct{}

func (hexToDec) Name() string        { return "hex-dec" }
func (hexToDec) Category() string    { return "numbers" }
func (hexToDec) Description() string { return "Convert hexadecimal to decimal integer" }

func (hexToDec) Convert(input string) (string, error) {
	cleaned := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "0x"))
	n, err := strconv.ParseInt(cleaned, 16, 64)
	if err != nil {
		return "", fmt.Errorf("invalid hexadecimal integer: %w", err)
	}
	return strconv.FormatInt(n, 10), nil
}
