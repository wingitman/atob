package compression

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(zlibCompress{})
	conversions.Register(zlibDecompress{})
}

type zlibCompress struct{}

func (zlibCompress) Name() string        { return "zlib-compress" }
func (zlibCompress) Category() string    { return "compression" }
func (zlibCompress) Description() string { return "Zlib-compress text and Base64-encode the result" }

func (zlibCompress) Convert(input string) (string, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write([]byte(strings.TrimRight(input, "\n"))); err != nil {
		return "", fmt.Errorf("zlib write error: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("zlib close error: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

type zlibDecompress struct{}

func (zlibDecompress) Name() string        { return "zlib-decompress" }
func (zlibDecompress) Category() string    { return "compression" }
func (zlibDecompress) Description() string { return "Base64-decode then zlib-decompress to text" }

func (zlibDecompress) Convert(input string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input))
	if err != nil {
		return "", fmt.Errorf("invalid base64 input: %w", err)
	}
	r, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("not a valid zlib stream: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("zlib decompress error: %w", err)
	}
	return string(out), nil
}
