package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(uuidGenerate{})
	conversions.Register(uuidValidate{})
}

type uuidGenerate struct{}

func (uuidGenerate) Name() string        { return "uuid-generate" }
func (uuidGenerate) Category() string    { return "identity" }
func (uuidGenerate) Description() string { return "Generate a new random UUID v4 (ignores input)" }

func (uuidGenerate) Convert(_ string) (string, error) {
	id, err := newUUIDv4()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	return id, nil
}

type uuidValidate struct{}

func (uuidValidate) Name() string        { return "uuid-validate" }
func (uuidValidate) Category() string    { return "identity" }
func (uuidValidate) Description() string { return "Validate a UUID and report its version" }

func (uuidValidate) Convert(input string) (string, error) {
	id, version, err := parseUUID(strings.TrimSpace(input))
	if err != nil {
		return "", fmt.Errorf("invalid UUID: %w", err)
	}
	return fmt.Sprintf("valid UUID v%d: %s", version, id), nil
}

// newUUIDv4 generates a random UUID v4 using crypto/rand.
// Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
// where y is one of 8, 9, a, or b (variant bits 10xxxxxx).
func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// Set version to 4 (bits 12–15 of time_hi_and_version)
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant to 10xxxxxx (RFC 4122)
	b[8] = (b[8] & 0x3f) | 0x80
	return formatUUID(b), nil
}

// formatUUID formats 16 bytes as a standard UUID string.
func formatUUID(b [16]byte) string {
	var buf [36]byte
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf[:])
}

// parseUUID validates a UUID string (with or without hyphens) and returns
// the canonical form plus the version number.
func parseUUID(s string) (canonical string, version int, err error) {
	// Normalise: remove hyphens then check length
	clean := strings.ReplaceAll(s, "-", "")
	if len(clean) != 32 {
		return "", 0, fmt.Errorf("expected 32 hex digits, got %d", len(clean))
	}

	var b [16]byte
	if _, err := hex.Decode(b[:], []byte(clean)); err != nil {
		return "", 0, fmt.Errorf("not valid hex: %w", err)
	}

	// Version is the high nibble of byte 6.
	version = int(b[6] >> 4)

	// Validate variant for versions 1–5 and 6–8 (RFC 4122 / new UUIDs).
	// Variant bits: byte 8 high bits should be 10xxxxxx (0x80–0xbf).
	// We accept any UUID and just report version without enforcing variant.

	return formatUUID(b), version, nil
}
