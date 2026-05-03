package caseconv

import (
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(toKebab{})
	conversions.Register(toScreamingKebab{})
}

type toKebab struct{}

func (toKebab) Name() string        { return "case-kebab" }
func (toKebab) Category() string    { return "case" }
func (toKebab) Description() string { return "Convert text to kebab-case" }

func (toKebab) Convert(input string) (string, error) {
	return strcase.ToKebab(strings.TrimRight(input, "\n")), nil
}

type toScreamingKebab struct{}

func (toScreamingKebab) Name() string        { return "case-screaming-kebab" }
func (toScreamingKebab) Category() string    { return "case" }
func (toScreamingKebab) Description() string { return "Convert text to SCREAMING-KEBAB-CASE" }

func (toScreamingKebab) Convert(input string) (string, error) {
	return strcase.ToScreamingDelimited(strings.TrimRight(input, "\n"), '-', "", true), nil
}
