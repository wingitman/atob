package encoding

import (
	"html"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(htmlEncode{})
	conversions.Register(htmlDecode{})
}

type htmlEncode struct{}

func (htmlEncode) Name() string        { return "html-encode" }
func (htmlEncode) Category() string    { return "encoding" }
func (htmlEncode) Description() string { return "Encode special characters to HTML entities" }

func (htmlEncode) Convert(input string) (string, error) {
	return html.EscapeString(strings.TrimRight(input, "\n")), nil
}

type htmlDecode struct{}

func (htmlDecode) Name() string        { return "html-decode" }
func (htmlDecode) Category() string    { return "encoding" }
func (htmlDecode) Description() string { return "Decode HTML entities to text" }

func (htmlDecode) Convert(input string) (string, error) {
	return html.UnescapeString(strings.TrimSpace(input)), nil
}
