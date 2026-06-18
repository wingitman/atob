package cmd

import "strings"

// Canonical type identifiers used throughout the dispatch layer.
// These are the normalised names — all user input is resolved to one of these.
const (
	TypeJSON           = "json"
	TypeYAML           = "yaml"
	TypeTOML           = "toml"
	TypeXML            = "xml"
	TypeCSV            = "csv"
	TypeXLSX           = "xlsx"
	TypeBase64         = "base64"
	TypeHex            = "hex"
	TypeURL            = "url"
	TypeHTML           = "html"
	TypeBinary         = "binary"
	TypeOctal          = "octal"
	TypeText           = "text"
	TypeMD5            = "md5"
	TypeSHA1           = "sha1"
	TypeSHA256         = "sha256"
	TypeSHA512         = "sha512"
	TypeGzip           = "gzip"
	TypeZlib           = "zlib"
	TypeUUID           = "uuid"
	TypeEpoch          = "epoch"
	TypeDecimal        = "decimal"
	TypeInspect        = "inspect"
	TypeHexdump        = "hexdump"
	TypeStrings        = "strings"
	TypeDecompile      = "decompile"
	TypeMsgpack        = "msgpack"
	TypeCBOR           = "cbor"
	TypeCamel          = "camel"
	TypePascal         = "pascal"
	TypeSnake          = "snake"
	TypeKebab          = "kebab"
	TypeScreamingSnake = "screaming-snake"
	TypeScreamingKebab = "screaming-kebab"
	TypeMorsecode      = "morsecode"
)

// aliases maps every accepted user-facing word to its canonical type.
var aliases = map[string]string{
	// json
	"json": TypeJSON,
	"js":   TypeJSON,
	// yaml
	"yaml": TypeYAML,
	"yml":  TypeYAML,
	// toml
	"toml": TypeTOML,
	// xml
	"xml": TypeXML,
	// csv
	"csv": TypeCSV,
	// xlsx
	"xlsx":  TypeXLSX,
	"excel": TypeXLSX,
	"xls":   TypeXLSX,
	// base64
	"base64": TypeBase64,
	"b64":    TypeBase64,
	// hex
	"hex":         TypeHex,
	"hexadecimal": TypeHex,
	// url
	"url":         TypeURL,
	"urlencode":   TypeURL,
	"percentcode": TypeURL,
	// html
	"html":     TypeHTML,
	"entities": TypeHTML,
	// binary
	"binary": TypeBinary,
	"bin":    TypeBinary,
	// octal
	"octal": TypeOctal,
	"oct":   TypeOctal,
	// text / plain
	"text":   TypeText,
	"plain":  TypeText,
	"str":    TypeText,
	"string": TypeText,
	"raw":    TypeText,
	// hashes
	"md5":    TypeMD5,
	"sha1":   TypeSHA1,
	"sha256": TypeSHA256,
	"sha512": TypeSHA512,
	// compression
	"gzip": TypeGzip,
	"gz":   TypeGzip,
	"zlib": TypeZlib,
	// uuid
	"uuid": TypeUUID,
	"guid": TypeUUID,
	// epoch / timestamp
	"epoch":     TypeEpoch,
	"timestamp": TypeEpoch,
	"ts":        TypeEpoch,
	"unix":      TypeEpoch,
	// decimal number
	"decimal": TypeDecimal,
	"dec":     TypeDecimal,
	"number":  TypeDecimal,
	"num":     TypeDecimal,
	"int":     TypeDecimal,
	// binary file targets
	"inspect":  TypeInspect,
	"info":     TypeInspect,
	"describe": TypeInspect,
	"hexdump":  TypeHexdump,
	"xxd":      TypeHexdump,
	"dump":     TypeHexdump,
	"strings":  TypeStrings,
	"strs":     TypeStrings,
	"decompile":       TypeDecompile,
	"decomp":          TypeDecompile,
	"lambda-decompile": TypeDecompile,
	"lambda":          TypeDecompile,
	"unpacker":        TypeDecompile,
	"msgpack":  TypeMsgpack,
	"mp":       TypeMsgpack,
	"cbor":     TypeCBOR,
	// morse code
	"morsecode": TypeMorsecode,
	"morse":     TypeMorsecode,
	// case styles
	"camel":          TypeCamel,
	"camelcase":      TypeCamel,
	"pascal":         TypePascal,
	"pascalcase":     TypePascal,
	"uppercamel":     TypePascal,
	"snake":          TypeSnake,
	"snakecase":      TypeSnake,
	"kebab":          TypeKebab,
	"kebabcase":      TypeKebab,
	"screaming-snake": TypeScreamingSnake,
	"screamingsnake":  TypeScreamingSnake,
	"upper-snake":     TypeScreamingSnake,
	"uppersnake":      TypeScreamingSnake,
	"screaming-kebab": TypeScreamingKebab,
	"screamingkebab": TypeScreamingKebab,
	"upper-kebab":    TypeScreamingKebab,
	"upperkebab":     TypeScreamingKebab,
}

// oneWayTargets are types that always treat their input as plain text.
// When one of these is the target, auto-detection is skipped — the from is
// implicitly TypeText. This covers:
//   - hashes (md5, sha1, sha256, sha512) — always hash raw bytes
//   - compression (gzip, zlib) — always compress raw text
//   - uuid — generates a new UUID, ignores input entirely
//   - encoding targets (base64, hex, url, html) — encode the raw input string;
//     auto-detecting e.g. "<b>bold</b>" as XML and then refusing to html-encode
//     it would be wrong and confusing
//   - case targets — transform the raw string as-is
//   - epoch target — parse the raw datetime string
var oneWayTargets = map[string]bool{
	TypeMD5:            true,
	TypeSHA1:           true,
	TypeSHA256:         true,
	TypeSHA512:         true,
	TypeGzip:           true,
	TypeZlib:           true,
	TypeUUID:           true,
	TypeBase64:         true,
	TypeHex:            true,
	TypeURL:            true,
	TypeHTML:           true,
	TypeMorsecode:      true,
	TypeBinary:         true,
	TypeOctal:          true,
	TypeCamel:          true,
	TypePascal:         true,
	TypeSnake:          true,
	TypeKebab:          true,
	TypeScreamingSnake: true,
	TypeScreamingKebab: true,
	TypeEpoch:          true,
	// binary targets — input is always raw bytes, never auto-detected as text
	TypeInspect: true,
	TypeHexdump: true,
	TypeStrings: true,
	TypeDecompile: true,
}

// binaryTargets is the set of targets that require raw []byte input.
var binaryTargets = map[string]bool{
	TypeInspect: true,
	TypeHexdump: true,
	TypeStrings: true,
	TypeDecompile: true,
}

// caseTypes is the set of all case-style types.
var caseTypes = map[string]bool{
	TypeCamel:          true,
	TypePascal:         true,
	TypeSnake:          true,
	TypeKebab:          true,
	TypeScreamingSnake: true,
	TypeScreamingKebab: true,
}

// ResolveType normalises a user-supplied type word to a canonical type.
// Returns ("", false) if the word is not recognised.
func ResolveType(word string) (string, bool) {
	t, ok := aliases[strings.ToLower(strings.TrimSpace(word))]
	return t, ok
}
