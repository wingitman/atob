package numbers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(decToOct{})
	conversions.Register(octToDec{})
}

type decToOct struct{}

func (decToOct) Name() string        { return "dec-oct" }
func (decToOct) Category() string    { return "numbers" }
func (decToOct) Description() string { return "Convert decimal integer to octal" }

func (decToOct) Convert(input string) (string, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid decimal integer: %w", err)
	}
	return strconv.FormatInt(n, 8), nil
}

type octToDec struct{}

func (octToDec) Name() string        { return "oct-dec" }
func (octToDec) Category() string    { return "numbers" }
func (octToDec) Description() string { return "Convert octal to decimal integer" }

func (octToDec) Convert(input string) (string, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(input), 8, 64)
	if err != nil {
		return "", fmt.Errorf("invalid octal integer: %w", err)
	}
	return strconv.FormatInt(n, 10), nil
}
