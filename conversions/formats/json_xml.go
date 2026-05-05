package formats

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(jsonToXML{})
	conversions.Register(xmlToJSON{})
}

type jsonToXML struct{}

func (jsonToXML) Name() string        { return "json-xml" }
func (jsonToXML) Category() string    { return "formats" }
func (jsonToXML) Description() string { return "Convert JSON to XML" }

func (jsonToXML) Convert(input string) (string, error) {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")

	if err := encodeXMLValue(enc, "root", data); err != nil {
		return "", fmt.Errorf("XML encode error: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return "", fmt.Errorf("XML flush error: %w", err)
	}
	buf.WriteByte('\n')
	return buf.String(), nil
}

type xmlToJSON struct{}

func (xmlToJSON) Name() string        { return "xml-json" }
func (xmlToJSON) Category() string    { return "formats" }
func (xmlToJSON) Description() string { return "Convert XML to JSON" }

func (xmlToJSON) Convert(input string) (string, error) {
	dec := xml.NewDecoder(strings.NewReader(strings.TrimSpace(input)))
	m, err := decodeXMLToMap(dec)
	if err != nil {
		return "", fmt.Errorf("invalid XML: %w", err)
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON encode error: %w", err)
	}
	return string(out), nil
}

// ── JSON → XML ────────────────────────────────────────────────────────────────

// encodeXMLValue writes a single JSON value as XML under the given element name.
func encodeXMLValue(enc *xml.Encoder, name string, v any) error {
	start := xml.StartElement{Name: xml.Name{Local: xmlSafeName(name)}}

	switch val := v.(type) {
	case map[string]any:
		if err := enc.EncodeToken(start); err != nil {
			return err
		}
		for k, child := range val {
			if err := encodeXMLValue(enc, k, child); err != nil {
				return err
			}
		}
		return enc.EncodeToken(start.End())

	case []any:
		// Repeat the parent element for each array item.
		for _, item := range val {
			if err := encodeXMLValue(enc, name, item); err != nil {
				return err
			}
		}
		return nil

	case nil:
		if err := enc.EncodeToken(start); err != nil {
			return err
		}
		return enc.EncodeToken(start.End())

	default:
		if err := enc.EncodeToken(start); err != nil {
			return err
		}
		text := fmt.Sprintf("%v", val)
		if err := enc.EncodeToken(xml.CharData(text)); err != nil {
			return err
		}
		return enc.EncodeToken(start.End())
	}
}

// xmlSafeName converts a string to a valid XML element name.
// Replaces characters that are illegal in XML names with underscores.
func xmlSafeName(s string) string {
	if s == "" {
		return "_"
	}
	var b strings.Builder
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			b.WriteRune(r)
		case i > 0 && (r >= '0' && r <= '9' || r == '-' || r == '.'):
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// ── XML → JSON ────────────────────────────────────────────────────────────────

// decodeXMLToMap reads the first top-level element of an XML document and
// returns it as a map[string]any (or string for text-only elements).
func decodeXMLToMap(dec *xml.Decoder) (any, error) {
	// Skip to first start element
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("no XML element found: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			val, err := decodeElement(dec, se)
			if err != nil {
				return nil, err
			}
			// Wrap in a map keyed by the root element name
			return map[string]any{se.Name.Local: val}, nil
		}
	}
}

// decodeElement reads the content of an already-opened start element.
func decodeElement(dec *xml.Decoder, start xml.StartElement) (any, error) {
	children := map[string]any{}
	var text strings.Builder
	hasChildren := false

	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			hasChildren = true
			child, err := decodeElement(dec, t)
			if err != nil {
				return nil, err
			}
			addToMap(children, t.Name.Local, child)

		case xml.EndElement:
			if hasChildren {
				if txt := strings.TrimSpace(text.String()); txt != "" {
					children["#text"] = txt
				}
				return children, nil
			}
			// Text-only element
			return strings.TrimSpace(text.String()), nil

		case xml.CharData:
			text.Write(t)
		}
	}
}

// addToMap adds value v under key k. If key already exists, it converts the
// existing value to (or appends to) a []any slice.
func addToMap(m map[string]any, k string, v any) {
	existing, exists := m[k]
	if !exists {
		m[k] = v
		return
	}
	switch e := existing.(type) {
	case []any:
		m[k] = append(e, v)
	default:
		m[k] = []any{existing, v}
	}
}
