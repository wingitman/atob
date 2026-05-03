package hash

import (
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(md5Hash{})
}

type md5Hash struct{}

func (md5Hash) Name() string        { return "hash-md5" }
func (md5Hash) Category() string    { return "hash" }
func (md5Hash) Description() string { return "Hash text with MD5 (hex output)" }

func (md5Hash) Convert(input string) (string, error) {
	h := md5.Sum([]byte(strings.TrimRight(input, "\n"))) //nolint:gosec
	return hex.EncodeToString(h[:]), nil
}
