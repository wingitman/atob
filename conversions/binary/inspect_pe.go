package binary

import (
	"bytes"
	"debug/pe"
	"encoding/json"
	"fmt"
	"strings"
)

type peInfo struct {
	Type         string   `json:"type"`
	Arch         string   `json:"arch"`
	Subsystem    string   `json:"subsystem"`
	Bits         int      `json:"bits"`
	EntryPoint   string   `json:"entry_point"`
	Language     string   `json:"language"`
	Sections     []string `json:"sections"`
	ImportedLibs []string `json:"imported_libs"`
	Stripped     bool     `json:"stripped"`
	FileSize     int      `json:"file_size"`
}

func inspectPE(data []byte) (string, error) {
	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("not a valid PE file: %w", err)
	}
	defer f.Close()

	info := peInfo{
		Type:     "PE",
		FileSize: len(data),
	}

	// Architecture + bits
	switch f.Machine {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		info.Arch = "x86-64"
		info.Bits = 64
	case pe.IMAGE_FILE_MACHINE_I386:
		info.Arch = "x86"
		info.Bits = 32
	case pe.IMAGE_FILE_MACHINE_ARM64:
		info.Arch = "arm64"
		info.Bits = 64
	case pe.IMAGE_FILE_MACHINE_ARMNT:
		info.Arch = "arm"
		info.Bits = 32
	default:
		info.Arch = fmt.Sprintf("0x%x", uint16(f.Machine))
	}

	// Entry point + subsystem from optional header
	switch oh := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		info.EntryPoint = fmt.Sprintf("0x%x", oh.AddressOfEntryPoint)
		info.Subsystem = peSubsystemName(oh.Subsystem)
	case *pe.OptionalHeader64:
		info.EntryPoint = fmt.Sprintf("0x%x", oh.AddressOfEntryPoint)
		info.Subsystem = peSubsystemName(oh.Subsystem)
	}

	// Sections
	hasSymbol := false
	for _, s := range f.Sections {
		info.Sections = append(info.Sections, s.Name)
		if strings.Contains(s.Name, "symbol") || s.Name == ".debug" {
			hasSymbol = true
		}
	}
	info.Stripped = !hasSymbol

	// Imports
	libs, _ := f.ImportedLibraries()
	info.ImportedLibs = libs
	if info.ImportedLibs == nil {
		info.ImportedLibs = []string{}
	}

	// Language detection
	info.Language = detectPELanguage(f, libs)

	out, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

func detectPELanguage(f *pe.File, libs []string) string {
	// Go: has .symtab or specific section names
	for _, s := range f.Sections {
		if s.Name == ".note.go.buildid" || s.Name == ".gosymtab" || s.Name == ".gopclntab" {
			return "Go"
		}
	}
	// Check for Go runtime symbols in imports or section data
	for _, lib := range libs {
		if strings.EqualFold(lib, "msvcrt.dll") {
			// Could be C, C++, or Rust
		}
	}
	syms, _ := f.ImportedSymbols()
	for _, sym := range syms {
		if strings.HasPrefix(sym, "_ZN") {
			return "C++"
		}
	}
	for _, lib := range libs {
		lower := strings.ToLower(lib)
		if strings.Contains(lower, "vcruntime") || strings.Contains(lower, "msvcp") {
			return "C++"
		}
	}
	return "C"
}

func peSubsystemName(s uint16) string {
	switch s {
	case 1:
		return "native"
	case 2:
		return "windows-gui"
	case 3:
		return "windows-cui"
	case 7:
		return "posix-cui"
	case 9:
		return "windows-ce-gui"
	case 10:
		return "efi-application"
	default:
		return fmt.Sprintf("0x%x", s)
	}
}
