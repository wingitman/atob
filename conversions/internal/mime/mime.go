// Package mime detects MIME types from the leading bytes (magic numbers) of
// binary data. It is intentionally minimal — it only covers the formats that
// the binary inspector needs to route correctly.
package mime

import "bytes"

// Detect returns a MIME type string for data based on magic bytes.
// It never returns an empty string; unknown data returns "application/octet-stream".
func Detect(data []byte) string {
	if len(data) == 0 {
		return "application/octet-stream"
	}

	switch {
	// ── Executable / binary formats ──────────────────────────────────────────

	// ELF
	case has(data, 0, 0x7f, 'E', 'L', 'F'):
		return "application/x-elf"

	// PE (MZ header)
	case has(data, 0, 'M', 'Z'):
		return "application/x-dosexec"

	// Mach-O (all four magic values)
	case has(data, 0, 0xfe, 0xed, 0xfa, 0xce), // 32-bit BE
		has(data, 0, 0xce, 0xfa, 0xed, 0xfe), // 32-bit LE
		has(data, 0, 0xfe, 0xed, 0xfa, 0xcf), // 64-bit BE
		has(data, 0, 0xcf, 0xfa, 0xed, 0xfe), // 64-bit LE
		has(data, 0, 0xca, 0xfe, 0xba, 0xbe): // fat binary
		return "application/x-mach-binary"

	// ── Archives ─────────────────────────────────────────────────────────────

	// ZIP (also .jar, .docx, .xlsx, .apk …)
	case has(data, 0, 'P', 'K', 0x03, 0x04):
		return "application/zip"

	// TAR magic at offset 257
	case len(data) > 262 &&
		(bytes.Equal(data[257:262], []byte("ustar")) ||
			bytes.Equal(data[257:265], []byte("ustar  \x00"))):
		return "application/x-tar"

	// GZip
	case has(data, 0, 0x1f, 0x8b):
		return "application/gzip"

	// BZip2
	case has(data, 0, 'B', 'Z', 'h'):
		return "application/x-bzip2"

	// XZ
	case has(data, 0, 0xfd, '7', 'z', 'X', 'Z', 0x00):
		return "application/x-xz"

	// 7z
	case has(data, 0, '7', 'z', 0xbc, 0xaf, 0x27, 0x1c):
		return "application/x-7z-compressed"

	// ── Images ───────────────────────────────────────────────────────────────

	// JPEG
	case has(data, 0, 0xff, 0xd8, 0xff):
		return "image/jpeg"

	// PNG
	case has(data, 0, 0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a):
		return "image/png"

	// GIF
	case has(data, 0, 'G', 'I', 'F', '8'):
		return "image/gif"

	// WebP: RIFF????WEBP
	case len(data) >= 12 &&
		bytes.Equal(data[0:4], []byte("RIFF")) &&
		bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp"

	// TIFF (little-endian and big-endian)
	case has(data, 0, 'I', 'I', 0x2a, 0x00),
		has(data, 0, 'M', 'M', 0x00, 0x2a):
		return "image/tiff"

	// BMP
	case has(data, 0, 'B', 'M'):
		return "image/bmp"

	// ICO
	case has(data, 0, 0x00, 0x00, 0x01, 0x00):
		return "image/x-icon"

	// ── Documents / data ─────────────────────────────────────────────────────

	// PDF
	case has(data, 0, '%', 'P', 'D', 'F', '-'):
		return "application/pdf"

	// SQLite
	case has(data, 0, 'S', 'Q', 'L', 'i', 't', 'e', ' ', 'f', 'o', 'r', 'm', 'a', 't', ' ', '3'):
		return "application/x-sqlite3"

	// CBOR has no universal magic; MIME hint only from Content-Type metadata.
	// MessagePack likewise. Both fall through to octet-stream.

	default:
		return "application/octet-stream"
	}
}

// has returns true if data starting at offset contains exactly the bytes in magic.
func has(data []byte, offset int, magic ...byte) bool {
	end := offset + len(magic)
	if end > len(data) {
		return false
	}
	return bytes.Equal(data[offset:end], magic)
}
