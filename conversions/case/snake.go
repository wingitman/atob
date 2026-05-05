package caseconv

import (
	"strings"

	internalstrcase "github.com/wingitman/atob/conversions/internal/strcase"
	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(toSnake{})
	conversions.Register(toScreamingSnake{})
}

type toSnake struct{}

func (toSnake) Name() string        { return "case-snake" }
func (toSnake) Category() string    { return "case" }
func (toSnake) Description() string { return "Convert text to snake_case" }

func (toSnake) Convert(input string) (string, error) {
	return internalstrcase.ToSnake(strings.TrimRight(input, "\n")), nil
}

type toScreamingSnake struct{}

func (toScreamingSnake) Name() string        { return "case-screaming-snake" }
func (toScreamingSnake) Category() string    { return "case" }
func (toScreamingSnake) Description() string { return "Convert text to SCREAMING_SNAKE_CASE" }

func (toScreamingSnake) Convert(input string) (string, error) {
	return internalstrcase.ToScreamingSnake(strings.TrimRight(input, "\n")), nil
}
