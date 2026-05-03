package binary

import (
	"bytes"
	"debug/macho"
	"encoding/json"
	"fmt"
	"strings"
)

type machoInfo struct {
	Type         string   `json:"type"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	Bits         int      `json:"bits"`
	Endian       string   `json:"endian"`
	EntryPoint   string   `json:"entry_point,omitempty"`
	Language     string   `json:"language"`
	GoVersion    string   `json:"go_version,omitempty"`
	Sections     []string `json:"sections"`
	ImportedLibs []string `json:"imported_libs"`
	Stripped     bool     `json:"stripped"`
	FileSize     int      `json:"file_size"`
}

func inspectMachO(data []byte) (string, error) {
	f, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("not a valid Mach-O file: %w", err)
	}
	defer f.Close()

	info := machoInfo{
		Type:     "Mach-O",
		OS:       "macOS",
		FileSize: len(data),
	}

	// Architecture
	switch f.Cpu {
	case macho.CpuAmd64:
		info.Arch = "x86-64"
		info.Bits = 64
	case macho.Cpu386:
		info.Arch = "x86"
		info.Bits = 32
	case macho.CpuArm64:
		info.Arch = "arm64"
		info.Bits = 64
	case macho.CpuArm:
		info.Arch = "arm"
		info.Bits = 32
	case macho.CpuPpc64:
		info.Arch = "ppc64"
		info.Bits = 64
	default:
		info.Arch = fmt.Sprintf("cpu-0x%x", uint32(f.Cpu))
	}

	// Byte order
	info.Endian = "little" // Mach-O on Apple Silicon / x86 is LE

	// Sections
	hasSymtab := false
	for _, s := range f.Sections {
		info.Sections = append(info.Sections, s.Name)
		if s.Name == "__symbol_table" || strings.Contains(s.Name, "symtab") {
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
	info.Language, info.GoVersion = detectMachOLanguage(f)

	out, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

func detectMachOLanguage(f *macho.File) (lang, goVersion string) {
	for _, s := range f.Sections {
		if s.Name == "__go_buildinfo" || s.Name == "__gosymtab" || s.Name == "__gopclntab" {
			lang = "Go"
		}
	}
	if lang == "Go" {
		// Try to read Go version from __go_buildinfo section
		for _, s := range f.Sections {
			if s.Name == "__go_buildinfo" {
				data, err := s.Data()
				if err == nil {
					ver, _ := parseGoBuildInfoBytes(data)
					goVersion = ver
				}
				break
			}
		}
		return
	}

	// Rust
	for _, s := range f.Sections {
		if strings.Contains(s.Name, "rust") {
			return "Rust", ""
		}
	}

	// C++ via imported symbols
	syms, _ := f.ImportedSymbols()
	for _, sym := range syms {
		if strings.HasPrefix(sym, "__ZN") {
			return "C++", ""
		}
	}
	libs, _ := f.ImportedLibraries()
	for _, lib := range libs {
		if strings.Contains(lib, "libc++") || strings.Contains(lib, "libstdc++") {
			return "C++", ""
		}
	}
	return "C", ""
}

// isMachO returns true if data starts with a Mach-O magic number.
func isMachO(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	magic := []byte{data[0], data[1], data[2], data[3]}
	return bytes.Equal(magic, []byte{0xfe, 0xed, 0xfa, 0xce}) || // 32-bit BE
		bytes.Equal(magic, []byte{0xce, 0xfa, 0xed, 0xfe}) || // 32-bit LE
		bytes.Equal(magic, []byte{0xfe, 0xed, 0xfa, 0xcf}) || // 64-bit BE
		bytes.Equal(magic, []byte{0xcf, 0xfa, 0xed, 0xfe}) || // 64-bit LE
		bytes.Equal(magic, []byte{0xca, 0xfe, 0xba, 0xbe})    // fat binary
}
