package encoding

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(urlEncode{})
	conversions.Register(urlDecode{})
}

type urlEncode struct{}

func (urlEncode) Name() string        { return "url-encode" }
func (urlEncode) Category() string    { return "encoding" }
func (urlEncode) Description() string { return "URL-encode (percent-encode) text" }

func (urlEncode) Convert(input string) (string, error) {
	return url.QueryEscape(strings.TrimRight(input, "\n")), nil
}

type urlDecode struct{}

func (urlDecode) Name() string        { return "url-decode" }
func (urlDecode) Category() string    { return "encoding" }
func (urlDecode) Description() string { return "Decode URL-encoded (percent-encoded) text" }

func (urlDecode) Convert(input string) (string, error) {
	out, err := url.QueryUnescape(strings.TrimSpace(input))
	if err != nil {
		return "", fmt.Errorf("invalid url-encoded input: %w", err)
	}
	return out, nil
}
