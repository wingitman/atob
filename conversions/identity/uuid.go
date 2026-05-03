package identity

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(uuidGenerate{})
	conversions.Register(uuidValidate{})
}

type uuidGenerate struct{}

func (uuidGenerate) Name() string        { return "uuid-generate" }
func (uuidGenerate) Category() string    { return "identity" }
func (uuidGenerate) Description() string { return "Generate a new random UUID v4 (ignores input)" }

func (uuidGenerate) Convert(_ string) (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	return id.String(), nil
}

type uuidValidate struct{}

func (uuidValidate) Name() string        { return "uuid-validate" }
func (uuidValidate) Category() string    { return "identity" }
func (uuidValidate) Description() string { return "Validate a UUID and report its version" }

func (uuidValidate) Convert(input string) (string, error) {
	id, err := uuid.Parse(strings.TrimSpace(input))
	if err != nil {
		return "", fmt.Errorf("invalid UUID: %w", err)
	}
	return fmt.Sprintf("valid UUID v%d: %s", id.Version(), id.String()), nil
}
