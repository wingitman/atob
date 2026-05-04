package encoding

import (
	"fmt"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(morseEncoder{})
	conversions.Register(morseDecoder{})
}

// charToMorse maps uppercase letters, digits, and common punctuation to morse code.
var charToMorse = map[string]string{
	"A":  ".-",
	"B":  "-...",
	"C":  "-.-.",
	"D":  "-..",
	"E":  ".",
	"F":  "..-.",
	"G":  "--.",
	"H":  "....",
	"I":  "..",
	"J":  ".---",
	"K":  "-.-",
	"L":  ".-..",
	"M":  "--",
	"N":  "-.",
	"O":  "---",
	"P":  ".--.",
	"Q":  "--.-",
	"R":  ".-.",
	"S":  "...",
	"T":  "-",
	"U":  "..-",
	"V":  "...-",
	"W":  ".--",
	"X":  "-..-",
	"Y":  "-.--",
	"Z":  "--..",
	"0":  "-----",
	"1":  ".----",
	"2":  "..---",
	"3":  "...--",
	"4":  "....-",
	"5":  ".....",
	"6":  "-....",
	"7":  "--...",
	"8":  "---..",
	"9":  "----.",
	".":  ".-.-.-",
	",":  "--..--",
	"?":  "..--..",
	"'":  ".----.",
	"!":  "-.-.--",
	"/":  "-..-.",
	"(":  "-.--.",
	")":  "-.--.-",
	"&":  ".-...",
	":":  "---...",
	";":  "-.-.-.",
	"=":  "-...-",
	"+":  ".-.-.",
	"-":  "-....-",
	"_":  "..--.-",
	"\"": ".-..-.",
	"$":  "...-..-",
	"@":  ".--.-.",
}

// morseToChar is the reverse of charToMorse, built at init time.
var morseToChar map[string]string

func init() {
	morseToChar = make(map[string]string, len(charToMorse))
	for char, code := range charToMorse {
		morseToChar[code] = char
	}
}

// --- encoder ---

type morseEncoder struct{}

func (morseEncoder) Name() string        { return "morse-encode" }
func (morseEncoder) Category() string    { return "encoding" }
func (morseEncoder) Description() string { return "Encode text to Morse code" }

func (morseEncoder) Convert(input string) (string, error) {
	input = strings.TrimRight(input, "\n")
	words := strings.Fields(input)
	morseWords := make([]string, 0, len(words))
	for _, word := range words {
		letters := make([]string, 0, len(word))
		for _, r := range strings.ToUpper(word) {
			code, ok := charToMorse[string(r)]
			if !ok {
				// skip unrecognised characters
				continue
			}
			letters = append(letters, code)
		}
		if len(letters) > 0 {
			morseWords = append(morseWords, strings.Join(letters, " "))
		}
	}
	return strings.Join(morseWords, " / "), nil
}

// --- decoder ---

type morseDecoder struct{}

func (morseDecoder) Name() string        { return "morse-decode" }
func (morseDecoder) Category() string    { return "encoding" }
func (morseDecoder) Description() string { return "Decode Morse code to text" }

func (morseDecoder) Convert(input string) (string, error) {
	input = strings.TrimSpace(input)
	wordParts := strings.Split(input, " / ")
	decodedWords := make([]string, 0, len(wordParts))
	for _, wordPart := range wordParts {
		wordPart = strings.TrimSpace(wordPart)
		if wordPart == "" {
			continue
		}
		tokens := strings.Split(wordPart, " ")
		sb := strings.Builder{}
		for _, token := range tokens {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			char, ok := morseToChar[token]
			if !ok {
				return "", fmt.Errorf("unknown morse code token: %q", token)
			}
			sb.WriteString(char)
		}
		decodedWords = append(decodedWords, sb.String())
	}
	return strings.Join(decodedWords, " "), nil
}
