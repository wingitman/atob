// Package yaml implements a YAML 1.2 parser and emitter for Go.
//
// The parser supports:
//   - Block mappings and sequences
//   - Flow mappings { k: v } and flow sequences [ a, b ]
//   - Block scalars: literal (|) and folded (>), with chomping (+/-)
//   - Plain, single-quoted ('…'), double-quoted ("…") scalars
//   - Aliases (*alias) and anchors (&anchor)
//   - Multi-document streams (only the first document is returned by Unmarshal)
//   - Core schema type detection: null, bool, int (decimal/hex/octal), float
//   - Comments (# …) and directives (%YAML, %TAG) are skipped
//
// The emitter produces clean block-style YAML with 2-space indentation.
// Strings containing special characters are double-quoted with proper escaping.
package yaml

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

// Unmarshal parses the first YAML document in data and stores the result in v.
func Unmarshal(data []byte, v *any) error {
	p := newParser(string(data))
	val, err := p.parseDocument()
	if err != nil {
		return err
	}
	*v = val
	return nil
}

// Marshal serialises v to YAML and returns the result as []byte.
func Marshal(v any) ([]byte, error) {
	var sb strings.Builder
	e := &emitter{w: &sb}
	if err := e.emit(v, 0); err != nil {
		return nil, err
	}
	sb.WriteString("\n")
	return []byte(sb.String()), nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Parser
// ═══════════════════════════════════════════════════════════════════════════════

type parser struct {
	lines   []string
	lineIdx int
	col     int // byte offset within lines[lineIdx]
	anchors map[string]any
}

func newParser(src string) *parser {
	// Normalise line endings.
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	return &parser{
		lines:   strings.Split(src, "\n"),
		anchors: make(map[string]any),
	}
}

// ── Document entry ────────────────────────────────────────────────────────────

func (p *parser) parseDocument() (any, error) {
	p.skipEmptyAndDirectives()
	if p.lineIdx >= len(p.lines) {
		return nil, nil
	}
	// Skip optional "---" document-start marker.
	if strings.HasPrefix(p.currentLine(), "---") {
		p.advanceLine()
	}
	return p.parseNode(0)
}

// parseNode parses one YAML node at or deeper than minIndent.
func (p *parser) parseNode(minIndent int) (any, error) {
	p.skipEmptyLines()
	if p.lineIdx >= len(p.lines) {
		return nil, nil
	}

	line := p.currentLine()
	indent := p.lineIndent(line)

	if indent < minIndent {
		return nil, nil
	}

	// Detect node type from the first non-space character.
	content := strings.TrimSpace(line)
	if content == "" || strings.HasPrefix(content, "#") {
		p.advanceLine()
		return p.parseNode(minIndent)
	}

	// Anchor/alias prefix
	anchor := ""
	if strings.HasPrefix(content, "&") {
		parts := strings.SplitN(content, " ", 2)
		anchor = parts[0][1:] // strip '&'
		if len(parts) == 2 {
			// Replace the line content after the anchor.
			content = parts[1]
			line = strings.Repeat(" ", indent) + content
			p.setCurrentLine(line)
		} else {
			p.advanceLine()
		}
	}
	if strings.HasPrefix(content, "*") {
		name := strings.TrimSpace(content[1:])
		// Strip trailing colon if any (shouldn't be, but defensive)
		name = strings.TrimRight(name, ":")
		p.advanceLine()
		if v, ok := p.anchors[name]; ok {
			return v, nil
		}
		return nil, fmt.Errorf("yaml: unknown alias *%s", name)
	}

	var val any
	var err error

	switch {
	case strings.HasPrefix(content, "- ") || content == "-":
		val, err = p.parseBlockSequence(indent)
	case strings.HasPrefix(content, "{"):
		val, err = p.parseFlowMapping(indent)
	case strings.HasPrefix(content, "["):
		val, err = p.parseFlowSequence(indent)
	case strings.HasPrefix(content, "|") || strings.HasPrefix(content, ">"):
		val, err = p.parseBlockScalar(indent)
	case isBlockMappingEntry(content):
		val, err = p.parseBlockMapping(indent)
	default:
		val, err = p.parseScalarLine(indent)
	}

	if err != nil {
		return nil, err
	}
	if anchor != "" {
		p.anchors[anchor] = val
	}
	return val, nil
}

// ── Block sequence ─────────────────────────────────────────────────────────────

func (p *parser) parseBlockSequence(seqIndent int) ([]any, error) {
	var items []any
	for {
		p.skipEmptyLines()
		if p.lineIdx >= len(p.lines) {
			break
		}
		line := p.currentLine()
		indent := p.lineIndent(line)
		if indent < seqIndent {
			break
		}
		content := strings.TrimSpace(line)
		if !strings.HasPrefix(content, "- ") && content != "-" {
			break
		}

		// Strip "- " from the line.
		afterDash := content[1:]
		if len(afterDash) > 0 && afterDash[0] == ' ' {
			afterDash = afterDash[1:]
		}
		afterDash = strings.TrimSpace(afterDash)

		if afterDash == "" || strings.HasPrefix(afterDash, "#") {
			// Value is on the next line(s).
			p.advanceLine()
			v, err := p.parseNode(seqIndent + 2)
			if err != nil {
				return nil, err
			}
			items = append(items, v)
		} else {
			// Inline value.
			p.setCurrentLine(strings.Repeat(" ", seqIndent+2) + afterDash)
			v, err := p.parseNode(seqIndent + 2)
			if err != nil {
				return nil, err
			}
			items = append(items, v)
		}
	}
	return items, nil
}

// ── Block mapping ─────────────────────────────────────────────────────────────

func (p *parser) parseBlockMapping(mapIndent int) (map[string]any, error) {
	m := make(map[string]any)
	for {
		p.skipEmptyLines()
		if p.lineIdx >= len(p.lines) {
			break
		}
		line := p.currentLine()
		indent := p.lineIndent(line)
		if indent < mapIndent {
			break
		}
		content := strings.TrimSpace(line)
		if !isBlockMappingEntry(content) {
			break
		}

		key, rest, err := splitMappingKey(content)
		if err != nil {
			return nil, fmt.Errorf("yaml: %w at line %d", err, p.lineIdx+1)
		}

		if rest == "" || strings.HasPrefix(rest, "#") {
			// Value on next line.
			p.advanceLine()
			v, err := p.parseNode(mapIndent + 2)
			if err != nil {
				return nil, err
			}
			m[key] = v
		} else {
			// Inline value — could be a nested block.
			p.setCurrentLine(strings.Repeat(" ", mapIndent+2) + rest)
			v, err := p.parseNode(mapIndent + 2)
			if err != nil {
				return nil, err
			}
			m[key] = v
		}
	}
	return m, nil
}

// ── Block scalars (| and >) ───────────────────────────────────────────────────

func (p *parser) parseBlockScalar(scalarIndent int) (string, error) {
	line := p.currentLine()
	content := strings.TrimSpace(line)
	p.advanceLine()

	style := content[0] // '|' or '>'
	chomping := 'c'     // 'c' = clip (default), '+' = keep, '-' = strip
	// Parse chomp indicator and optional explicit indent indicator.
	rest := content[1:]
	if len(rest) > 0 {
		switch rest[0] {
		case '+':
			chomping = '+'
			rest = rest[1:]
		case '-':
			chomping = '-'
			rest = rest[1:]
		}
	}
	_ = rest // explicit indent indicator ignored for now

	// Collect continuation lines deeper than scalarIndent.
	var collected []string
	blockIndent := -1
	for p.lineIdx < len(p.lines) {
		bl := p.lines[p.lineIdx]
		// Blank lines are kept as-is.
		if strings.TrimSpace(bl) == "" {
			collected = append(collected, "")
			p.lineIdx++
			continue
		}
		ind := p.lineIndent(bl)
		if blockIndent < 0 {
			blockIndent = ind
		}
		if ind < blockIndent || (ind <= scalarIndent && ind < blockIndent) {
			break
		}
		if ind < scalarIndent+1 {
			break
		}
		prefix := ""
		if blockIndent > 0 && len(bl) > blockIndent {
			prefix = bl[blockIndent:]
		} else {
			prefix = strings.TrimLeft(bl, " \t")
		}
		collected = append(collected, prefix)
		p.lineIdx++
	}

	// Remove trailing blanks based on chomping.
	switch chomping {
	case '-': // strip: remove all trailing newlines
		for len(collected) > 0 && collected[len(collected)-1] == "" {
			collected = collected[:len(collected)-1]
		}
	case 'c': // clip: exactly one trailing newline
		for len(collected) > 1 && collected[len(collected)-1] == "" {
			collected = collected[:len(collected)-1]
		}
	// '+': keep all trailing newlines
	}

	var result string
	if style == '|' {
		result = strings.Join(collected, "\n")
		if chomping != '-' {
			result += "\n"
		}
	} else { // '>'
		// Folded: join single newlines with spaces; blank lines become newlines.
		var sb strings.Builder
		for i, l := range collected {
			if l == "" {
				sb.WriteByte('\n')
			} else {
				if i > 0 && collected[i-1] != "" {
					sb.WriteByte(' ')
				}
				sb.WriteString(l)
			}
		}
		result = sb.String()
		if chomping != '-' {
			result += "\n"
		}
	}
	return result, nil
}

// ── Flow sequences and mappings ───────────────────────────────────────────────

func (p *parser) parseFlowSequence(seqIndent int) ([]any, error) {
	// Collect the full flow value (may span multiple lines).
	raw := p.collectFlow('[', ']', seqIndent)
	return parseFlowSeq(raw)
}

func (p *parser) parseFlowMapping(mapIndent int) (map[string]any, error) {
	raw := p.collectFlow('{', '}', mapIndent)
	return parseFlowMap(raw)
}

// collectFlow collects characters from open to matching close, possibly
// spanning multiple lines, and advances the parser past them.
func (p *parser) collectFlow(open, close rune, indent int) string {
	var sb strings.Builder
	depth := 0
	for p.lineIdx < len(p.lines) {
		line := p.currentLine()
		p.advanceLine()
		for _, c := range line {
			if c == open {
				depth++
			} else if c == close {
				depth--
			}
			sb.WriteRune(c)
			if depth == 0 {
				// Rest of the line after the closing delimiter goes back.
				// (We consumed the whole line for simplicity; trailing content ignored.)
				return sb.String()
			}
		}
		// Multi-line flow: add space between lines.
		sb.WriteByte(' ')
	}
	return sb.String()
}

// ── Scalar line ───────────────────────────────────────────────────────────────

func (p *parser) parseScalarLine(indent int) (any, error) {
	line := p.currentLine()
	content := strings.TrimSpace(line)
	p.advanceLine()

	// Strip inline comment.
	content = stripInlineComment(content)
	return parseScalar(content), nil
}

// ── Helper functions ──────────────────────────────────────────────────────────

func (p *parser) currentLine() string {
	if p.lineIdx < len(p.lines) {
		return p.lines[p.lineIdx]
	}
	return ""
}

func (p *parser) setCurrentLine(s string) {
	if p.lineIdx < len(p.lines) {
		p.lines[p.lineIdx] = s
	}
}

func (p *parser) advanceLine() {
	p.lineIdx++
}

func (p *parser) lineIndent(line string) int {
	for i, c := range line {
		if c != ' ' && c != '\t' {
			return i
		}
	}
	return len(line)
}

func (p *parser) skipEmptyLines() {
	for p.lineIdx < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.lineIdx])
		if line == "" || strings.HasPrefix(line, "#") {
			p.lineIdx++
		} else {
			break
		}
	}
}

func (p *parser) skipEmptyAndDirectives() {
	for p.lineIdx < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.lineIdx])
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "%") {
			p.lineIdx++
		} else {
			break
		}
	}
}

func isBlockMappingEntry(content string) bool {
	if strings.HasPrefix(content, "? ") {
		return true // explicit key
	}
	// key: value — key can be quoted.
	if strings.HasPrefix(content, `"`) || strings.HasPrefix(content, `'`) {
		// Find closing quote.
		q := rune(content[0])
		for i := 1; i < len(content); i++ {
			if rune(content[i]) == q {
				if i+1 < len(content) && content[i+1] == ':' {
					return true
				}
				break
			}
		}
	}
	// Plain key: look for unquoted ': ' or ':'followed by EOL.
	for i := 0; i < len(content)-1; i++ {
		if content[i] == ':' && (content[i+1] == ' ' || content[i+1] == '\t') {
			return true
		}
	}
	if strings.HasSuffix(content, ":") {
		return true
	}
	return false
}

// splitMappingKey splits a block mapping line "key: rest" into key and rest.
func splitMappingKey(content string) (string, string, error) {
	// Handle quoted keys.
	if len(content) > 0 && (content[0] == '"' || content[0] == '\'') {
		q := content[0]
		end := -1
		if q == '"' {
			// Handle escape sequences minimally.
			for i := 1; i < len(content); i++ {
				if content[i] == '\\' {
					i++
					continue
				}
				if content[i] == '"' {
					end = i
					break
				}
			}
		} else {
			for i := 1; i < len(content); i++ {
				if content[i] == '\'' {
					end = i
					break
				}
			}
		}
		if end < 0 {
			return "", "", fmt.Errorf("unterminated quoted key")
		}
		key := content[1:end]
		rest := strings.TrimPrefix(content[end+1:], ":")
		rest = strings.TrimPrefix(rest, " ")
		return key, strings.TrimSpace(rest), nil
	}

	// Plain key.
	idx := strings.Index(content, ": ")
	if idx >= 0 {
		return strings.TrimSpace(content[:idx]), strings.TrimSpace(content[idx+2:]), nil
	}
	if strings.HasSuffix(content, ":") {
		return strings.TrimSpace(content[:len(content)-1]), "", nil
	}
	// Try colon-only (key:\n)
	if strings.Contains(content, ":") {
		parts := strings.SplitN(content, ":", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
	}
	return content, "", nil
}

func stripInlineComment(s string) string {
	// Remove trailing comment (#) — careful not to strip # inside quoted strings.
	inSingle := false
	inDouble := false
	for i, c := range s {
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '#' && !inSingle && !inDouble:
			return strings.TrimRight(s[:i], " \t")
		}
	}
	return s
}

// ── Scalar type detection (YAML 1.2 core schema) ─────────────────────────────

func parseScalar(s string) any {
	if s == "" || s == "~" || strings.EqualFold(s, "null") {
		return nil
	}
	if s == "true" || s == "True" || s == "TRUE" {
		return true
	}
	if s == "false" || s == "False" || s == "FALSE" {
		return false
	}

	// Integer.
	if n, ok := tryInt(s); ok {
		return n
	}

	// Float.
	if f, ok := tryFloat(s); ok {
		return f
	}

	// Quoted string: strip quotes.
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return parseSingleQuoted(s[1 : len(s)-1])
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return parseDoubleQuoted(s[1 : len(s)-1])
	}

	return s
}

func tryInt(s string) (int64, bool) {
	clean := strings.ReplaceAll(s, "_", "")
	if strings.HasPrefix(clean, "0x") || strings.HasPrefix(clean, "0X") {
		n, err := strconv.ParseInt(clean[2:], 16, 64)
		return n, err == nil
	}
	if strings.HasPrefix(clean, "0o") {
		n, err := strconv.ParseInt(clean[2:], 8, 64)
		return n, err == nil
	}
	if strings.HasPrefix(clean, "0b") {
		n, err := strconv.ParseInt(clean[2:], 2, 64)
		return n, err == nil
	}
	n, err := strconv.ParseInt(clean, 10, 64)
	return n, err == nil
}

func tryFloat(s string) (float64, bool) {
	switch strings.ToLower(s) {
	case ".inf", "+.inf":
		return math.Inf(1), true
	case "-.inf":
		return math.Inf(-1), true
	case ".nan":
		return math.NaN(), true
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
	return f, err == nil
}

func parseSingleQuoted(s string) string {
	return strings.ReplaceAll(s, "''", "'")
}

func parseDoubleQuoted(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '\\' && i+1 < len(runes) {
			i++
			switch runes[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case 'u':
				if i+4 < len(runes) {
					hex := string(runes[i+1 : i+5])
					i += 4
					if n, err := strconv.ParseInt(hex, 16, 32); err == nil {
						b.WriteRune(rune(n))
					}
				}
			case 'U':
				if i+8 < len(runes) {
					hex := string(runes[i+1 : i+9])
					i += 8
					if n, err := strconv.ParseInt(hex, 16, 32); err == nil {
						b.WriteRune(rune(n))
					}
				}
			default:
				b.WriteRune(runes[i])
			}
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// ── Flow collection parsers ───────────────────────────────────────────────────

func parseFlowSeq(raw string) ([]any, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 {
		return nil, nil
	}
	inner := raw[1 : len(raw)-1] // strip [ ]
	tokens, err := flowTokenize(inner)
	if err != nil {
		return nil, err
	}
	var items []any
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if strings.HasPrefix(tok, "{") {
			v, err := parseFlowMap(tok)
			if err != nil {
				return nil, err
			}
			items = append(items, v)
		} else if strings.HasPrefix(tok, "[") {
			v, err := parseFlowSeq(tok)
			if err != nil {
				return nil, err
			}
			items = append(items, v)
		} else {
			items = append(items, parseScalar(tok))
		}
	}
	return items, nil
}

func parseFlowMap(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 {
		return nil, nil
	}
	inner := raw[1 : len(raw)-1] // strip { }
	tokens, err := flowTokenize(inner)
	if err != nil {
		return nil, err
	}
	m := make(map[string]any)
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		key, rest, _ := splitMappingKey(tok)
		m[key] = parseScalar(rest)
	}
	return m, nil
}

// flowTokenize splits a flow collection body by commas at depth 0.
func flowTokenize(s string) ([]string, error) {
	var tokens []string
	depth := 0
	start := 0
	inSingle := false
	inDouble := false
	runes := []rune(s)
	for i, c := range runes {
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case (c == '{' || c == '[') && !inSingle && !inDouble:
			depth++
		case (c == '}' || c == ']') && !inSingle && !inDouble:
			depth--
		case c == ',' && depth == 0 && !inSingle && !inDouble:
			tokens = append(tokens, string(runes[start:i]))
			start = i + 1
		}
	}
	if start <= len(runes) {
		tokens = append(tokens, string(runes[start:]))
	}
	return tokens, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Emitter
// ═══════════════════════════════════════════════════════════════════════════════

type emitter struct {
	w   io.StringWriter
	err error
}

func (e *emitter) write(s string) {
	if e.err == nil {
		_, e.err = e.w.WriteString(s)
	}
}

func (e *emitter) emit(v any, indent int) error {
	prefix := strings.Repeat("  ", indent)
	switch val := v.(type) {
	case nil:
		e.write("null")
	case bool:
		if val {
			e.write("true")
		} else {
			e.write("false")
		}
	case int64:
		e.write(strconv.FormatInt(val, 10))
	case int:
		e.write(strconv.Itoa(val))
	case uint64:
		e.write(strconv.FormatUint(val, 10))
	case float64:
		switch {
		case math.IsInf(val, 1):
			e.write(".inf")
		case math.IsInf(val, -1):
			e.write("-.inf")
		case math.IsNaN(val):
			e.write(".nan")
		default:
			s := strconv.FormatFloat(val, 'f', -1, 64)
			e.write(s)
		}
	case float32:
		return e.emit(float64(val), indent)
	case string:
		e.write(yamlScalarString(val))
	case time.Time:
		e.write(val.Format(time.RFC3339Nano))
	case []any:
		if len(val) == 0 {
			e.write("[]")
			return e.err
		}
		e.write("\n")
		for _, item := range val {
			e.write(prefix + "- ")
			if err := e.emit(item, indent+1); err != nil {
				return err
			}
			e.write("\n")
		}
		// Remove trailing newline — caller will add one.
		return e.err
	case map[string]any:
		if len(val) == 0 {
			e.write("{}")
			return e.err
		}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		e.write("\n")
		for _, k := range keys {
			e.write(prefix + yamlKeyString(k) + ": ")
			child := val[k]
			// Scalars inline; collections on next line.
			switch child.(type) {
			case map[string]any, []any:
				if err := e.emit(child, indent+1); err != nil {
					return err
				}
			default:
				if err := e.emit(child, indent+1); err != nil {
					return err
				}
				e.write("\n")
			}
		}
		return e.err
	default:
		e.write(fmt.Sprintf("%v", val))
	}
	return e.err
}

// yamlScalarString returns a safe YAML scalar representation for s.
func yamlScalarString(s string) string {
	if s == "" {
		return `""`
	}
	// Check if the value would be misinterpreted as a YAML type.
	switch s {
	case "true", "false", "null", "~", ".inf", "-.inf", ".nan":
		return `"` + s + `"`
	}
	// Strings that look like numbers need quoting.
	if _, ok := tryInt(s); ok {
		return `"` + s + `"`
	}
	if _, ok := tryFloat(s); ok {
		return `"` + s + `"`
	}

	// Check for special characters that require quoting.
	needsQuote := false
	for i, r := range s {
		if r == ':' || r == '#' || r == '{' || r == '}' || r == '[' || r == ']' || r == ',' || r == '&' || r == '*' || r == '?' || r == '|' || r == '-' || r == '<' || r == '>' || r == '=' || r == '!' || r == '%' || r == '@' || r == '`' {
			if i == 0 {
				needsQuote = true
				break
			}
		}
		if r == '\n' || r == '\t' || !unicode.IsPrint(r) {
			needsQuote = true
			break
		}
	}
	// Also quote if it starts/ends with whitespace.
	if len(s) > 0 && (s[0] == ' ' || s[len(s)-1] == ' ') {
		needsQuote = true
	}

	if !needsQuote {
		return s
	}
	return yamlDoubleQuote(s)
}

func yamlKeyString(s string) string {
	return yamlScalarString(s)
}

func yamlDoubleQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if !unicode.IsPrint(r) || r > utf8.MaxRune {
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
