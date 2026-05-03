package caseconv

import (
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(toTitle{})
}

type toTitle struct{}

func (toTitle) Name() string        { return "case-title" }
func (toTitle) Category() string    { return "case" }
func (toTitle) Description() string { return "Convert text to Title Case" }

func (toTitle) Convert(input string) (string, error) {
	return strcase.ToDelimited(strings.TrimRight(input, "\n"), ' '), nil
}
