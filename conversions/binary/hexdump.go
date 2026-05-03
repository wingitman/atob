// Package binary implements converters that operate on raw binary input.
package binary

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.RegisterBinary(hexdump{})
}

type hexdump struct{}

func (hexdump) Name() string        { return "hexdump" }
func (hexdump) Category() string    { return "binary" }
func (hexdump) Description() string { return "Pretty hex dump (offset + hex bytes + ASCII panel)" }

func (hexdump) ConvertBytes(data []byte) (string, error) {
	const width = 16
	var sb strings.Builder

	for offset := 0; offset < len(data); offset += width {
		end := offset + width
		if end > len(data) {
			end = len(data)
		}
		row := data[offset:end]

		// offset column
		fmt.Fprintf(&sb, "%08x  ", offset)

		// hex bytes — two groups of 8
		for i := 0; i < width; i++ {
			if i == 8 {
				sb.WriteString(" ")
			}
			if i < len(row) {
				fmt.Fprintf(&sb, "%02x ", row[i])
			} else {
				sb.WriteString("   ")
			}
		}

		// ASCII panel
		sb.WriteString(" |")
		for _, b := range row {
			r := rune(b)
			if b < 0x80 && unicode.IsPrint(r) {
				sb.WriteRune(r)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteString("|\n")
	}

	// final offset marker
	fmt.Fprintf(&sb, "%08x\n", len(data))
	return sb.String(), nil
}
