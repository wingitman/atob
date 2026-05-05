// Package binary implements converters that operate on raw binary input.
// Each converter implements conversions.BinaryConverter and is registered
// via init() so the cmd layer can look it up by name.
package binary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	internalmime "github.com/wingitman/atob/conversions/internal/mime"
	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.RegisterBinary(inspect{})
}

// inspect is the universal binary inspector. It auto-detects the file type
// from magic bytes / MIME type and routes to the appropriate format handler.
type inspect struct{}

func (inspect) Name() string        { return "inspect" }
func (inspect) Category() string    { return "binary" }
func (inspect) Description() string { return "Auto-detect binary format and return JSON metadata" }

func (inspect) ConvertBytes(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty input")
	}

	// Detect MIME type from magic bytes
	mt := internalmime.Detect(data) // e.g. "application/x-elf", "image/jpeg"

	// Route by magic bytes first (more reliable than MIME for executables)
	switch {
	// ── ELF ──────────────────────────────────────────────────────────────────
	case isELF(data):
		return inspectELF(data)

	// ── PE (Windows .exe / .dll) ──────────────────────────────────────────────
	case isPE(data):
		return inspectPE(data)

	// ── Mach-O ───────────────────────────────────────────────────────────────
	case isMachO(data):
		return inspectMachO(data)

	// ── ZIP (also .jar, .docx, .xlsx, .apk, etc.) ────────────────────────────
	case isZIP(data):
		return inspectZIP(data)

	// ── TAR (plain) ───────────────────────────────────────────────────────────
	case isTAR(data):
		return inspectTAR(data, "none")

	// ── TAR.GZ ────────────────────────────────────────────────────────────────
	case isGzip(data) && containsTAR(data):
		return inspectTAR(data, "gzip")

	// ── GZIP (non-tar) ────────────────────────────────────────────────────────
	case isGzip(data):
		return genericInfo(data, "GZIP", mt)

	// ── BZIP2 ─────────────────────────────────────────────────────────────────
	case isBzip2(data) && containsTARBzip2(data):
		return inspectTAR(data, "bzip2")

	// ── Images ────────────────────────────────────────────────────────────────
	case strings.HasPrefix(mt, "image/"):
		return inspectImage(data, mt)

	// ── MessagePack ───────────────────────────────────────────────────────────
	// MessagePack has no universal magic bytes; try to decode it if MIME hints
	case strings.Contains(mt, "msgpack") || strings.Contains(mt, "x-msgpack"):
		return inspectMsgpack(data)

	// ── CBOR ──────────────────────────────────────────────────────────────────
	case strings.Contains(mt, "cbor"):
		return inspectCBOR(data)

	// ── Fallback: generic info ────────────────────────────────────────────────
	default:
		// Last-ditch attempt: try MessagePack and CBOR decode (no magic bytes)
		if out, err := inspectMsgpack(data); err == nil {
			return withTypeWrapper("MessagePack", out)
		}
		if out, err := inspectCBOR(data); err == nil {
			return withTypeWrapper("CBOR", out)
		}
		return genericInfo(data, "unknown", mt)
	}
}

// genericInfo returns basic file information when format is unknown.
func genericInfo(data []byte, fileType, mimeType string) (string, error) {
	info := map[string]any{
		"type":      fileType,
		"mime_type": mimeType,
		"file_size": len(data),
	}
	// First 16 bytes as hex for identification
	preview := data
	if len(preview) > 16 {
		preview = preview[:16]
	}
	hexBytes := make([]string, len(preview))
	for i, b := range preview {
		hexBytes[i] = fmt.Sprintf("%02x", b)
	}
	info["magic_bytes"] = strings.Join(hexBytes, " ")
	out, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

// withTypeWrapper wraps a JSON string with a top-level "decoded_as" field.
func withTypeWrapper(typeName, jsonStr string) (string, error) {
	var inner any
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonStr)), &inner); err != nil {
		return jsonStr, nil
	}
	wrapped := map[string]any{
		"type":    typeName,
		"decoded": inner,
	}
	out, err := json.MarshalIndent(wrapped, "", "  ")
	if err != nil {
		return jsonStr, nil
	}
	return string(out) + "\n", nil
}

// ── magic byte helpers ────────────────────────────────────────────────────────

func isELF(data []byte) bool {
	return len(data) >= 4 &&
		data[0] == 0x7f && data[1] == 'E' && data[2] == 'L' && data[3] == 'F'
}

func isPE(data []byte) bool {
	// MZ header
	if len(data) < 64 || data[0] != 'M' || data[1] != 'Z' {
		return false
	}
	// PE offset at 0x3c
	peOffset := int(data[0x3c]) | int(data[0x3d])<<8 | int(data[0x3e])<<16 | int(data[0x3f])<<24
	if peOffset+4 > len(data) {
		return false
	}
	return data[peOffset] == 'P' && data[peOffset+1] == 'E' &&
		data[peOffset+2] == 0 && data[peOffset+3] == 0
}

func isZIP(data []byte) bool {
	return len(data) >= 4 &&
		data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04
}

func isTAR(data []byte) bool {
	// TAR magic at offset 257: "ustar"
	return len(data) > 262 &&
		(bytes.Equal(data[257:262], []byte("ustar")) ||
			bytes.Equal(data[257:265], []byte("ustar  \x00")))
}

func isGzip(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

func isBzip2(data []byte) bool {
	return len(data) >= 3 && data[0] == 'B' && data[1] == 'Z' && data[2] == 'h'
}

// containsTAR attempts a quick check: decompress just enough to see TAR magic.
// We don't decompress fully; just check if it parses as tar after gzip
func containsTAR(data []byte) bool {
	_, err := inspectTAR(data, "gzip")
	return err == nil
}

func containsTARBzip2(data []byte) bool {
	_, err := inspectTAR(data, "bzip2")
	return err == nil
}
