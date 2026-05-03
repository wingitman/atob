package binary

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.RegisterBinary(stringsExtract{})
}

type stringsExtract struct{}

func (stringsExtract) Name() string        { return "strings" }
func (stringsExtract) Category() string    { return "binary" }
func (stringsExtract) Description() string { return "Extract printable strings from binary data" }

const minStringLen = 4

func (stringsExtract) ConvertBytes(data []byte) (string, error) {
	var results []string
	var current strings.Builder

	flush := func() {
		if current.Len() >= minStringLen {
			results = append(results, current.String())
		}
		current.Reset()
	}

	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError && size > 1 {
			// Valid multi-byte UTF-8 rune
			if unicode.IsPrint(r) || r == '\t' {
				current.WriteRune(r)
				i += size
				continue
			}
		}
		// Single byte
		b := data[i]
		if b >= 0x20 && b < 0x7f {
			// Printable ASCII
			current.WriteByte(b)
		} else if b == '\t' || b == '\r' || b == '\n' {
			// Allow whitespace inside a string but not at the start
			if current.Len() > 0 {
				current.WriteByte(b)
			} else {
				flush()
			}
		} else {
			flush()
		}
		i++
	}
	flush()

	if len(results) == 0 {
		return "(no printable strings found)\n", nil
	}
	return strings.Join(results, "\n") + "\n", nil
}
