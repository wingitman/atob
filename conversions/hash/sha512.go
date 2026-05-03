package hash

import (
	"crypto/sha512"
	"encoding/hex"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(sha512Hash{})
}

type sha512Hash struct{}

func (sha512Hash) Name() string        { return "hash-sha512" }
func (sha512Hash) Category() string    { return "hash" }
func (sha512Hash) Description() string { return "Hash text with SHA-512 (hex output)" }

func (sha512Hash) Convert(input string) (string, error) {
	h := sha512.Sum512([]byte(strings.TrimRight(input, "\n")))
	return hex.EncodeToString(h[:]), nil
}
