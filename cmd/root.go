// Package cmd implements the atob CLI using cobra.
// All converter packages are imported here (blank imports) so their init()
// functions run and register converters into the global registry.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wingitman/atob/internal/config"
	"github.com/wingitman/atob/tui"

	// --- register all converters via init() ---
	_ "github.com/wingitman/atob/conversions/binary"
	_ "github.com/wingitman/atob/conversions/case"
	_ "github.com/wingitman/atob/conversions/compression"
	_ "github.com/wingitman/atob/conversions/encoding"
	_ "github.com/wingitman/atob/conversions/formats"
	_ "github.com/wingitman/atob/conversions/hash"
	_ "github.com/wingitman/atob/conversions/identity"
	_ "github.com/wingitman/atob/conversions/numbers"
)

var jsonOutput bool
var pickerOutput bool
var recordUpdate bool
var updateCommit string
var updateRepo string

var (
	buildVersion   = "dev"
	buildTimestamp = "unknown"
)

// SetVersion is called from main.go to inject the version and build time
// stamped in at compile time via -ldflags.
func SetVersion(version, timestamp string) {
	buildVersion = version
	buildTimestamp = timestamp
	rootCmd.Version = fmt.Sprintf("%s (built %s)", version, timestamp)
	tui.SetVersion(version)
}

// Execute is the entrypoint called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "atob",
	Short: "atob — convert anything to anything",
	Long: `atob — a universal conversion tool.

Usage:
  atob '<input>' <target>             auto-detect input type, convert to target
  atob '<input>' <from> <to>          explicit source and target types
  echo '<input>' | atob <target>      stdin variant
  echo '<input>' | atob <from> <to>   stdin with explicit types
  atob <from> <to> in.file out.file   file-based conversions (xlsx, csv)

Examples:
  atob 'hello world' base64
  atob '{"a":1}' yaml
  atob 'hello_world' snake camel
  echo '{"a":1}' | atob toml
  atob csv xlsx data.csv data.xlsx

Run 'atob list' to see all supported conversions.`,
	Args:          cobra.ArbitraryArgs,
	RunE:          runConvert,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all supported conversions",
	RunE:  runList,
}

func init() {
	listCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	listCmd.Flags().BoolVar(&pickerOutput, "picker", false, "Output deduplicated picker JSON (used by atob.nvim)")
	rootCmd.Flags().BoolVar(&recordUpdate, "record-update", false, "record update metadata")
	rootCmd.Flags().StringVar(&updateCommit, "update-commit", "", "installed update commit")
	rootCmd.Flags().StringVar(&updateRepo, "update-repo", "", "update source repo path")
	_ = rootCmd.Flags().MarkHidden("record-update")
	_ = rootCmd.Flags().MarkHidden("update-commit")
	_ = rootCmd.Flags().MarkHidden("update-repo")
	rootCmd.AddCommand(listCmd)
}

// listEntry is emitted by atob list --json for machine consumption.
type listEntry struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Internal  string `json:"internal"`
	FileBased bool   `json:"file_based"`
}

// pickerEntry is emitted by atob list --picker for atob.nvim.
// Case conversions are collapsed to a single "any → <case>" entry per target,
// and no-op / internal-only pairs are excluded.
type pickerEntry struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Label       string `json:"label"`
	Description string `json:"description"`
	FileBased   bool   `json:"file_based"`
}

// runList prints all supported conversions grouped by category.
func runList(cmd *cobra.Command, args []string) error {
	if pickerOutput {
		return runPickerList()
	}

	if jsonOutput {
		entries := make([]listEntry, 0, len(matrix))
		for key, internal := range matrix {
			if internal == "" {
				continue // skip no-op entries
			}
			entries = append(entries, listEntry{
				From:      key.from,
				To:        key.to,
				Internal:  strings.TrimPrefix(internal, "*"),
				FileBased: strings.HasPrefix(internal, "*"),
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	fmt.Print(`FORMAT CONVERSIONS
  json  →  yaml, toml, xml, csv, json (pretty-print)
  yaml  →  json
  toml  →  json
  xml   →  json
  csv   →  json, xlsx  [file]
  xlsx  →  csv         [file]

ENCODING
  text    →  base64, hex, url, html
  base64  →  text
  hex     →  text
  url     →  text
  html    →  text

HASHING  (input always treated as plain text)
  text  →  md5, sha1, sha256, sha512

COMPRESSION  (binary-safe via base64 wrapping)
  text  →  gzip, zlib
  gzip  →  text
  zlib  →  text

CASE
  any  →  camel, pascal, snake, kebab, screaming-snake, screaming-kebab

NUMBERS
  text    →  binary, octal, hex
  binary  →  text
  octal   →  text
  hex     →  text

IDENTITY
  text   →  uuid   (generates a new UUID v4, ignores input)
  text   →  epoch  (datetime string → unix timestamp)
  epoch  →  text   (unix timestamp → human datetime)

BINARY  (accepts a file path or stdin pipe)
  <file>  →  inspect  (auto-detect type, return JSON metadata)
  <file>  →  hexdump  (hex dump with offsets and ASCII panel)
  <file>  →  strings  (extract printable strings)

  Supported formats for inspect:
    ELF (Linux/BSD), PE (Windows), Mach-O (macOS)
    ZIP, TAR, .tar.gz, .tar.bz2
    PNG, JPEG, GIF, WebP, TIFF  (includes EXIF)
    MessagePack, CBOR

Run 'atob list --json' for machine-readable output.
`)
	return nil
}

// runConvert is the main dispatch handler. It parses the positional arguments
// into (input, from, to, filePaths) and calls Run().
//
// Accepted patterns:
//
//	atob <to>                        stdin → auto-detect → to
//	atob <from> <to>                 stdin, explicit from → to
//	atob <from> <to> f1 f2           file-based
//	atob '<input>' <to>              inline input, auto-detect from
//	atob '<input>' <from> <to>       inline input, explicit from → to
//	atob '<input>' <from> <to> f1 f2 inline input, file-based
//
// runPickerList emits a deduplicated JSON array of picker entries for atob.nvim.
// Case conversions are collapsed: instead of emitting every (snake→camel),
// (camel→snake), (pascal→kebab) … pair, we emit one entry per case target
// with from="any". File-based and no-op entries are handled appropriately.
func runPickerList() error {
	// Descriptions for well-known (from, to) pairs shown in the picker.
	descriptions := map[conversionKey]string{
		{TypeJSON, TypeYAML}:   "Convert JSON to YAML",
		{TypeJSON, TypeTOML}:   "Convert JSON to TOML",
		{TypeJSON, TypeXML}:    "Convert JSON to XML",
		{TypeJSON, TypeCSV}:    "Convert JSON array to CSV",
		{TypeJSON, TypeJSON}:   "Pretty-print JSON",
		{TypeYAML, TypeJSON}:   "Convert YAML to JSON",
		{TypeTOML, TypeJSON}:   "Convert TOML to JSON",
		{TypeXML, TypeJSON}:    "Convert XML to JSON",
		{TypeCSV, TypeJSON}:    "Convert CSV to JSON",
		{TypeCSV, TypeXLSX}:    "Convert CSV file to XLSX",
		{TypeXLSX, TypeCSV}:    "Convert XLSX file to CSV",
		{TypeText, TypeBase64}: "Encode text to Base64",
		{TypeBase64, TypeText}: "Decode Base64 to text",
		{TypeText, TypeHex}:    "Hex-encode text",
		{TypeHex, TypeText}:    "Hex-decode to text",
		{TypeText, TypeURL}:    "URL-encode text",
		{TypeURL, TypeText}:    "URL-decode text",
		{TypeText, TypeHTML}:   "HTML-encode special characters",
		{TypeHTML, TypeText}:   "HTML-decode entities",
		{TypeText, TypeMD5}:    "Hash text with MD5",
		{TypeText, TypeSHA1}:   "Hash text with SHA-1",
		{TypeText, TypeSHA256}: "Hash text with SHA-256",
		{TypeText, TypeSHA512}: "Hash text with SHA-512",
		{TypeText, TypeGzip}:   "Gzip-compress text (base64 output)",
		{TypeGzip, TypeText}:   "Gzip-decompress (base64 input)",
		{TypeText, TypeZlib}:   "Zlib-compress text (base64 output)",
		{TypeZlib, TypeText}:   "Zlib-decompress (base64 input)",
		{TypeText, TypeBinary}: "Convert decimal to binary",
		{TypeBinary, TypeText}: "Convert binary to decimal",
		{TypeText, TypeOctal}:  "Convert decimal to octal",
		{TypeOctal, TypeText}:  "Convert octal to decimal",
		{TypeDecimal, TypeHex}: "Convert decimal to hex",
		{TypeHex, TypeDecimal}: "Convert hex to decimal",
		{TypeEpoch, TypeText}:  "Unix epoch → human datetime",
		{TypeText, TypeEpoch}:  "Datetime string → Unix epoch",
		{TypeText, TypeUUID}:   "Generate a new UUID v4",
	}

	caseDescriptions := map[string]string{
		TypeCamel:          "Convert text to camelCase",
		TypePascal:         "Convert text to PascalCase",
		TypeSnake:          "Convert text to snake_case",
		TypeKebab:          "Convert text to kebab-case",
		TypeScreamingSnake: "Convert text to SCREAMING_SNAKE_CASE",
		TypeScreamingKebab: "Convert text to SCREAMING-KEBAB-CASE",
	}

	var entries []pickerEntry

	// Emit non-case, non-no-op entries in a stable order.
	orderedKeys := []conversionKey{
		{TypeJSON, TypeYAML}, {TypeJSON, TypeTOML}, {TypeJSON, TypeXML},
		{TypeJSON, TypeCSV}, {TypeJSON, TypeJSON},
		{TypeYAML, TypeJSON}, {TypeTOML, TypeJSON}, {TypeXML, TypeJSON},
		{TypeCSV, TypeJSON}, {TypeCSV, TypeXLSX}, {TypeXLSX, TypeCSV},
		{TypeText, TypeBase64}, {TypeBase64, TypeText},
		{TypeText, TypeHex}, {TypeHex, TypeText},
		{TypeText, TypeURL}, {TypeURL, TypeText},
		{TypeText, TypeHTML}, {TypeHTML, TypeText},
		{TypeText, TypeMD5}, {TypeText, TypeSHA1},
		{TypeText, TypeSHA256}, {TypeText, TypeSHA512},
		{TypeText, TypeGzip}, {TypeGzip, TypeText},
		{TypeText, TypeZlib}, {TypeZlib, TypeText},
		{TypeText, TypeBinary}, {TypeBinary, TypeText},
		{TypeText, TypeOctal}, {TypeOctal, TypeText},
		{TypeDecimal, TypeHex}, {TypeHex, TypeDecimal},
		{TypeEpoch, TypeText}, {TypeText, TypeEpoch},
		{TypeText, TypeUUID},
	}

	for _, key := range orderedKeys {
		internal, ok := matrix[key]
		if !ok || internal == "" {
			continue
		}
		desc := descriptions[key]
		label := fmt.Sprintf("%s → %s", key.from, key.to)
		entries = append(entries, pickerEntry{
			From:        key.from,
			To:          key.to,
			Label:       label,
			Description: desc,
			FileBased:   strings.HasPrefix(internal, "*"),
		})
	}

	// Binary entries
	binaryEntries := []pickerEntry{
		{From: "file", To: TypeInspect, Label: "file → inspect", Description: "Auto-detect binary format, return JSON metadata", FileBased: false},
		{From: "file", To: TypeHexdump, Label: "file → hexdump", Description: "Hex dump with offsets and ASCII panel", FileBased: false},
		{From: "file", To: TypeStrings, Label: "file → strings", Description: "Extract printable strings from binary", FileBased: false},
	}
	entries = append(entries, binaryEntries...)

	// Emit one entry per case target (from="any").
	caseOrder := []string{
		TypeCamel, TypePascal, TypeSnake, TypeKebab,
		TypeScreamingSnake, TypeScreamingKebab,
	}
	for _, to := range caseOrder {
		entries = append(entries, pickerEntry{
			From:        "any",
			To:          to,
			Label:       fmt.Sprintf("any → %s", to),
			Description: caseDescriptions[to],
			FileBased:   false,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func runConvert(cobraCmd *cobra.Command, args []string) error {
	if recordUpdate {
		return config.RecordUpdateMetadata(updateCommit, updateRepo)
	}

	// ── No arguments → launch interactive TUI ────────────────────────────────
	if len(args) == 0 {
		cfg, _ := config.Load()
		return tui.Start(cfg, tui.PreloadNone)
	}

	// ── Single arg that is a readable file path → TUI with file pre-loaded ───
	// e.g. `atob ./myfile.json` or `atob /usr/bin/ls`
	if len(args) == 1 {
		_, firstIsType := ResolveType(args[0])
		if !firstIsType {
			if info, err := os.Stat(args[0]); err == nil && !info.IsDir() {
				cfg, _ := config.Load()
				return tui.Start(cfg, tui.PreloadFile(args[0]))
			}
		}
	}

	// ── Binary targets (inspect, hexdump, strings) ────────────────────────────
	// These consume raw bytes from either a file path or stdin.
	// Arg patterns:
	//   atob /path/to/file <target>
	//   cat file | atob <target>
	if to, ok := parseBinaryArgs(args); ok {
		return runBinary(args, to)
	}

	stdinData, stdinErr := readStdin()

	input, from, to, filePaths, err := parseArgs(args, stdinData, stdinErr)
	if err != nil {
		return err
	}

	// One-way targets always treat input as plain text — skip detection.
	if oneWayTargets[to] {
		from = TypeText
	}

	// Auto-detect from if not supplied.
	if from == "" {
		from, err = Detect(input)
		if err != nil {
			return err
		}
	}

	return Run(input, from, to, filePaths)
}

// parseBinaryArgs checks whether the args form a binary conversion pattern and
// returns the target type. It returns ("", false) if this is not a binary call.
//
// Binary patterns:
//
//	atob <target>               — stdin pipe, target is a binary target word
//	atob <filepath> <target>    — file path + binary target word
func parseBinaryArgs(args []string) (to string, ok bool) {
	// Single arg: atob <target>  — only valid if it's a binary target
	if len(args) == 1 {
		if t, resolved := ResolveType(args[0]); resolved && binaryTargets[t] {
			return t, true
		}
		return "", false
	}

	// Two args: atob <filepath> <target>
	// first arg is not a type word (file path), second is a binary target
	if len(args) == 2 {
		_, firstIsType := ResolveType(args[0])
		t, secondIsType := ResolveType(args[1])
		if !firstIsType && secondIsType && binaryTargets[t] {
			return t, true
		}
	}

	return "", false
}

// runBinary reads raw bytes from either a file path or stdin, then runs the
// named BinaryConverter.
func runBinary(args []string, to string) error {
	var data []byte
	var err error

	if len(args) == 2 {
		// File path provided
		data, err = os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("cannot read file %q: %w", args[0], err)
		}
	} else {
		// Read from stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return fmt.Errorf(
				"no input: pipe a file to stdin or provide a file path\n"+
					"  cat file.bin | atob %s\n"+
					"  atob /path/to/file %s",
				to, to,
			)
		}
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
	}

	return RunBinary(data, to)
}

// parseArgs decodes the positional argument list into components.
// A word is treated as a type token when it resolves against the alias table;
// otherwise it is the input value.
func parseArgs(args []string, stdinData string, stdinErr error) (input, from, to string, filePaths []string, err error) {
	// Resolve each arg to a canonical type, or "" if it's not a type word.
	resolved := make([]string, len(args))
	for i, a := range args {
		t, _ := ResolveType(a)
		resolved[i] = t
	}

	switch {
	// ── first arg is a type word → input comes from stdin or file paths ───
	case resolved[0] != "":
		switch {
		case len(args) == 1:
			// atob <to>  — needs stdin
			if stdinErr != nil {
				return "", "", "", nil, stdinErr
			}
			if stdinData == "" {
				return "", "", "", nil, fmt.Errorf(
					"no input: pipe text to stdin or pass it as the first argument\n"+
						"  echo 'hello' | atob %s\n"+
						"  atob 'hello' %s",
					args[0], args[0],
				)
			}
			input = stdinData
			to = resolved[0]

		case len(args) >= 2 && resolved[1] != "" && len(args) >= 4:
			// atob <from> <to> file1 file2  — file-based, no stdin needed
			from = resolved[0]
			to = resolved[1]
			filePaths = args[2:]
			// input unused for file converters but set to empty string
			input = ""

		case len(args) >= 2 && resolved[1] != "":
			// atob <from> <to>  — needs stdin
			if stdinErr != nil {
				return "", "", "", nil, stdinErr
			}
			if stdinData == "" {
				return "", "", "", nil, fmt.Errorf(
					"no input: pipe text to stdin or pass it as the first argument\n"+
						"  echo 'hello' | atob %s %s\n"+
						"  atob 'hello' %s %s",
					args[0], args[1], args[0], args[1],
				)
			}
			input = stdinData
			from = resolved[0]
			to = resolved[1]
			filePaths = args[2:]

		default:
			return "", "", "", nil, badArgError(args)
		}

	// ── first arg is an input value ────────────────────────────────────────
	case resolved[0] == "" && len(args) >= 2:
		input = args[0]
		switch {
		case len(args) == 2 && resolved[1] != "":
			// atob '<input>' <to>
			to = resolved[1]
		case len(args) >= 3 && resolved[1] != "" && resolved[2] != "":
			// atob '<input>' <from> <to> [file1 file2]
			from = resolved[1]
			to = resolved[2]
			filePaths = args[3:]
		default:
			return "", "", "", nil, badArgError(args)
		}

	default:
		return "", "", "", nil, badArgError(args)
	}

	return input, from, to, filePaths, nil
}

// badArgError produces a helpful error for unrecognised argument patterns.
func badArgError(args []string) error {
	return fmt.Errorf(
		"unrecognised argument pattern: %s\n\n"+
			"usage:\n"+
			"  atob '<input>' <target>\n"+
			"  atob '<input>' <from> <to>\n"+
			"  echo '<input>' | atob <target>\n"+
			"  echo '<input>' | atob <from> <to>\n\n"+
			"run 'atob list' to see all type names and supported conversions",
		shellQuote(args),
	)
}

func shellQuote(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\n") {
			quoted[i] = "'" + a + "'"
		} else {
			quoted[i] = a
		}
	}
	return strings.Join(quoted, " ")
}

// readStdin reads from stdin only when it is a pipe (not an interactive TTY).
// Returns ("", nil) when stdin is a TTY.
func readStdin() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", nil
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return "", nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	return string(b), nil
}
