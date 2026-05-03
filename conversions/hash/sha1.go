package hash

import (
	"crypto/sha1" //nolint:gosec
	"encoding/hex"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(sha1Hash{})
}

type sha1Hash struct{}

func (sha1Hash) Name() string        { return "hash-sha1" }
func (sha1Hash) Category() string    { return "hash" }
func (sha1Hash) Description() string { return "Hash text with SHA-1 (hex output)" }

func (sha1Hash) Convert(input string) (string, error) {
	h := sha1.Sum([]byte(strings.TrimRight(input, "\n"))) //nolint:gosec
	return hex.EncodeToString(h[:]), nil
}
