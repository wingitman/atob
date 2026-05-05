// Package toml implements a TOML v1.0 parser and encoder.
//
// # Supported types (parser)
//
//   - Strings: basic ("…"), literal ('…'), multi-line basic ("""…"""), multi-line literal ('''…''')
//   - Integers: decimal, hex (0x), octal (0o), binary (0b), with underscores
//   - Floats: with fractional/exponent parts, inf, nan, special sign variants
//   - Booleans: true / false
//   - Datetimes: RFC 3339 offset datetimes, local datetimes, local dates, local times
//   - Arrays: mixed types, nested, multi-line
//   - Inline tables: { key = val, … }
//   - Standard tables: [table]
//   - Arrays of tables: [[array]]
//   - Comments: # to end of line
//
// Decoded datetime values are returned as time.Time (offset/local datetime,
// local date) or as a string (local time, since Go has no time-only type).
//
// # Supported types (encoder)
//
// The encoder accepts map[string]any values where leaf values may be:
// string, bool, int64/int/float64/float32, time.Time, []any, map[string]any.
package toml

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Public API
// ═══════════════════════════════════════════════════════════════════════════════

// Decode parses TOML text and stores the result in v (must be *map[string]any).
func Decode(text string, v *map[string]any) error {
	p := newParser(text)
	m, err := p.parse()
	if err != nil {
		return err
	}
	*v = m
	return nil
}

// Encode writes the map as TOML to w.
func Encode(w io.Writer, v map[string]any) error {
	e := &encoder{w: w}
	return e.encodeTable("", v)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Parser
// ═══════════════════════════════════════════════════════════════════════════════

type parser struct {
	src  []rune
	pos  int
	line int

	// root table
	root    map[string]any
	current map[string]any // points into root (possibly nested)
	path    []string       // current [table] path
	// track which tables/array-of-tables were defined explicitly
	defined    map[string]bool // "a.b" → true if explicitly [a.b]
	arrDefined map[string]bool // "a.b" → true if [[a.b]]
}

func newParser(src string) *parser {
	return &parser{
		src:        []rune(src),
		line:       1,
		root:       make(map[string]any),
		defined:    make(map[string]bool),
		arrDefined: make(map[string]bool),
	}
}

func (p *parser) parse() (map[string]any, error) {
	p.current = p.root
	p.path = nil

	for p.pos < len(p.src) {
		p.skipWS()
		if p.pos >= len(p.src) {
			break
		}
		c := p.src[p.pos]

		switch {
		case c == '\n':
			p.pos++
			p.line++
		case c == '\r':
			p.pos++
			if p.pos < len(p.src) && p.src[p.pos] == '\n' {
				p.pos++
			}
			p.line++
		case c == '#':
			p.skipComment()
		case c == '[':
			if err := p.parseTableHeader(); err != nil {
				return nil, err
			}
		default:
			if err := p.parseKeyVal(); err != nil {
				return nil, err
			}
		}
	}
	return p.root, nil
}

// ── Table headers ─────────────────────────────────────────────────────────────

func (p *parser) parseTableHeader() error {
	p.pos++ // consume '['
	isArray := false
	if p.pos < len(p.src) && p.src[p.pos] == '[' {
		isArray = true
		p.pos++
	}

	p.skipWS()
	keys, err := p.parseDottedKey()
	if err != nil {
		return err
	}
	p.skipWS()

	if isArray {
		if p.pos >= len(p.src) || p.src[p.pos] != ']' {
			return p.errf("expected ']]'")
		}
		p.pos++
		if p.pos >= len(p.src) || p.src[p.pos] != ']' {
			return p.errf("expected second ']' for array table")
		}
		p.pos++
	} else {
		if p.pos >= len(p.src) || p.src[p.pos] != ']' {
			return p.errf("expected ']'")
		}
		p.pos++
	}
	p.skipToNewline()

	if isArray {
		p.path = keys
		p.defined[strings.Join(keys, ".")] = true
		p.arrDefined[strings.Join(keys, ".")] = true
		p.current = p.getOrCreateAOT(keys)
	} else {
		path := strings.Join(keys, ".")
		if p.defined[path] {
			return p.errf("duplicate table: [%s]", path)
		}
		p.defined[path] = true
		p.path = keys
		p.current = p.getOrCreateTable(keys)
	}
	return nil
}

// getOrCreateTable navigates/creates nested maps for [table] headers.
func (p *parser) getOrCreateTable(keys []string) map[string]any {
	m := p.root
	for i, k := range keys {
		if i == len(keys)-1 {
			if existing, ok := m[k]; ok {
				if em, ok := existing.(map[string]any); ok {
					return em
				}
			}
			sub := make(map[string]any)
			m[k] = sub
			return sub
		}
		if existing, ok := m[k]; ok {
			switch e := existing.(type) {
			case map[string]any:
				m = e
			case []any:
				// Navigate into the last element of an AOT.
				if len(e) == 0 {
					return nil
				}
				if last, ok := e[len(e)-1].(map[string]any); ok {
					m = last
				}
			}
		} else {
			sub := make(map[string]any)
			m[k] = sub
			m = sub
		}
	}
	return m
}

// getOrCreateAOT navigates to the parent of keys[-1] and appends a new map
// to the array of tables.
func (p *parser) getOrCreateAOT(keys []string) map[string]any {
	m := p.root
	for i, k := range keys {
		if i == len(keys)-1 {
			existing, ok := m[k]
			if !ok {
				arr := []any{}
				newEntry := make(map[string]any)
				arr = append(arr, newEntry)
				m[k] = arr
				return newEntry
			}
			if arr, ok := existing.([]any); ok {
				newEntry := make(map[string]any)
				m[k] = append(arr, newEntry)
				return newEntry
			}
			return nil
		}
		if existing, ok := m[k]; ok {
			switch e := existing.(type) {
			case map[string]any:
				m = e
			case []any:
				if len(e) > 0 {
					if last, ok := e[len(e)-1].(map[string]any); ok {
						m = last
					}
				}
			}
		} else {
			sub := make(map[string]any)
			m[k] = sub
			m = sub
		}
	}
	return m
}

// ── Key-value pairs ───────────────────────────────────────────────────────────

func (p *parser) parseKeyVal() error {
	keys, err := p.parseDottedKey()
	if err != nil {
		return err
	}
	p.skipWS()
	if p.pos >= len(p.src) || p.src[p.pos] != '=' {
		return p.errf("expected '=' after key")
	}
	p.pos++
	p.skipWS()

	val, err := p.parseValue()
	if err != nil {
		return err
	}

	p.skipWS()
	p.skipComment()
	p.skipNewline()

	// Set value in current table, creating intermediate tables for dotted keys.
	return setNestedKey(p.current, keys, val)
}

func setNestedKey(m map[string]any, keys []string, val any) error {
	for i, k := range keys {
		if i == len(keys)-1 {
			if _, exists := m[k]; exists {
				return fmt.Errorf("duplicate key: %s", k)
			}
			m[k] = val
			return nil
		}
		if existing, ok := m[k]; ok {
			if sub, ok := existing.(map[string]any); ok {
				m = sub
			} else {
				return fmt.Errorf("key %s already has a non-table value", k)
			}
		} else {
			sub := make(map[string]any)
			m[k] = sub
			m = sub
		}
	}
	return nil
}

// ── Key parsing ───────────────────────────────────────────────────────────────

func (p *parser) parseDottedKey() ([]string, error) {
	var keys []string
	for {
		k, err := p.parseSimpleKey()
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
		p.skipWS()
		if p.pos < len(p.src) && p.src[p.pos] == '.' {
			p.pos++
			p.skipWS()
		} else {
			break
		}
	}
	return keys, nil
}

func (p *parser) parseSimpleKey() (string, error) {
	if p.pos >= len(p.src) {
		return "", p.errf("unexpected end of input while parsing key")
	}
	c := p.src[p.pos]
	switch {
	case c == '"':
		return p.parseBasicString()
	case c == '\'':
		return p.parseLiteralString()
	default:
		// Bare key: A-Za-z0-9_-
		start := p.pos
		for p.pos < len(p.src) {
			c := p.src[p.pos]
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
				p.pos++
			} else {
				break
			}
		}
		if p.pos == start {
			return "", p.errf("empty or invalid key character '%c'", p.src[p.pos])
		}
		return string(p.src[start:p.pos]), nil
	}
}

// ── Value parsing ─────────────────────────────────────────────────────────────

func (p *parser) parseValue() (any, error) {
	if p.pos >= len(p.src) {
		return nil, p.errf("unexpected end of input in value")
	}
	c := p.src[p.pos]

	switch {
	case c == '"':
		if p.startsWith(`"""`) {
			return p.parseMLBasicString()
		}
		return p.parseBasicString()
	case c == '\'':
		if p.startsWith(`'''`) {
			return p.parseMLLiteralString()
		}
		return p.parseLiteralString()
	case c == 't':
		if p.startsWith("true") {
			p.pos += 4
			return true, nil
		}
		return nil, p.errf("expected 'true'")
	case c == 'f':
		if p.startsWith("false") {
			p.pos += 5
			return false, nil
		}
		return nil, p.errf("expected 'false'")
	case c == '[':
		return p.parseArray()
	case c == '{':
		return p.parseInlineTable()
	case c == 'i' || c == 'n': // inf, nan
		return p.parseSpecialFloat()
	case c == '+' || c == '-':
		return p.parseNumberOrDatetime()
	case c >= '0' && c <= '9':
		return p.parseNumberOrDatetime()
	default:
		return nil, p.errf("unexpected character '%c' in value", c)
	}
}

// ── String parsing ────────────────────────────────────────────────────────────

func (p *parser) parseBasicString() (string, error) {
	p.pos++ // opening "
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '"' {
			p.pos++
			return b.String(), nil
		}
		if c == '\n' {
			return "", p.errf("newline in basic string")
		}
		if c == '\\' {
			p.pos++
			r, err := p.parseEscape()
			if err != nil {
				return "", err
			}
			b.WriteRune(r)
			continue
		}
		b.WriteRune(c)
		p.pos++
	}
	return "", p.errf("unterminated basic string")
}

func (p *parser) parseLiteralString() (string, error) {
	p.pos++ // opening '
	start := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '\'' {
			s := string(p.src[start:p.pos])
			p.pos++
			return s, nil
		}
		if c == '\n' {
			return "", p.errf("newline in literal string")
		}
		p.pos++
	}
	return "", p.errf("unterminated literal string")
}

func (p *parser) parseMLBasicString() (string, error) {
	p.pos += 3 // """
	// Skip immediate newline after opening delimiter.
	if p.pos < len(p.src) && p.src[p.pos] == '\n' {
		p.pos++
		p.line++
	} else if p.pos+1 < len(p.src) && p.src[p.pos] == '\r' && p.src[p.pos+1] == '\n' {
		p.pos += 2
		p.line++
	}

	var b strings.Builder
	for p.pos < len(p.src) {
		if p.startsWith(`"""`) {
			// Allow up to 2 extra quotes before the closing delimiter.
			p.pos += 3
			// Handle up to 2 trailing quotes
			for p.pos < len(p.src) && p.src[p.pos] == '"' && b.Len() > 0 {
				b.WriteRune('"')
				p.pos++
			}
			s := b.String()
			// Trim trailing quotes that were added by the above loop.
			return strings.TrimRight(s, `"`), nil
		}
		c := p.src[p.pos]
		if c == '\\' {
			p.pos++
			if p.pos < len(p.src) && (p.src[p.pos] == '\n' || p.src[p.pos] == '\r') {
				// Line ending backslash: skip whitespace/newlines.
				for p.pos < len(p.src) && (p.src[p.pos] == '\n' || p.src[p.pos] == '\r' || p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
					if p.src[p.pos] == '\n' {
						p.line++
					}
					p.pos++
				}
				continue
			}
			r, err := p.parseEscape()
			if err != nil {
				return "", err
			}
			b.WriteRune(r)
			continue
		}
		if c == '\n' {
			p.line++
		}
		b.WriteRune(c)
		p.pos++
	}
	return "", p.errf("unterminated multi-line basic string")
}

func (p *parser) parseMLLiteralString() (string, error) {
	p.pos += 3 // '''
	// Skip immediate newline.
	if p.pos < len(p.src) && p.src[p.pos] == '\n' {
		p.pos++
		p.line++
	} else if p.pos+1 < len(p.src) && p.src[p.pos] == '\r' && p.src[p.pos+1] == '\n' {
		p.pos += 2
		p.line++
	}

	var b strings.Builder
	for p.pos < len(p.src) {
		if p.startsWith(`'''`) {
			p.pos += 3
			return b.String(), nil
		}
		c := p.src[p.pos]
		if c == '\n' {
			p.line++
		}
		b.WriteRune(c)
		p.pos++
	}
	return "", p.errf("unterminated multi-line literal string")
}

func (p *parser) parseEscape() (rune, error) {
	if p.pos >= len(p.src) {
		return 0, p.errf("unexpected end in escape sequence")
	}
	c := p.src[p.pos]
	p.pos++
	switch c {
	case 'b':
		return '\b', nil
	case 't':
		return '\t', nil
	case 'n':
		return '\n', nil
	case 'f':
		return '\f', nil
	case 'r':
		return '\r', nil
	case '"':
		return '"', nil
	case '\\':
		return '\\', nil
	case 'u':
		return p.parseUnicodeEscape(4)
	case 'U':
		return p.parseUnicodeEscape(8)
	default:
		return 0, p.errf("invalid escape sequence '\\%c'", c)
	}
}

func (p *parser) parseUnicodeEscape(n int) (rune, error) {
	if p.pos+n > len(p.src) {
		return 0, p.errf("short unicode escape")
	}
	s := string(p.src[p.pos : p.pos+n])
	p.pos += n
	v, err := strconv.ParseInt(s, 16, 32)
	if err != nil {
		return 0, p.errf("invalid unicode escape: %s", s)
	}
	r := rune(v)
	if !utf8.ValidRune(r) {
		return 0, p.errf("invalid unicode codepoint: U+%04X", v)
	}
	return r, nil
}

// ── Number / datetime parsing ─────────────────────────────────────────────────

func (p *parser) parseNumberOrDatetime() (any, error) {
	start := p.pos
	// Collect the token: stop at whitespace, comma, ], }, newline, comment.
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',' || c == ']' || c == '}' || c == '#' {
			break
		}
		p.pos++
	}
	tok := string(p.src[start:p.pos])
	if tok == "" {
		return nil, p.errf("empty number/datetime token")
	}

	// Try datetime first (contains '-' or ':' in specific positions).
	if t, ok := tryDatetime(tok); ok {
		return t, nil
	}

	// Integer with base prefix.
	if strings.HasPrefix(tok, "0x") || strings.HasPrefix(tok, "0X") {
		s := strings.ReplaceAll(tok[2:], "_", "")
		n, err := strconv.ParseInt(s, 16, 64)
		if err != nil {
			return nil, p.errf("invalid hex integer: %s", tok)
		}
		return n, nil
	}
	if strings.HasPrefix(tok, "0o") || strings.HasPrefix(tok, "0O") {
		s := strings.ReplaceAll(tok[2:], "_", "")
		n, err := strconv.ParseInt(s, 8, 64)
		if err != nil {
			return nil, p.errf("invalid octal integer: %s", tok)
		}
		return n, nil
	}
	if strings.HasPrefix(tok, "0b") || strings.HasPrefix(tok, "0B") {
		s := strings.ReplaceAll(tok[2:], "_", "")
		n, err := strconv.ParseInt(s, 2, 64)
		if err != nil {
			return nil, p.errf("invalid binary integer: %s", tok)
		}
		return n, nil
	}

	// Float or integer.
	clean := strings.ReplaceAll(tok, "_", "")
	if strings.ContainsAny(clean, ".eE") {
		f, err := strconv.ParseFloat(clean, 64)
		if err != nil {
			return nil, p.errf("invalid float: %s", tok)
		}
		return f, nil
	}

	n, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		// Try float anyway (e.g. +1.0 without 'e')
		f, err2 := strconv.ParseFloat(clean, 64)
		if err2 != nil {
			return nil, p.errf("invalid number: %s", tok)
		}
		return f, nil
	}
	return n, nil
}

func (p *parser) parseSpecialFloat() (any, error) {
	for _, prefix := range []string{"+inf", "-inf", "inf", "+nan", "-nan", "nan"} {
		if p.startsWith(prefix) {
			p.pos += len([]rune(prefix))
			switch prefix {
			case "inf", "+inf":
				return math.Inf(1), nil
			case "-inf":
				return math.Inf(-1), nil
			default:
				return math.NaN(), nil
			}
		}
	}
	return nil, p.errf("expected inf or nan")
}

// tryDatetime attempts to parse a TOML datetime token.
// Returns (value, true) on success.
func tryDatetime(s string) (any, bool) {
	// Offset datetime: 1979-05-27T07:32:00Z or with offset.
	// Local datetime: 1979-05-27T07:32:00
	// Local date: 1979-05-27
	// Local time: 07:32:00

	// Must start with 4 digits for date/datetime.
	if len(s) < 8 {
		return nil, false
	}
	if !isDigit(rune(s[0])) || !isDigit(rune(s[1])) || !isDigit(rune(s[2])) || !isDigit(rune(s[3])) {
		return nil, false
	}

	// Local time only: HH:MM:SS[.nanos]
	if s[2] == ':' {
		return s, true // return as string; Go has no time.Time for local time only
	}

	// Date-like token must have '-' at position 4.
	if len(s) < 10 || s[4] != '-' {
		return nil, false
	}

	// Local date only: YYYY-MM-DD
	if len(s) == 10 {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return nil, false
		}
		return t, true
	}

	// Datetime with 'T' or space separator.
	sep := s[10]
	if sep != 'T' && sep != 't' && sep != ' ' {
		return nil, false
	}

	// Try offset datetime.
	for _, layout := range []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999999999Z",
	} {
		norm := strings.ToUpper(s)
		t, err := time.Parse(layout, norm)
		if err == nil {
			return t, true
		}
	}

	// Local datetime (no timezone).
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
	} {
		norm := strings.ToUpper(s)
		t, err := time.Parse(layout, norm)
		if err == nil {
			return t, true
		}
	}

	return nil, false
}

// ── Array & inline table ──────────────────────────────────────────────────────

func (p *parser) parseArray() ([]any, error) {
	p.pos++ // [
	var items []any
	for {
		p.skipWSNewline()
		if p.pos >= len(p.src) {
			return nil, p.errf("unterminated array")
		}
		if p.src[p.pos] == ']' {
			p.pos++
			return items, nil
		}
		if p.src[p.pos] == '#' {
			p.skipComment()
			continue
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		items = append(items, v)
		p.skipWSNewline()
		if p.pos >= len(p.src) {
			return nil, p.errf("unterminated array")
		}
		if p.src[p.pos] == ',' {
			p.pos++
		}
	}
}

func (p *parser) parseInlineTable() (map[string]any, error) {
	p.pos++ // {
	m := make(map[string]any)
	first := true
	for {
		p.skipWS()
		if p.pos >= len(p.src) {
			return nil, p.errf("unterminated inline table")
		}
		if p.src[p.pos] == '}' {
			p.pos++
			return m, nil
		}
		if !first {
			if p.src[p.pos] != ',' {
				return nil, p.errf("expected ',' in inline table")
			}
			p.pos++
			p.skipWS()
		}
		first = false

		if p.pos >= len(p.src) || p.src[p.pos] == '}' {
			// Trailing comma not allowed in TOML inline tables.
			return nil, p.errf("trailing comma in inline table")
		}

		keys, err := p.parseDottedKey()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		if p.pos >= len(p.src) || p.src[p.pos] != '=' {
			return nil, p.errf("expected '=' in inline table")
		}
		p.pos++
		p.skipWS()
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if err := setNestedKey(m, keys, v); err != nil {
			return nil, err
		}
	}
}

// ── Utility ───────────────────────────────────────────────────────────────────

func (p *parser) skipWS() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
}

func (p *parser) skipWSNewline() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' {
			p.pos++
		} else if c == '\n' {
			p.pos++
			p.line++
		} else if c == '\r' {
			p.pos++
			if p.pos < len(p.src) && p.src[p.pos] == '\n' {
				p.pos++
			}
			p.line++
		} else {
			break
		}
	}
}

func (p *parser) skipComment() {
	for p.pos < len(p.src) && p.src[p.pos] != '\n' {
		p.pos++
	}
}

func (p *parser) skipToNewline() {
	p.skipWS()
	p.skipComment()
	p.skipNewline()
}

func (p *parser) skipNewline() {
	if p.pos < len(p.src) {
		if p.src[p.pos] == '\n' {
			p.pos++
			p.line++
		} else if p.src[p.pos] == '\r' {
			p.pos++
			if p.pos < len(p.src) && p.src[p.pos] == '\n' {
				p.pos++
			}
			p.line++
		}
	}
}

func (p *parser) startsWith(s string) bool {
	runes := []rune(s)
	if p.pos+len(runes) > len(p.src) {
		return false
	}
	for i, r := range runes {
		if p.src[p.pos+i] != r {
			return false
		}
	}
	return true
}

func (p *parser) errf(format string, args ...any) error {
	return fmt.Errorf("line %d: "+format, append([]any{p.line}, args...)...)
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// ═══════════════════════════════════════════════════════════════════════════════
// Encoder
// ═══════════════════════════════════════════════════════════════════════════════

type encoder struct {
	w   io.Writer
	err error
}

func (e *encoder) write(s string) {
	if e.err == nil {
		_, e.err = io.WriteString(e.w, s)
	}
}

func (e *encoder) writef(format string, args ...any) {
	e.write(fmt.Sprintf(format, args...))
}

// encodeTable writes a TOML table. prefix is the dot-separated key path so far.
func (e *encoder) encodeTable(prefix string, m map[string]any) error {
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// First pass: write scalar key-value pairs.
	for _, k := range keys {
		v := m[k]
		if isScalar(v) || isScalarArray(v) {
			e.writeKeyVal(k, v)
		}
	}

	// Second pass: write subtables and arrays of tables.
	for _, k := range keys {
		v := m[k]
		subPrefix := k
		if prefix != "" {
			subPrefix = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			e.write("\n")
			e.writef("[%s]\n", subPrefix)
			if err := e.encodeTable(subPrefix, val); err != nil {
				return err
			}
		case []any:
			if !isScalarArray(v) {
				for _, item := range val {
					if sub, ok := item.(map[string]any); ok {
						e.write("\n")
						e.writef("[[%s]]\n", subPrefix)
						if err := e.encodeTable(subPrefix, sub); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return e.err
}

func (e *encoder) writeKeyVal(k string, v any) {
	e.write(tomlKey(k) + " = ")
	e.writeValue(v)
	e.write("\n")
}

func (e *encoder) writeValue(v any) {
	switch val := v.(type) {
	case nil:
		e.write(`""`) // TOML has no null; encode as empty string.
	case bool:
		if val {
			e.write("true")
		} else {
			e.write("false")
		}
	case int64:
		e.writef("%d", val)
	case int:
		e.writef("%d", val)
	case float64:
		switch {
		case math.IsInf(val, 1):
			e.write("inf")
		case math.IsInf(val, -1):
			e.write("-inf")
		case math.IsNaN(val):
			e.write("nan")
		default:
			s := strconv.FormatFloat(val, 'f', -1, 64)
			if !strings.Contains(s, ".") {
				s += ".0"
			}
			e.write(s)
		}
	case float32:
		e.writeValue(float64(val))
	case string:
		e.write(tomlBasicString(val))
	case time.Time:
		e.writef("%s", val.Format(time.RFC3339Nano))
	case []any:
		e.write("[")
		for i, item := range val {
			if i > 0 {
				e.write(", ")
			}
			e.writeValue(item)
		}
		e.write("]")
	case map[string]any:
		// Inline table
		e.write("{")
		first := true
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !first {
				e.write(", ")
			}
			first = false
			e.write(tomlKey(k) + " = ")
			e.writeValue(val[k])
		}
		e.write("}")
	default:
		// Fallback: use fmt
		e.writef("%v", val)
	}
}

// isScalar returns true for types that render as a single-line TOML value.
func isScalar(v any) bool {
	switch v.(type) {
	case nil, bool, int64, int, float64, float32, string, time.Time:
		return true
	}
	return false
}

// isScalarArray returns true for []any containing only scalars.
func isScalarArray(v any) bool {
	arr, ok := v.([]any)
	if !ok {
		return false
	}
	for _, item := range arr {
		if _, ok := item.(map[string]any); ok {
			return false
		}
	}
	return true
}

// tomlKey returns a safe TOML key (bare if possible, otherwise quoted).
func tomlKey(s string) string {
	bare := true
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			bare = false
			break
		}
	}
	if bare && s != "" {
		return s
	}
	return tomlBasicString(s)
}

// tomlBasicString wraps s in TOML basic string quotes with proper escaping.
func tomlBasicString(s string) string {
	var b strings.Builder
	b.WriteRune('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteRune('"')
	return b.String()
}

// silence unused import
var _ = unicode.IsLetter
