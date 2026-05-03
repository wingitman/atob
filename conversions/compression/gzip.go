package compression

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(gzipCompress{})
	conversions.Register(gzipDecompress{})
}

// gzip output is binary, so we Base64-encode it for safe terminal transport.

type gzipCompress struct{}

func (gzipCompress) Name() string        { return "gzip-compress" }
func (gzipCompress) Category() string    { return "compression" }
func (gzipCompress) Description() string { return "Gzip-compress text and Base64-encode the result" }

func (gzipCompress) Convert(input string) (string, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(strings.TrimRight(input, "\n"))); err != nil {
		return "", fmt.Errorf("gzip write error: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("gzip close error: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

type gzipDecompress struct{}

func (gzipDecompress) Name() string        { return "gzip-decompress" }
func (gzipDecompress) Category() string    { return "compression" }
func (gzipDecompress) Description() string { return "Base64-decode then gzip-decompress to text" }

func (gzipDecompress) Convert(input string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input))
	if err != nil {
		return "", fmt.Errorf("invalid base64 input: %w", err)
	}
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("not a valid gzip stream: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("gzip decompress error: %w", err)
	}
	return string(out), nil
}
