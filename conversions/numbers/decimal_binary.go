package numbers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(decToBin{})
	conversions.Register(binToDec{})
}

type decToBin struct{}

func (decToBin) Name() string        { return "dec-bin" }
func (decToBin) Category() string    { return "numbers" }
func (decToBin) Description() string { return "Convert decimal integer to binary" }

func (decToBin) Convert(input string) (string, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid decimal integer: %w", err)
	}
	return strconv.FormatInt(n, 2), nil
}

type binToDec struct{}

func (binToDec) Name() string        { return "bin-dec" }
func (binToDec) Category() string    { return "numbers" }
func (binToDec) Description() string { return "Convert binary to decimal integer" }

func (binToDec) Convert(input string) (string, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(input), 2, 64)
	if err != nil {
		return "", fmt.Errorf("invalid binary integer: %w", err)
	}
	return strconv.FormatInt(n, 10), nil
}
