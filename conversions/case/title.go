package caseconv

import (
	"strings"

	internalstrcase "github.com/wingitman/atob/conversions/internal/strcase"
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
	return internalstrcase.ToDelimited(strings.TrimRight(input, "\n"), ' '), nil
}
