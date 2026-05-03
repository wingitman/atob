package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(sha256Hash{})
}

type sha256Hash struct{}

func (sha256Hash) Name() string        { return "hash-sha256" }
func (sha256Hash) Category() string    { return "hash" }
func (sha256Hash) Description() string { return "Hash text with SHA-256 (hex output)" }

func (sha256Hash) Convert(input string) (string, error) {
	h := sha256.Sum256([]byte(strings.TrimRight(input, "\n")))
	return hex.EncodeToString(h[:]), nil
}
