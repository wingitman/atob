package caseconv

import (
	"strings"

	internalstrcase "github.com/wingitman/atob/conversions/internal/strcase"
	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(toCamel{})
	conversions.Register(toLowerCamel{})
}

type toCamel struct{}

func (toCamel) Name() string        { return "case-pascal" }
func (toCamel) Category() string    { return "case" }
func (toCamel) Description() string { return "Convert text to PascalCase (UpperCamelCase)" }

func (toCamel) Convert(input string) (string, error) {
	return internalstrcase.ToCamel(strings.TrimRight(input, "\n")), nil
}

type toLowerCamel struct{}

func (toLowerCamel) Name() string        { return "case-camel" }
func (toLowerCamel) Category() string    { return "case" }
func (toLowerCamel) Description() string { return "Convert text to camelCase (lowerCamelCase)" }

func (toLowerCamel) Convert(input string) (string, error) {
	return internalstrcase.ToLowerCamel(strings.TrimRight(input, "\n")), nil
}
