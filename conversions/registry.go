// Package conversions provides the core interfaces and registry for all
// atob converters. To add a new converter, implement the Converter interface
// (or FileConverter for file-based conversions) and call Register() or
// RegisterFile() in an init() function in your converter file.
package conversions

import (
	"fmt"
	"sort"
	"strings"
)

// Converter is implemented by any type that can transform a string input
// into a string output. This is the primary interface for all text-based
// conversions (encoding, hashing, case, number bases, etc.).
type Converter interface {
	// Name returns the unique identifier used on the CLI, e.g. "base64-encode".
	Name() string
	// Category groups related converters together, e.g. "encoding".
	Category() string
	// Description is a short human-readable description shown in `atob list`.
	Description() string
	// Convert performs the conversion and returns the result or an error.
	Convert(input string) (string, error)
}

// FileConverter is implemented by converters that require file paths instead
// of reading from stdin (e.g. xlsx/csv conversions where binary formats are
// involved). Input and output are both file paths.
type FileConverter interface {
	// Name returns the unique identifier used on the CLI, e.g. "csv-xlsx".
	Name() string
	// Category groups related converters together, e.g. "formats".
	Category() string
	// Description is a short human-readable description shown in `atob list`.
	Description() string
	// ConvertFile reads inputPath and writes the result to outputPath.
	ConvertFile(inputPath, outputPath string) error
}

// ConverterInfo is a unified view of either a Converter or FileConverter,
// used for listing and discovery.
type ConverterInfo struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	FileBased   bool   `json:"file_based"`
}

// BinaryConverter is implemented by converters that operate on raw bytes.
// Input is the file's raw bytes (read from a file path or stdin).
// Output is always a UTF-8 string (JSON, hex dump, extracted strings, etc.).
type BinaryConverter interface {
	// Name returns the unique identifier, e.g. "inspect", "hexdump", "strings".
	Name() string
	// Category groups related converters together.
	Category() string
	// Description is shown in atob list.
	Description() string
	// ConvertBytes performs the conversion on raw binary input.
	ConvertBytes(input []byte) (string, error)
}

var (
	converters       = map[string]Converter{}
	fileConverters   = map[string]FileConverter{}
	binaryConverters = map[string]BinaryConverter{}
)

// Register adds a Converter to the global registry. It is typically called
// from an init() function in each converter file.
func Register(c Converter) {
	name := strings.ToLower(c.Name())
	if _, exists := converters[name]; exists {
		panic(fmt.Sprintf("converter %q already registered", name))
	}
	converters[name] = c
}

// RegisterFile adds a FileConverter to the global registry. It is typically
// called from an init() function in each converter file.
func RegisterFile(c FileConverter) {
	name := strings.ToLower(c.Name())
	if _, exists := fileConverters[name]; exists {
		panic(fmt.Sprintf("file converter %q already registered", name))
	}
	fileConverters[name] = c
}

// RegisterBinary adds a BinaryConverter to the global registry.
func RegisterBinary(c BinaryConverter) {
	name := strings.ToLower(c.Name())
	if _, exists := binaryConverters[name]; exists {
		panic(fmt.Sprintf("binary converter %q already registered", name))
	}
	binaryConverters[name] = c
}

// Get returns a Converter by name, or false if not found.
func Get(name string) (Converter, bool) {
	c, ok := converters[strings.ToLower(name)]
	return c, ok
}

// GetFile returns a FileConverter by name, or false if not found.
func GetFile(name string) (FileConverter, bool) {
	c, ok := fileConverters[strings.ToLower(name)]
	return c, ok
}

// GetBinary returns a BinaryConverter by name, or false if not found.
func GetBinary(name string) (BinaryConverter, bool) {
	c, ok := binaryConverters[strings.ToLower(name)]
	return c, ok
}

// All returns a sorted list of ConverterInfo for every registered converter,
// both text-based and file-based.
func All() []ConverterInfo {
	var list []ConverterInfo
	for _, c := range converters {
		list = append(list, ConverterInfo{
			Name:        c.Name(),
			Category:    c.Category(),
			Description: c.Description(),
			FileBased:   false,
		})
	}
	for _, c := range fileConverters {
		list = append(list, ConverterInfo{
			Name:        c.Name(),
			Category:    c.Category(),
			Description: c.Description(),
			FileBased:   true,
		})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Category != list[j].Category {
			return list[i].Category < list[j].Category
		}
		return list[i].Name < list[j].Name
	})
	return list
}
