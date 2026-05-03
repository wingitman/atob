package cmd

import (
	"fmt"
	"strings"

	"github.com/wingitman/atob/conversions"
)

// conversionKey is a (from, to) pair used to look up the internal converter name.
type conversionKey struct{ from, to string }

// matrix maps (from, to) pairs to the internal converter name registered in
// the conversions registry. File-based converters are marked with a leading "*".
var matrix = map[conversionKey]string{
	// ── formats ──────────────────────────────────────────────────────────────
	{TypeJSON, TypeYAML}: "json-yaml",
	{TypeJSON, TypeTOML}: "json-toml",
	{TypeJSON, TypeXML}:  "json-xml",
	{TypeJSON, TypeCSV}:  "json-csv",
	{TypeJSON, TypeJSON}: "json-pretty",
	{TypeYAML, TypeJSON}: "yaml-json",
	{TypeTOML, TypeJSON}: "toml-json",
	{TypeXML, TypeJSON}:  "xml-json",
	{TypeCSV, TypeJSON}:  "csv-json",
	{TypeCSV, TypeXLSX}:  "*csv-xlsx",
	{TypeXLSX, TypeCSV}:  "*xlsx-csv",

	// ── encoding ─────────────────────────────────────────────────────────────
	{TypeText, TypeBase64}:   "base64-encode",
	{TypeBase64, TypeText}:   "base64-decode",
	{TypeText, TypeHex}:      "hex-encode",
	{TypeHex, TypeText}:      "hex-decode",
	{TypeText, TypeURL}:      "url-encode",
	{TypeURL, TypeText}:      "url-decode",
	{TypeText, TypeHTML}:     "html-encode",
	{TypeHTML, TypeText}:     "html-decode",

	// ── hashing (one-way, from is always text) ────────────────────────────────
	{TypeText, TypeMD5}:    "hash-md5",
	{TypeText, TypeSHA1}:   "hash-sha1",
	{TypeText, TypeSHA256}: "hash-sha256",
	{TypeText, TypeSHA512}: "hash-sha512",

	// ── compression (one-way) ─────────────────────────────────────────────────
	{TypeText, TypeGzip}: "gzip-compress",
	{TypeGzip, TypeText}: "gzip-decompress",
	{TypeText, TypeZlib}: "zlib-compress",
	{TypeZlib, TypeText}: "zlib-decompress",

	// ── numbers ───────────────────────────────────────────────────────────────
	{TypeText, TypeBinary}:       "dec-bin",
	{TypeBinary, TypeText}:       "bin-dec",
	{TypeBinary, TypeDecimal}:    "bin-dec",
	{TypeText, TypeOctal}:        "dec-oct",
	{TypeOctal, TypeText}:        "oct-dec",
	{TypeOctal, TypeDecimal}:     "oct-dec",
	{TypeText, TypeDecimal}:      "", // passthrough / no-op, handled in Run
	{TypeDecimal, TypeBinary}:    "dec-bin",
	{TypeDecimal, TypeOctal}:     "dec-oct",
	{TypeDecimal, TypeHex}:       "dec-hex",
	{TypeHex, TypeDecimal}:       "hex-dec",
	{TypeText, TypeHex}:          "hex-encode", // overwritten in init()
	{TypeHex, TypeText}:          "hex-decode",

	// ── identity ──────────────────────────────────────────────────────────────
	{TypeEpoch, TypeText}: "epoch-human",
	{TypeText, TypeEpoch}: "human-epoch",
	{TypeText, TypeUUID}:  "uuid-generate",

	// ── case styles — all pairs generated in init() ───────────────────────────
}

// caseConverters maps each case target type to its internal converter name.
var caseConverters = map[string]string{
	TypeCamel:          "case-camel",
	TypePascal:         "case-pascal",
	TypeSnake:          "case-snake",
	TypeKebab:          "case-kebab",
	TypeScreamingSnake: "case-screaming-snake",
	TypeScreamingKebab: "case-screaming-kebab",
}

func init() {
	// Register all case conversion pairs in the matrix.
	// Any case type can convert to any other case type.
	allCaseTypes := []string{
		TypeText, // plain text can also be converted to a case style
		TypeCamel, TypePascal, TypeSnake, TypeKebab,
		TypeScreamingSnake, TypeScreamingKebab,
	}
	for _, from := range allCaseTypes {
		for _, to := range []string{
			TypeCamel, TypePascal, TypeSnake, TypeKebab,
			TypeScreamingSnake, TypeScreamingKebab,
		} {
			if from == to {
				continue
			}
			key := conversionKey{from, to}
			if _, exists := matrix[key]; !exists {
				matrix[key] = caseConverters[to]
			}
		}
	}

	// hex-encode vs dec-hex disambiguation:
	// {TypeText, TypeHex} is entered twice above — overwrite to use hex-encode
	// (the numeric dec-hex converter is accessible via explicit "text hex" when
	// the input is a decimal number; detection returns TypeText for plain decimals).
	matrix[conversionKey{TypeText, TypeHex}] = "hex-encode"
}

// Run performs the conversion for the given (from, to) pair on input.
// For file-based converters the input is treated as two space-separated paths.
func Run(input, from, to string, filePaths []string) error {
	// Plain text / decimal passthrough
	if (from == TypeText || from == TypeDecimal) && (to == TypeText || to == TypeDecimal) {
		out := strings.TrimRight(input, "\n")
		fmt.Println(out)
		return nil
	}

	// Numeric disambiguation: when from=text/decimal and to=hex/octal/binary
	// and the input is a pure decimal integer, use the number-base converter.
	if (from == TypeText || from == TypeDecimal) && (to == TypeHex || to == TypeOctal || to == TypeBinary) {
		if isPureDecimal(strings.TrimSpace(input)) {
			switch to {
			case TypeHex:
				return runConverter("dec-hex", input)
			case TypeOctal:
				return runConverter("dec-oct", input)
			case TypeBinary:
				return runConverter("dec-bin", input)
			}
		}
	}

	key := conversionKey{from, to}
	converterName, ok := matrix[key]
	if !ok {
		return fmt.Errorf(
			"no conversion available from %q to %q\n"+
				"run 'atob list' to see all supported conversions",
			from, to,
		)
	}

	// ── file-based converter ─────────────────────────────────────────────────
	if strings.HasPrefix(converterName, "*") {
		name := converterName[1:]
		if len(filePaths) < 2 {
			return fmt.Errorf(
				"%s→%s conversion requires file paths:\n  atob %s %s <input-file> <output-file>",
				from, to, from, to,
			)
		}
		fc, ok := conversions.GetFile(name)
		if !ok {
			return fmt.Errorf("internal error: file converter %q not found", name)
		}
		return fc.ConvertFile(filePaths[0], filePaths[1])
	}

	// ── text-based converter ─────────────────────────────────────────────────
	return runConverter(converterName, input)
}

// runConverter looks up a text-based converter by internal name and runs it.
func runConverter(name, input string) error {
	c, ok := conversions.Get(name)
	if !ok {
		return fmt.Errorf("internal error: converter %q not found", name)
	}
	out, err := c.Convert(input)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	fmt.Print(out)
	return nil
}

// isPureDecimal returns true if s contains only ASCII digit characters.
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
