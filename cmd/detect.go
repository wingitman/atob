package cmd

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// reUUID matches a standard UUID (v1–v5).
var reUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// reBase64 matches a base64-encoded string (standard or URL-safe, with optional padding).
var reBase64 = regexp.MustCompile(`^[A-Za-z0-9+/\-_]+=*$`)

// reHex matches a pure hex string (even length, optional 0x prefix).
var reHex = regexp.MustCompile(`(?i)^(0x)?[0-9a-f]+$`)

// rePercent matches URL percent-encoded sequences.
var rePercent = regexp.MustCompile(`%[0-9A-Fa-f]{2}`)

// reHTMLEntity matches HTML entities like &amp; &lt; &#123; &#x1a;
var reHTMLEntity = regexp.MustCompile(`&([a-zA-Z]+|#[0-9]+|#x[0-9a-fA-F]+);`)

// Detect attempts to identify the type of the input string.
// Returns a canonical type constant (see types.go) or an error when the
// input is genuinely ambiguous and the caller should ask for an explicit type.
func Detect(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return TypeText, nil
	}

	// 1. JSON — starts with { or [
	if (strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")) && json.Valid([]byte(s)) {
		return TypeJSON, nil
	}

	// 2. XML — starts with < and parses as XML
	if strings.HasPrefix(s, "<") {
		if isXML(s) {
			return TypeXML, nil
		}
		// Starts with < but isn't valid XML — could be HTML entities or broken XML;
		// fall through to entity check
	}

	// 3. UUID — exact match before any numeric checks
	if reUUID.MatchString(s) {
		return TypeUUID, nil
	}

	// 4. YAML — contains ": " or starts with "---" or has "- " list items,
	//    and is NOT a single word (which would just be plain text)
	if isYAML(s) {
		return TypeYAML, nil
	}

	// 5. TOML — contains " = " assignment and looks like TOML sections
	if isTOML(s) {
		return TypeTOML, nil
	}

	// 6. CSV — multiple lines with consistent comma-separated columns
	if isCSV(s) {
		return TypeCSV, nil
	}

	// 7. Pure binary — only 0 and 1, at least 4 chars (avoids "0", "1", "10" etc.)
	if isPureChars(s, "01") && len(s) >= 4 {
		return TypeBinary, nil
	}

	// 8. URL-encoded — contains %XX sequences
	if rePercent.MatchString(s) {
		return TypeURL, nil
	}

	// 9. HTML entities
	if reHTMLEntity.MatchString(s) {
		return TypeHTML, nil
	}

	// 10. Numeric ambiguity detection — only attempt if the string looks purely numeric
	if isPureChars(s, "0123456789abcdefABCDEF") && !strings.ContainsAny(s, " \t\n") {
		return detectNumeric(s)
	}

	// 11. Base64 — matches charset, length divisible by 4 (with padding), longer strings
	if reBase64.MatchString(s) && len(s)%4 == 0 && len(s) >= 8 {
		return TypeBase64, nil
	}

	// 12. Case style detection
	if t, ok := detectCase(s); ok {
		return t, nil
	}

	// 13. Epoch — pure digit string of plausible Unix timestamp length (8–11 digits)
	if isPureChars(s, "0123456789") && len(s) >= 8 && len(s) <= 11 {
		return TypeEpoch, nil
	}

	return TypeText, nil
}

// detectNumeric resolves ambiguity among hex / octal / binary / decimal / base64.
// Called when the string contains only hex-compatible chars.
func detectNumeric(s string) (string, error) {
	isHexOnly := reHex.MatchString(s) // true even for pure decimal strings
	isAllDecimal := isPureChars(s, "0123456789")
	isAllOctal := isPureChars(s, "01234567")
	isAllBinary := isPureChars(s, "01")
	isHexChars := !isAllDecimal && isPureChars(strings.ToLower(s), "0123456789abcdef")

	// Pure decimal: could be decimal number or epoch
	if isAllDecimal {
		// epoch range 8–11 digits (year ~2001–2286)
		if len(s) >= 8 && len(s) <= 11 {
			return "", fmt.Errorf(
				"input %q is ambiguous (could be a decimal number or a Unix epoch timestamp)\n"+
					"hint: use an explicit from type:\n"+
					"  atob %q epoch text\n"+
					"  atob %q text binary",
				s, s, s,
			)
		}
		return TypeText, nil
	}

	// Has hex chars (a-f) but is all-octal-safe digits — unlikely but possible
	if isHexChars && !isAllOctal && !isAllBinary {
		// unambiguously hex (contains a-f letters)
		return TypeHex, nil
	}

	// Only octal digits but not binary — could be octal or decimal
	if isAllOctal && !isAllBinary && !isHexChars {
		return "", fmt.Errorf(
			"input %q is ambiguous (could be octal or decimal)\n"+
				"hint: use an explicit from type:\n"+
				"  atob %q octal text\n"+
				"  atob %q text binary",
			s, s, s,
		)
	}

	_ = isHexOnly
	return TypeText, nil
}

// isXML returns true if s parses as valid XML.
func isXML(s string) bool {
	d := xml.NewDecoder(strings.NewReader(s))
	hasElement := false
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		if _, ok := tok.(xml.StartElement); ok {
			hasElement = true
		}
	}
	return hasElement
}

// isYAML returns true if s looks like YAML (but not JSON or a single word).
func isYAML(s string) bool {
	if json.Valid([]byte(s)) {
		return false
	}
	if strings.HasPrefix(s, "---") {
		return true
	}
	lines := strings.Split(s, "\n")
	yamlLines := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "---" || trimmed == "..." || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, ": ") || strings.HasPrefix(trimmed, "- ") {
			yamlLines++
		}
	}
	// Require at least one YAML line; for multi-line input require 2+ to avoid
	// false positives on things like "Content-Type: application/json".
	if len(lines) == 1 {
		return yamlLines >= 1
	}
	return yamlLines >= 2
}

// isTOML returns true if s looks like TOML.
func isTOML(s string) bool {
	if json.Valid([]byte(s)) {
		return false
	}
	lines := strings.Split(s, "\n")
	hasAssignment := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// TOML section header
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			return true
		}
		// TOML key = value
		if strings.Contains(trimmed, " = ") {
			hasAssignment = true
		}
	}
	return hasAssignment
}

// isCSV returns true if s looks like CSV data (multiple rows, consistent columns).
func isCSV(s string) bool {
	if json.Valid([]byte(s)) {
		return false
	}
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) < 2 {
		return false
	}
	// Count columns in header row
	headerCols := strings.Count(lines[0], ",") + 1
	if headerCols < 2 {
		return false
	}
	// At least one data row should have the same number of columns
	for _, line := range lines[1:] {
		if strings.Count(line, ",")+1 == headerCols {
			return true
		}
	}
	return false
}

// isPureChars returns true if every character in s is in the allowed set.
func isPureChars(s, allowed string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(allowed, r) {
			return false
		}
	}
	return true
}

// detectCase identifies camelCase, PascalCase, snake_case, and kebab-case.
func detectCase(s string) (string, bool) {
	// Must be a single "word-like" token — no spaces
	if strings.ContainsAny(s, " \t\n") {
		return "", false
	}

	hasUnderscore := strings.Contains(s, "_")
	hasDash := strings.Contains(s, "-")

	if hasUnderscore && !hasDash {
		if strings.ToUpper(s) == s {
			return TypeScreamingSnake, true
		}
		return TypeSnake, true
	}

	if hasDash && !hasUnderscore {
		if strings.ToUpper(s) == s {
			return TypeScreamingKebab, true
		}
		return TypeKebab, true
	}

	// No separators — look for mixed case
	if !hasUnderscore && !hasDash {
		hasMidUpper := false
		for i, r := range s {
			if i > 0 && unicode.IsUpper(r) {
				hasMidUpper = true
				break
			}
		}
		if hasMidUpper {
			if unicode.IsUpper(rune(s[0])) {
				return TypePascal, true
			}
			return TypeCamel, true
		}
	}

	return "", false
}
