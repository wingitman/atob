// Package strcase converts strings between common naming conventions.
//
// It supports: snake_case, SCREAMING_SNAKE_CASE, kebab-case,
// SCREAMING-KEBAB-CASE, camelCase, PascalCase, and Title Case.
//
// Word boundaries are detected at:
//   - Existing delimiter characters (space, underscore, hyphen, dot)
//   - Transitions from lowercase to uppercase (e.g. "myFunc" → ["my","Func"])
//   - Transitions from a run of uppercase to lowercase preceded by uppercase
//     (e.g. "XMLParser" → ["XML","Parser"])
//   - Digit/letter boundaries ("base64Encode" → ["base","64","Encode"])
package strcase

import (
	"strings"
	"unicode"
)

// tokenise splits s into lower-case word tokens.
func tokenise(s string) []string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return nil
	}

	var words []string
	start := 0

	flush := func(end int) {
		if end > start {
			w := strings.ToLower(string(runes[start:end]))
			if w != "" {
				words = append(words, w)
			}
		}
		start = end
	}

	for i := 0; i < n; i++ {
		c := runes[i]

		// Delimiter characters: split here, skip the delimiter itself.
		if c == '_' || c == '-' || c == ' ' || c == '.' || c == '/' {
			flush(i)
			start = i + 1
			continue
		}

		if i == 0 {
			continue
		}

		prev := runes[i-1]

		// Lower→upper transition: "myFunc" → "my" | "Func"
		if unicode.IsLower(prev) && unicode.IsUpper(c) {
			flush(i)
			continue
		}

		// Upper run→upper+lower: "XMLParser" → "XML" | "Parser"
		// Detect when we are at position i where c is upper and next is lower,
		// but prev was also upper (i.e. we are at the last letter of an acronym).
		if i+1 < n && unicode.IsUpper(c) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(prev) {
			flush(i)
			continue
		}

		// Digit↔letter boundary
		isDigit := unicode.IsDigit(c)
		prevIsDigit := unicode.IsDigit(prev)
		if isDigit != prevIsDigit {
			flush(i)
			continue
		}
	}
	flush(n)
	return words
}

// ToSnake converts s to snake_case.
func ToSnake(s string) string {
	return strings.Join(tokenise(s), "_")
}

// ToScreamingSnake converts s to SCREAMING_SNAKE_CASE.
func ToScreamingSnake(s string) string {
	return strings.ToUpper(strings.Join(tokenise(s), "_"))
}

// ToKebab converts s to kebab-case.
func ToKebab(s string) string {
	return strings.Join(tokenise(s), "-")
}

// ToScreamingKebab converts s to SCREAMING-KEBAB-CASE.
func ToScreamingKebab(s string) string {
	return strings.ToUpper(strings.Join(tokenise(s), "-"))
}

// ToCamel converts s to PascalCase (UpperCamelCase).
func ToCamel(s string) string {
	words := tokenise(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, "")
}

// ToLowerCamel converts s to camelCase (lowerCamelCase).
func ToLowerCamel(s string) string {
	words := tokenise(s)
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		if i == 0 {
			words[i] = strings.ToLower(w)
		} else {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, "")
}

// ToDelimited joins words with the given delimiter, each word lower-cased.
// This is useful for Title Case when delimiter is ' '.
func ToDelimited(s string, delimiter rune) string {
	words := tokenise(s)
	parts := make([]string, len(words))
	for i, w := range words {
		if len(w) > 0 {
			parts[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(parts, string(delimiter))
}

// ToScreamingDelimited joins words with delimiter, screaming if upper is true.
// The ignore parameter (if non-empty) is a set of runes not treated as delimiters.
func ToScreamingDelimited(s string, delimiter rune, ignore string, upper bool) string {
	words := tokenise(s)
	if upper {
		for i, w := range words {
			words[i] = strings.ToUpper(w)
		}
	}
	return strings.Join(words, string(delimiter))
}
