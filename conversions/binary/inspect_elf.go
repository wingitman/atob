package binary

import (
	"bytes"
	"debug/elf"
	"encoding/json"
	"fmt"
	"strings"
)

// elfInfo holds the structured output for an ELF binary.
type elfInfo struct {
	Type         string   `json:"type"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	Bits         int      `json:"bits"`
	Endian       string   `json:"endian"`
	EntryPoint   string   `json:"entry_point"`
	Language     string   `json:"language"`
	GoVersion    string   `json:"go_version,omitempty"`
	BuildID      string   `json:"build_id,omitempty"`
	Sections     []string `json:"sections"`
	ImportedLibs []string `json:"imported_libs"`
	Stripped     bool     `json:"stripped"`
	FileSize     int      `json:"file_size"`
}

// inspectELF parses an ELF binary and returns structured JSON.
func inspectELF(data []byte) (string, error) {
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("not a valid ELF file: %w", err)
	}
	defer f.Close()

	info := elfInfo{
		Type:     "ELF",
		FileSize: len(data),
	}

	// OS / ABI
	info.OS = f.OSABI.String()

	// Architecture + bits + endian
	switch f.Machine {
	case elf.EM_X86_64:
		info.Arch = "x86-64"
	case elf.EM_386:
		info.Arch = "x86"
	case elf.EM_AARCH64:
		info.Arch = "arm64"
	case elf.EM_ARM:
		info.Arch = "arm"
	case elf.EM_RISCV:
		info.Arch = "riscv"
	case elf.EM_MIPS:
		info.Arch = "mips"
	case elf.EM_PPC64:
		info.Arch = "ppc64"
	case elf.EM_S390:
		info.Arch = "s390"
	default:
		info.Arch = f.Machine.String()
	}

	switch f.Class {
	case elf.ELFCLASS32:
		info.Bits = 32
	case elf.ELFCLASS64:
		info.Bits = 64
	}

	switch f.ByteOrder.String() {
	case "LittleEndian":
		info.Endian = "little"
	case "BigEndian":
		info.Endian = "big"
	}

	info.EntryPoint = fmt.Sprintf("0x%x", f.Entry)

	// Sections
	hasSymtab := false
	for _, s := range f.Sections {
		if s.Name == "" {
			continue
		}
		info.Sections = append(info.Sections, s.Name)
		if s.Name == ".symtab" {
			hasSymtab = true
		}
	}
	info.Stripped = !hasSymtab

	// Imported libraries
	libs, _ := f.ImportedLibraries()
	info.ImportedLibs = libs
	if info.ImportedLibs == nil {
		info.ImportedLibs = []string{}
	}

	// Language detection
	info.Language, info.GoVersion, info.BuildID = detectELFLanguage(f)

	out, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

// detectELFLanguage heuristically identifies the source language of an ELF.
func detectELFLanguage(f *elf.File) (lang, goVersion, buildID string) {
	// Go: look for .note.go.buildid section or Go-specific note
	for _, s := range f.Sections {
		if s.Name == ".note.go.buildid" {
			data, err := s.Data()
			if err == nil {
				buildID = extractNoteString(data)
			}
			lang = "Go"
		}
	}

	// Try Go build info embedded in the binary
	if goVer, id := readGoBuildInfo(f); goVer != "" {
		lang = "Go"
		goVersion = goVer
		if buildID == "" {
			buildID = id
		}
	}

	if lang == "Go" {
		return
	}

	// Rust: look for .note.rustc section
	for _, s := range f.Sections {
		if strings.Contains(s.Name, "rust") {
			return "Rust", "", ""
		}
	}

	// Check imported symbols for language clues
	syms, _ := f.ImportedSymbols()
	hasCPP := false
	for _, sym := range syms {
		if strings.HasPrefix(sym.Name, "_ZN") || strings.Contains(sym.Library, "stdc++") || strings.Contains(sym.Library, "c++") {
			hasCPP = true
			break
		}
	}
	if hasCPP {
		return "C++", "", ""
	}

	libs, _ := f.ImportedLibraries()
	for _, lib := range libs {
		if strings.Contains(lib, "stdc++") || strings.Contains(lib, "libc++") {
			return "C++", "", ""
		}
	}

	return "C", "", ""
}

// readGoBuildInfo finds the Go version and build ID embedded in the binary.
func readGoBuildInfo(f *elf.File) (goVersion, buildID string) {
	// Go embeds a build info structure starting with "\xff Go buildinf:"
	// We look in .rodata or .data sections.
	for _, s := range f.Sections {
		if s.Name != ".rodata" && s.Name != ".data" && s.Name != ".go.buildinfo" {
			continue
		}
		data, err := s.Data()
		if err != nil {
			continue
		}
		if v, id := parseGoBuildInfoBytes(data); v != "" {
			return v, id
		}
	}
	return "", ""
}

func parseGoBuildInfoBytes(data []byte) (goVersion, buildID string) {
	magic := []byte("\xff Go buildinf:")
	idx := bytes.Index(data, magic)
	if idx < 0 {
		// Also try the Go build info magic for older versions
		magic2 := []byte("go1.")
		idx = bytes.Index(data, magic2)
		if idx < 0 {
			return "", ""
		}
		end := idx + 2
		for end < len(data) && end < idx+20 {
			if data[end] == 0 || data[end] == ' ' {
				break
			}
			end++
		}
		return string(data[idx:end]), ""
	}
	// Parse the build info structure
	start := idx + len(magic)
	if start+32 > len(data) {
		return "", ""
	}
	// Version string follows; find NUL terminator
	end := start
	for end < len(data) && data[end] != 0 && end < start+64 {
		end++
	}
	return string(data[start:end]), ""
}

func extractNoteString(data []byte) string {
	if len(data) < 16 {
		return ""
	}
	// ELF note: namesz (4) + descsz (4) + type (4) + name + desc
	if len(data) < 12 {
		return ""
	}
	namesz := int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24
	descsz := int(data[4]) | int(data[5])<<8 | int(data[6])<<16 | int(data[7])<<24
	start := 12 + ((namesz + 3) &^ 3)
	end := start + descsz
	if end > len(data) {
		return ""
	}
	return strings.TrimRight(string(data[start:end]), "\x00")
}
