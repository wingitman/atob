package formats

import (
	"fmt"
	"strings"

	"github.com/clbanning/mxj/v2"
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
	mv, err := mxj.NewMapJson([]byte(strings.TrimSpace(input)))
	if err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	xmlBytes, err := mv.XmlIndent("", "  ")
	if err != nil {
		return "", fmt.Errorf("XML encode error: %w", err)
	}
	return string(xmlBytes), nil
}

type xmlToJSON struct{}

func (xmlToJSON) Name() string        { return "xml-json" }
func (xmlToJSON) Category() string    { return "formats" }
func (xmlToJSON) Description() string { return "Convert XML to JSON" }

func (xmlToJSON) Convert(input string) (string, error) {
	mv, err := mxj.NewMapXml([]byte(strings.TrimSpace(input)))
	if err != nil {
		return "", fmt.Errorf("invalid XML: %w", err)
	}
	jsonBytes, err := mv.JsonIndent("", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON encode error: %w", err)
	}
	return string(jsonBytes), nil
}
