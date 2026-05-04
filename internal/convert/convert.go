// Package convert provides the core conversion dispatch logic shared between
// the CLI (cmd package) and the TUI. It calls directly into the conversions
// registry without any I/O — all functions return strings, not print them.
package convert

import (
	"fmt"
	"strings"

	"github.com/wingitman/atob/conversions"
)

// Type constants — duplicated here so this package has no dependency on cmd.
const (
	TypeText    = "text"
	TypeDecimal = "decimal"
	TypeHex     = "hex"
	TypeOctal   = "octal"
	TypeBinary  = "binary"
)

// RunToString performs the conversion and returns the result as a string.
// from and to are canonical type names (e.g. "json", "yaml", "text", "base64").
// filePaths is only used for file-based converters (xlsx ↔ csv).
func RunToString(input, from, to string, filePaths []string) (string, error) {
	// Plain text / decimal passthrough
	if (from == TypeText || from == TypeDecimal) && (to == TypeText || to == TypeDecimal) {
		return strings.TrimRight(input, "\n"), nil
	}

	// Numeric disambiguation
	if (from == TypeText || from == TypeDecimal) && (to == TypeHex || to == TypeOctal || to == TypeBinary) {
		if isPureDecimal(strings.TrimSpace(input)) {
			switch to {
			case TypeHex:
				return converterToString("dec-hex", input)
			case TypeOctal:
				return converterToString("dec-oct", input)
			case TypeBinary:
				return converterToString("dec-bin", input)
			}
		}
	}

	// Look up in the conversion matrix via registry
	internalName, err := resolveConverter(from, to)
	if err != nil {
		return "", err
	}
	if internalName == "" {
		return strings.TrimRight(input, "\n"), nil // passthrough
	}

	// File-based converters
	if strings.HasPrefix(internalName, "*") {
		name := internalName[1:]
		if len(filePaths) < 2 {
			return "", fmt.Errorf(
				"%s→%s conversion requires file paths:\n  atob %s %s <input-file> <output-file>",
				from, to, from, to,
			)
		}
		fc, ok := conversions.GetFile(name)
		if !ok {
			return "", fmt.Errorf("internal error: file converter %q not found", name)
		}
		return "", fc.ConvertFile(filePaths[0], filePaths[1])
	}

	return converterToString(internalName, input)
}

// RunBinaryToString performs binary inspection and returns the result as a string.
func RunBinaryToString(data []byte, to string) (string, error) {
	c, ok := conversions.GetBinary(to)
	if !ok {
		return "", fmt.Errorf("unknown binary converter %q", to)
	}
	out, err := c.ConvertBytes(data)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(out, "\n"), nil
}

// converterToString looks up and runs a text-based converter by internal name.
func converterToString(name, input string) (string, error) {
	c, ok := conversions.Get(name)
	if !ok {
		return "", fmt.Errorf("internal error: converter %q not found", name)
	}
	out, err := c.Convert(input)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(out, "\n"), nil
}

func isPureDecimal(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// resolveConverter maps (from, to) to an internal converter name.
// This mirrors the matrix in cmd/dispatch.go but lives here to avoid cycles.
// File-based names are prefixed with "*".
func resolveConverter(from, to string) (string, error) {
	type key struct{ from, to string }
	matrix := map[key]string{
		// formats
		{"json", "yaml"}: "json-yaml",
		{"json", "toml"}: "json-toml",
		{"json", "xml"}:  "json-xml",
		{"json", "csv"}:  "json-csv",
		{"json", "json"}: "json-pretty",
		{"yaml", "json"}: "yaml-json",
		{"toml", "json"}: "toml-json",
		{"xml", "json"}:  "xml-json",
		{"csv", "json"}:  "csv-json",
		{"csv", "xlsx"}:  "*csv-xlsx",
		{"xlsx", "csv"}:  "*xlsx-csv",
		// encoding
		{"text", "base64"}: "base64-encode",
		{"base64", "text"}: "base64-decode",
		{"text", "hex"}:    "hex-encode",
		{"hex", "text"}:    "hex-decode",
		{"text", "url"}:    "url-encode",
		{"url", "text"}:    "url-decode",
		{"text", "html"}:      "html-encode",
		{"html", "text"}:      "html-decode",
		{"text", "morsecode"}: "morse-encode",
		{"morsecode", "text"}: "morse-decode",
		// hashing
		{"text", "md5"}:    "hash-md5",
		{"text", "sha1"}:   "hash-sha1",
		{"text", "sha256"}: "hash-sha256",
		{"text", "sha512"}: "hash-sha512",
		// compression
		{"text", "gzip"}: "gzip-compress",
		{"gzip", "text"}: "gzip-decompress",
		{"text", "zlib"}: "zlib-compress",
		{"zlib", "text"}: "zlib-decompress",
		// numbers
		{"text", "binary"}:    "dec-bin",
		{"binary", "text"}:    "bin-dec",
		{"binary", "decimal"}: "bin-dec",
		{"text", "octal"}:     "dec-oct",
		{"octal", "text"}:     "oct-dec",
		{"octal", "decimal"}:  "oct-dec",
		{"text", "decimal"}:   "", // passthrough
		{"decimal", "binary"}: "dec-bin",
		{"decimal", "octal"}:  "dec-oct",
		{"decimal", "hex"}:    "dec-hex",
		{"hex", "decimal"}:    "hex-dec",
		// identity
		{"epoch", "text"}: "epoch-human",
		{"text", "epoch"}: "human-epoch",
		{"text", "uuid"}:  "uuid-generate",
	}

	// Case conversions: any case type → any other case type
	caseTypes := []string{"text", "camel", "pascal", "snake", "kebab", "screaming-snake", "screaming-kebab"}
	caseConverters := map[string]string{
		"camel":          "case-camel",
		"pascal":         "case-pascal",
		"snake":          "case-snake",
		"kebab":          "case-kebab",
		"screaming-snake": "case-screaming-snake",
		"screaming-kebab": "case-screaming-kebab",
	}
	for _, f := range caseTypes {
		for _, t := range caseTypes[1:] { // skip "text" as target
			k := key{f, t}
			if _, exists := matrix[k]; !exists {
				if name, ok := caseConverters[t]; ok {
					matrix[k] = name
				}
			}
		}
	}

	name, ok := matrix[key{from, to}]
	if !ok {
		return "", fmt.Errorf(
			"no conversion available from %q to %q\n"+
				"run 'atob list' to see all supported conversions",
			from, to,
		)
	}
	return name, nil
}
