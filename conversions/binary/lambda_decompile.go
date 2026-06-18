package binary

import (
	"archive/zip"
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.RegisterBinary(lambdaDecompile{})
}

type lambdaDecompile struct{}

func (lambdaDecompile) Name() string        { return "decompile" }
func (lambdaDecompile) Category() string    { return "binary" }
func (lambdaDecompile) Description() string { return "Decompile and unpack AWS Lambda zip deployment packages, Java .class, Python .pyc, or .NET .dll files" }

func (lambdaDecompile) ConvertBytes(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty input data")
	}

	// Route based on file magic bytes
	switch {
	case isZIP(data):
		return decompileLambdaZip(data)
	case len(data) >= 4 && binary.BigEndian.Uint32(data[:4]) == 0xCAFEBABE:
		return decompileJavaClassDirect(data)
	case isPyc(data):
		return decompilePythonPycDirect(data)
	case isPE(data):
		if isDotNetAssembly(data) {
			return decompileDotNetAssemblyDirect(data)
		}
		return `# Native Binary Decompilation Error

The provided PE binary does not appear to be a .NET Assembly.

Atob's decompiler currently supports:
1. **AWS Lambda ZIP Packages**
2. **Java .class files**
3. **Python .pyc bytecode files**
4. **.NET CLI DLL / EXE files** (written in C# or F#)

For native C/C++ or Go/Rust executables, use atob's ` + "`inspect`" + ` target to view metadata or ` + "`strings`" + ` to extract static strings.
`, nil
	default:
		return `# Decompilation Error

The provided data does not appear to be a supported format.

Atob's decompiler currently supports:
1. **AWS Lambda ZIP Packages** (containing Node.js, Python, Java, .NET, or Go/Custom runtimes)
2. **Compiled Java .class files** (magic ` + "`0xCAFEBABE`" + `)
3. **Compiled Python .pyc bytecode files**
4. **Compiled .NET assembly .dll / .exe files**
`, nil
	}
}

// ── ZIP Package Decompiler ───────────────────────────────────────────────────

func decompileLambdaZip(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to parse ZIP file: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# AWS Lambda Decompilation Report\n\n")

	// 1. Gather all files and build directory metadata
	var files []string
	var fileSizes = make(map[string]int64)
	for _, f := range r.File {
		files = append(files, f.Name)
		fileSizes[f.Name] = int64(f.UncompressedSize64)
	}
	sort.Strings(files)

	// 2. Auto-detect runtime and entry point
	runtime, confidence, mainFile := detectLambdaRuntime(files)
	sb.WriteString("## Environment & Metadata\n")
	sb.WriteString(fmt.Sprintf("- **Detected Runtime:** %s (Confidence: %s)\n", runtime, confidence))
	if mainFile != "" {
		sb.WriteString(fmt.Sprintf("- **Primary Handler/Entry File:** `%s`\n", mainFile))
	} else {
		sb.WriteString("- **Primary Handler/Entry File:** *Could not be determined automatically*\n")
	}
	sb.WriteString(fmt.Sprintf("- **Total Archive Size:** %d bytes\n", len(data)))
	sb.WriteString(fmt.Sprintf("- **Total Files in Archive:** %d\n\n", len(files)))

	// 3. Render ASCII directory tree
	sb.WriteString("## Archive Structure\n")
	sb.WriteString("```\n")
	sb.WriteString(buildASCIIFileTree(files))
	sb.WriteString("```\n\n")

	// 4. Extract and decompile/format primary files
	sb.WriteString("## Decompiled & Formatted Sources\n\n")
	decompiledCount := 0

	// Gather key candidate files to show (the main handler first, then other code files)
	var candidates []string
	if mainFile != "" {
		candidates = append(candidates, mainFile)
	}
	for _, file := range files {
		if file == mainFile || isDir(file) {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file))
		// Show up to 3 files total to avoid overwhelming the output
		if decompiledCount < 3 && (ext == ".js" || ext == ".mjs" || ext == ".py" || ext == ".pyc" || ext == ".class" || ext == ".ts" || ext == ".dll") {
			// Skip node_modules or standard dependencies to focus on user code
			if !strings.Contains(file, "node_modules/") && !strings.Contains(file, "venv/") && !strings.Contains(file, "site-packages/") {
				candidates = append(candidates, file)
			}
		}
	}

	for _, file := range candidates {
		if decompiledCount >= 3 {
			break
		}
		// Find zip file
		var targetFile *zip.File
		for _, f := range r.File {
			if f.Name == file {
				targetFile = f
				break
			}
		}
		if targetFile == nil {
			continue
		}

		// Read content
		fileData, err := readZipFileContent(targetFile)
		if err != nil {
			sb.WriteString(fmt.Sprintf("### File: `%s`\n*Error reading file content: %s*\n\n", file, err))
			continue
		}

		ext := strings.ToLower(filepath.Ext(file))
		sb.WriteString(fmt.Sprintf("### File: `%s` (%d bytes)\n", file, len(fileData)))

		switch ext {
		case ".js", ".mjs", ".ts":
			beautified := beautifyJS(string(fileData))
			sb.WriteString("```javascript\n")
			sb.WriteString(beautified)
			sb.WriteString("\n```\n\n")
			decompiledCount++
		case ".py":
			sb.WriteString("```python\n")
			sb.WriteString(strings.TrimSpace(string(fileData)))
			sb.WriteString("\n```\n\n")
			decompiledCount++
		case ".pyc":
			decompiled, err := decompilePythonPyc(fileData)
			if err != nil {
				sb.WriteString(fmt.Sprintf("*Failed to disassemble Python bytecode: %s*\n\n", err))
			} else {
				sb.WriteString(decompiled)
				sb.WriteString("\n")
			}
			decompiledCount++
		case ".class":
			decompiled, err := decompileJavaClass(fileData)
			if err != nil {
				sb.WriteString(fmt.Sprintf("*Failed to parse Java class file: %s*\n\n", err))
			} else {
				sb.WriteString("```java\n")
				sb.WriteString(decompiled)
				sb.WriteString("\n```\n\n")
			}
			decompiledCount++
		case ".dll":
			if isDotNetAssembly(fileData) {
				decompiled, err := decompileDotNetAssembly(fileData)
				if err != nil {
					sb.WriteString(fmt.Sprintf("*Failed to parse .NET Assembly: %s*\n\n", err))
				} else {
					sb.WriteString("```csharp\n")
					sb.WriteString(decompiled)
					sb.WriteString("\n```\n\n")
					decompiledCount++
				}
			} else {
				sb.WriteString("*Skipped native non-.NET DLL library*\n\n")
			}
		}
	}

	if decompiledCount == 0 {
		sb.WriteString("*No decompilable source files (such as .js, .py, .pyc, .class, or .dll) were extracted from the root of the archive.*\n")
	}

	return sb.String(), nil
}

func readZipFileContent(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// Limit to 50KB to prevent giant outputs or memory exhaustion
	limitReader := io.LimitReader(rc, 50*1024)
	return io.ReadAll(limitReader)
}

func isDir(path string) bool {
	return strings.HasSuffix(path, "/")
}

// detectLambdaRuntime analyzes files in the archive to detect language and handler
func detectLambdaRuntime(files []string) (runtime string, confidence string, handlerFile string) {
	var scores = map[string]int{
		"Node.js (JavaScript)": 0,
		"Python":               0,
		"Java":                 0,
		".NET (C#)":            0,
		"Go / Custom":          0,
	}

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		base := filepath.Base(file)

		switch ext {
		case ".js", ".mjs":
			if !strings.Contains(file, "node_modules/") {
				scores["Node.js (JavaScript)"] += 5
				if base == "index.js" || base == "index.mjs" || base == "handler.js" {
					scores["Node.js (JavaScript)"] += 20
					if handlerFile == "" {
						handlerFile = file
					}
				}
			}
		case ".json":
			if base == "package.json" {
				scores["Node.js (JavaScript)"] += 15
			}
		case ".py":
			if !strings.Contains(file, "venv/") && !strings.Contains(file, "site-packages/") {
				scores["Python"] += 5
				if base == "lambda_function.py" || base == "index.py" || base == "main.py" || base == "handler.py" {
					scores["Python"] += 20
					if handlerFile == "" {
						handlerFile = file
					}
				}
			}
		case ".pyc":
			scores["Python"] += 2
		case ".txt":
			if base == "requirements.txt" {
				scores["Python"] += 12
			}
		case ".class":
			scores["Java"] += 5
			if strings.Contains(strings.ToLower(file), "handler") {
				scores["Java"] += 10
				if handlerFile == "" {
					handlerFile = file
				}
			}
		case ".jar":
			scores["Java"] += 15
		case ".dll":
			scores[".NET (C#)"] += 10
			if base == "bootstrap.dll" {
				scores[".NET (C#)"] += 20
			}
		case ".deps.json":
			scores[".NET (C#)"] += 15
		}

		if base == "bootstrap" {
			scores["Go / Custom"] += 25
			if handlerFile == "" {
				handlerFile = file
			}
		}
	}

	bestRuntime := "Unknown"
	highestScore := 0
	for rt, score := range scores {
		if score > highestScore {
			highestScore = score
			bestRuntime = rt
		}
	}

	if highestScore >= 30 {
		confidence = "High"
	} else if highestScore >= 10 {
		confidence = "Medium"
	} else if highestScore > 0 {
		confidence = "Low"
	} else {
		confidence = "None"
	}

	// Fallback logic to find ANY main file if none set
	if handlerFile == "" {
		for _, file := range files {
			base := filepath.Base(file)
			if bestRuntime == "Node.js (JavaScript)" && (strings.HasSuffix(base, ".js") || strings.HasSuffix(base, ".mjs")) {
				handlerFile = file
				break
			}
			if bestRuntime == "Python" && strings.HasSuffix(base, ".py") {
				handlerFile = file
				break
			}
			if bestRuntime == "Java" && strings.HasSuffix(base, ".class") {
				handlerFile = file
				break
			}
		}
	}

	return bestRuntime, confidence, handlerFile
}

// buildASCIIFileTree constructs a visual directory tree representation
func buildASCIIFileTree(files []string) string {
	type node struct {
		name     string
		children map[string]*node
	}

	root := &node{name: "root", children: make(map[string]*node)}

	// Build the tree hierarchy
	for _, file := range files {
		parts := strings.Split(strings.TrimSuffix(file, "/"), "/")
		curr := root
		for _, part := range parts {
			if part == "" {
				continue
			}
			if curr.children[part] == nil {
				curr.children[part] = &node{name: part, children: make(map[string]*node)}
			}
			curr = curr.children[part]
		}
	}

	var sb strings.Builder
	sb.WriteString(".\n")

	var render func(n *node, prefix string)
	render = func(n *node, prefix string) {
		keys := make([]string, 0, len(n.children))
		for k := range n.children {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// To prevent rendering endless file listings of e.g. node_modules, truncate directories if too large
		nodeModulesOmitted := false
		count := 0
		for i, k := range keys {
			if count > 20 {
				sb.WriteString(fmt.Sprintf("%s└── ... (%d more files omitted)\n", prefix, len(keys)-count))
				break
			}
			// Special-case node_modules / venv to keep tree compact
			if k == "node_modules" || k == "venv" || k == "site-packages" || k == ".git" {
				nodeModulesOmitted = true
			}

			isLast := i == len(keys)-1
			connector := "├── "
			nextPrefix := prefix + "│   "
			if isLast {
				connector = "└── "
				nextPrefix = prefix + "    "
			}

			sb.WriteString(prefix + connector + k)
			child := n.children[k]
			if len(child.children) > 0 {
				if nodeModulesOmitted && (k == "node_modules" || k == "venv" || k == "site-packages" || k == ".git") {
					sb.WriteString("/ (dependencies list collapsed)\n")
				} else {
					sb.WriteString("/\n")
					render(child, nextPrefix)
				}
			} else {
				sb.WriteString("\n")
			}
			count++
		}
	}

	render(root, "")
	return sb.String()
}

// ── Smart JS Beautifier ──────────────────────────────────────────────────────

// beautifyJS performs basic indentation and structure formatting for minified JS
func beautifyJS(code string) string {
	var out strings.Builder
	indent := 0
	inString := false
	var quoteChar rune
	escaped := false
	inLineComment := false
	inBlockComment := false

	// Helper to write indentation
	writeIndent := func() {
		for i := 0; i < indent; i++ {
			out.WriteString("  ")
		}
	}

	trimmed := strings.TrimSpace(code)
	runes := []rune(trimmed)
	length := len(runes)

	for i := 0; i < length; i++ {
		r := runes[i]

		// Handle escaping in strings
		if inString {
			if escaped {
				out.WriteRune(r)
				escaped = false
				continue
			}
			if r == '\\' {
				out.WriteRune(r)
				escaped = true
				continue
			}
			if r == quoteChar {
				out.WriteRune(r)
				inString = false
				continue
			}
			out.WriteRune(r)
			continue
		}

		// Handle comments
		if inLineComment {
			if r == '\n' {
				out.WriteRune(r)
				inLineComment = false
				writeIndent()
			} else {
				out.WriteRune(r)
			}
			continue
		}

		if inBlockComment {
			if r == '/' && i > 0 && runes[i-1] == '*' {
				out.WriteRune(r)
				inBlockComment = false
			} else {
				out.WriteRune(r)
			}
			continue
		}

		// Lookahead for comments
		if r == '/' && i+1 < length {
			if runes[i+1] == '/' {
				out.WriteString("//")
				inLineComment = true
				i++
				continue
			} else if runes[i+1] == '*' {
				out.WriteString("/*")
				inBlockComment = true
				i++
				continue
			}
		}

		// Handle entering strings
		if r == '"' || r == '\'' || r == '`' {
			inString = true
			quoteChar = r
			out.WriteRune(r)
			continue
		}

		// Formatting logic
		switch r {
		case '{':
			out.WriteString(" {")
			indent++
			out.WriteRune('\n')
			writeIndent()
		case '}':
			indent--
			if indent < 0 {
				indent = 0
			}
			// Back up to strip preceding whitespace if last character was indent spaces
			outStr := out.String()
			if strings.HasSuffix(outStr, "  ") {
				// Crude trim of spacing before the closing brace
				trimmed := strings.TrimRight(outStr, " ")
				out.Reset()
				out.WriteString(trimmed)
			} else {
				out.WriteRune('\n')
			}
			writeIndent()
			out.WriteRune('}')
			out.WriteRune('\n')
			writeIndent()
		case ';':
			out.WriteRune(';')
			out.WriteRune('\n')
			// Skip whitespace following a semicolon to avoid formatting errors
			for i+1 < length && (runes[i+1] == ' ' || runes[i+1] == '\t' || runes[i+1] == '\r' || runes[i+1] == '\n') {
				i++
			}
			writeIndent()
		case ',':
			out.WriteString(", ")
		case ':':
			out.WriteString(": ")
		case '\n', '\r':
			// Collapse multiple blank lines
			outStr := out.String()
			if !strings.HasSuffix(outStr, "\n") && !strings.HasSuffix(outStr, " ") {
				out.WriteRune('\n')
				writeIndent()
			}
		case ' ', '\t':
			// Avoid duplicate spacing
			outStr := out.String()
			if len(outStr) > 0 && outStr[len(outStr)-1] != ' ' && !strings.HasSuffix(outStr, "\n") {
				out.WriteRune(' ')
			}
		default:
			out.WriteRune(r)
		}
	}

	// Post-process to clean up excess empty lines
	lines := strings.Split(out.String(), "\n")
	var cleaned []string
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" && len(cleaned) > 0 && cleaned[len(cleaned)-1] == "" {
			continue // skip duplicate empty lines
		}
		cleaned = append(cleaned, line)
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// ── JVM .class Parser ────────────────────────────────────────────────────────

func decompileJavaClassDirect(data []byte) (string, error) {
	decompiled, err := decompileJavaClass(data)
	if err != nil {
		return "", err
	}
	return "# Java Class Decompilation\n\n```java\n" + decompiled + "\n```\n", nil
}

func decompileJavaClass(data []byte) (string, error) {
	if len(data) < 24 {
		return "", fmt.Errorf("class file too short")
	}

	reader := bytes.NewReader(data)

	// Magic
	var magic uint32
	if err := binary.Read(reader, binary.BigEndian, &magic); err != nil {
		return "", err
	}
	if magic != 0xCAFEBABE {
		return "", fmt.Errorf("invalid class magic: 0x%08X", magic)
	}

	// Minor/Major version
	var minor, major uint16
	binary.Read(reader, binary.BigEndian, &minor)
	binary.Read(reader, binary.BigEndian, &major)

	// Constant Pool
	var cpCount uint16
	binary.Read(reader, binary.BigEndian, &cpCount)

	// JVM Spec: constant pool indices are 1-based and some entries (Long, Double) take 2 slots
	type cpEntry interface{}
	cp := make([]cpEntry, cpCount)

	for i := int(1); i < int(cpCount); i++ {
		var tag uint8
		if err := binary.Read(reader, binary.BigEndian, &tag); err != nil {
			return "", fmt.Errorf("failed to read constant pool tag at %d: %w", i, err)
		}

		switch tag {
		case 1: // UTF8
			var length uint16
			binary.Read(reader, binary.BigEndian, &length)
			utfBytes := make([]byte, length)
			binary.Read(reader, binary.BigEndian, &utfBytes)
			cp[i] = string(utfBytes)
		case 7: // Class
			var nameIdx uint16
			binary.Read(reader, binary.BigEndian, &nameIdx)
			cp[i] = nameIdx // Store reference to index
		case 8: // String
			var strIdx uint16
			binary.Read(reader, binary.BigEndian, &strIdx)
			cp[i] = strIdx
		case 3, 4, 9, 10, 11, 12, 18: // Fieldref, Methodref, InterfaceMethodref, NameAndType, Integer, Float, InvokeDynamic
			var dummy1, dummy2 uint16
			binary.Read(reader, binary.BigEndian, &dummy1)
			binary.Read(reader, binary.BigEndian, &dummy2)
			if tag == 12 {
				cp[i] = [2]uint16{dummy1, dummy2} // NameAndType
			}
		case 5, 6: // Long, Double
			var dummy1, dummy2 uint32
			binary.Read(reader, binary.BigEndian, &dummy1)
			binary.Read(reader, binary.BigEndian, &dummy2)
			cp[i] = "long_or_double"
			i++ // Takes two slots
		case 15: // MethodHandle
			var kind uint8
			var idx uint16
			binary.Read(reader, binary.BigEndian, &kind)
			binary.Read(reader, binary.BigEndian, &idx)
		case 16: // MethodType
			var idx uint16
			binary.Read(reader, binary.BigEndian, &idx)
		default:
			return "", fmt.Errorf("unknown constant pool tag %d at index %d", tag, i)
		}
	}

	// Read class definition info
	var accessFlags, thisClassIdx, superClassIdx, interfacesCount uint16
	binary.Read(reader, binary.BigEndian, &accessFlags)
	binary.Read(reader, binary.BigEndian, &thisClassIdx)
	binary.Read(reader, binary.BigEndian, &superClassIdx)
	binary.Read(reader, binary.BigEndian, &interfacesCount)

	// Resolve Class Names
	resolveClassName := func(idx uint16) string {
		if idx == 0 || int(idx) >= len(cp) {
			return ""
		}
		nameIdxVal, ok := cp[idx].(uint16)
		if !ok {
			return ""
		}
		if nameIdxVal == 0 || int(nameIdxVal) >= len(cp) {
			return ""
		}
		strVal, ok := cp[nameIdxVal].(string)
		if !ok {
			return ""
		}
		return strings.ReplaceAll(strVal, "/", ".")
	}

	thisClassName := resolveClassName(thisClassIdx)
	if thisClassName == "" {
		thisClassName = "UnknownClass"
	}
	superClassName := resolveClassName(superClassIdx)

	// Skip interfaces in reader
	interfaces := make([]uint16, interfacesCount)
	for i := 0; i < int(interfacesCount); i++ {
		binary.Read(reader, binary.BigEndian, &interfaces[i])
	}

	// Access Flags helpers
	parseAccessFlags := func(flags uint16, isMethod bool) string {
		var access []string
		if flags&0x0001 != 0 {
			access = append(access, "public")
		}
		if flags&0x0002 != 0 {
			access = append(access, "private")
		}
		if flags&0x0004 != 0 {
			access = append(access, "protected")
		}
		if flags&0x0008 != 0 {
			access = append(access, "static")
		}
		if flags&0x0010 != 0 {
			access = append(access, "final")
		}
		if isMethod {
			if flags&0x0020 != 0 {
				access = append(access, "synchronized")
			}
			if flags&0x0100 != 0 {
				access = append(access, "native")
			}
			if flags&0x0400 != 0 {
				access = append(access, "abstract")
			}
		} else {
			if flags&0x0040 != 0 {
				access = append(access, "volatile")
			}
			if flags&0x0080 != 0 {
				access = append(access, "transient")
			}
		}
		return strings.Join(access, " ")
	}

	var sb strings.Builder

	// Write Class Declaration Header
	javaVer := fmt.Sprintf("// Java Class File version: %d.%d (Java %d)\n", major, minor, major-44)
	sb.WriteString(javaVer)

	pkgParts := strings.Split(thisClassName, ".")
	if len(pkgParts) > 1 {
		sb.WriteString(fmt.Sprintf("package %s;\n\n", strings.Join(pkgParts[:len(pkgParts)-1], ".")))
	}

	classAccess := parseAccessFlags(accessFlags, false)
	classNameOnly := pkgParts[len(pkgParts)-1]

	sb.WriteString(classAccess)
	if !strings.Contains(classAccess, "class") && !strings.Contains(classAccess, "interface") {
		sb.WriteString(" class ")
	} else {
		sb.WriteString(" ")
	}
	sb.WriteString(classNameOnly)

	if superClassName != "" && superClassName != "java.lang.Object" {
		sb.WriteString(" extends " + superClassName)
	}

	if interfacesCount > 0 {
		var ifList []string
		for _, ifIdx := range interfaces {
			ifList = append(ifList, resolveClassName(ifIdx))
		}
		sb.WriteString(" implements " + strings.Join(ifList, ", "))
	}

	sb.WriteString(" {\n")

	// Read Fields
	var fieldsCount uint16
	binary.Read(reader, binary.BigEndian, &fieldsCount)

	for i := 0; i < int(fieldsCount); i++ {
		var fAccess, nameIdx, descIdx, attrsCount uint16
		binary.Read(reader, binary.BigEndian, &fAccess)
		binary.Read(reader, binary.BigEndian, &nameIdx)
		binary.Read(reader, binary.BigEndian, &descIdx)
		binary.Read(reader, binary.BigEndian, &attrsCount)

		// Resolve Field Name and Type
		var fName, fType string
		if nameIdx > 0 && int(nameIdx) < len(cp) {
			if s, ok := cp[nameIdx].(string); ok {
				fName = s
			}
		}
		if descIdx > 0 && int(descIdx) < len(cp) {
			if s, ok := cp[descIdx].(string); ok {
				fType = parseJavaDescriptor(s)
			}
		}

		// Skip Attributes
		for a := 0; a < int(attrsCount); a++ {
			var attrNameIdx uint16
			var attrLen uint32
			binary.Read(reader, binary.BigEndian, &attrNameIdx)
			binary.Read(reader, binary.BigEndian, &attrLen)
			reader.Seek(int64(attrLen), io.SeekCurrent)
		}

		fieldFlags := parseAccessFlags(fAccess, false)
		if fieldFlags != "" {
			sb.WriteString(fmt.Sprintf("    %s %s %s;\n", fieldFlags, fType, fName))
		} else {
			sb.WriteString(fmt.Sprintf("    %s %s;\n", fType, fName))
		}
	}

	if fieldsCount > 0 {
		sb.WriteString("\n")
	}

	// Read Methods
	var methodsCount uint16
	binary.Read(reader, binary.BigEndian, &methodsCount)

	for i := 0; i < int(methodsCount); i++ {
		var mAccess, nameIdx, descIdx, attrsCount uint16
		binary.Read(reader, binary.BigEndian, &mAccess)
		binary.Read(reader, binary.BigEndian, &nameIdx)
		binary.Read(reader, binary.BigEndian, &descIdx)
		binary.Read(reader, binary.BigEndian, &attrsCount)

		var mName, mDesc string
		if nameIdx > 0 && int(nameIdx) < len(cp) {
			if s, ok := cp[nameIdx].(string); ok {
				mName = s
			}
		}
		if descIdx > 0 && int(descIdx) < len(cp) {
			if s, ok := cp[descIdx].(string); ok {
				mDesc = s
			}
		}

		// Parse JVM Method Parameter & Return types
		mArgs, mRet := parseJavaMethodDescriptor(mDesc)

		// Skip Attributes in reader
		for a := 0; a < int(attrsCount); a++ {
			var attrNameIdx uint16
			var attrLen uint32
			binary.Read(reader, binary.BigEndian, &attrNameIdx)
			binary.Read(reader, binary.BigEndian, &attrLen)
			reader.Seek(int64(attrLen), io.SeekCurrent)
		}

		methodFlags := parseAccessFlags(mAccess, true)

		// Handle Java constructor name
		if mName == "<init>" {
			mName = classNameOnly
			mRet = ""
		} else if mName == "<clinit>" {
			sb.WriteString("    static {\n        // Static initializer block\n    }\n\n")
			continue
		}

		signature := ""
		if methodFlags != "" {
			signature = methodFlags + " "
		}
		if mRet != "" {
			signature += mRet + " "
		}
		signature += fmt.Sprintf("%s(%s)", mName, strings.Join(mArgs, ", "))

		sb.WriteString(fmt.Sprintf("    %s {\n        // Decompiled JVM bytecode omitted\n    }\n\n", signature))
	}

	sb.WriteString("}\n")
	return sb.String(), nil
}

// parseJavaDescriptor converts internal JVM types to readable Java types
func parseJavaDescriptor(d string) string {
	if len(d) == 0 {
		return ""
	}
	switch d[0] {
	case 'B':
		return "byte"
	case 'C':
		return "char"
	case 'D':
		return "double"
	case 'F':
		return "float"
	case 'I':
		return "int"
	case 'J':
		return "long"
	case 'S':
		return "short"
	case 'Z':
		return "boolean"
	case 'V':
		return "void"
	case 'L':
		// Object types: Ljava/lang/String; -> java.lang.String
		end := strings.Index(d, ";")
		if end == -1 {
			return d[1:]
		}
		rawObj := d[1:end]
		return strings.ReplaceAll(rawObj, "/", ".")
	case '[':
		// Array types
		return parseJavaDescriptor(d[1:]) + "[]"
	}
	return d
}

// parseJavaMethodDescriptor parses JVM method arguments and return types
func parseJavaMethodDescriptor(d string) (args []string, ret string) {
	if !strings.HasPrefix(d, "(") {
		return nil, parseJavaDescriptor(d)
	}

	endArgs := strings.Index(d, ")")
	if endArgs == -1 {
		return nil, d
	}

	argStr := d[1:endArgs]
	retStr := d[endArgs+1:]

	// Decode args one by one
	for len(argStr) > 0 {
		i := 0
		for argStr[i] == '[' {
			i++
		}
		if argStr[i] == 'L' {
			semi := strings.Index(argStr[i:], ";")
			if semi != -1 {
				args = append(args, parseJavaDescriptor(argStr[:i+semi+1]))
				argStr = argStr[i+semi+1:]
			} else {
				args = append(args, parseJavaDescriptor(argStr))
				break
			}
		} else {
			args = append(args, parseJavaDescriptor(argStr[:i+1]))
			argStr = argStr[i+1:]
		}
	}

	ret = parseJavaDescriptor(retStr)
	return args, ret
}

// ── Python .pyc Bytecode Reader & Disassembler ────────────────────────────────

func isPyc(data []byte) bool {
	// A rough check: Pyc starts with a magic number that represents Python compile version.
	// Historically, this magic number is 4 bytes. We'll verify length and check common Python 3 ranges.
	if len(data) < 16 {
		return false
	}
	// Common Python 3.x magic numbers: Python 3.6 to 3.12 use ranges 0x0A0D0D03 - 0x0A0D0DFF, 
	// or in little-endian format they typically end in 0x0D 0x0D (or 0x03 0xF3 / 0x0D 0x0A).
	// Let's check typical patterns.
	magic := binary.LittleEndian.Uint32(data[:4])
	// Most Python 3 .pyc files end with \r\n (0x0d, 0x0a) as the top 2 bytes
	return (magic & 0xFFFF0000) == 0x0A0D0000 || (magic & 0x0000FFFF) == 0x0D0A || (data[2] == 0x0D && data[3] == 0x0A)
}

func decompilePythonPycDirect(data []byte) (string, error) {
	decompiled, err := decompilePythonPyc(data)
	if err != nil {
		return "", err
	}
	return decompiled, nil
}

func decompilePythonPyc(data []byte) (string, error) {
	if len(data) < 16 {
		return "", fmt.Errorf("compiled python file too short")
	}

	magic := binary.LittleEndian.Uint32(data[:4])
	pyVersion := lookupPythonVersion(magic)

	// Since Python 3.7, the header is 16 bytes.
	// Python 3.3-3.6 use 12 bytes.
	headerSize := 16
	if magic < 3394 { // Magic numbers below ~3394 are Python 3.6 or older
		headerSize = 12
	}

	if len(data) <= headerSize {
		return "", fmt.Errorf("pyc contains no bytecode payload")
	}

	marshalData := data[headerSize:]

	var sb strings.Builder
	sb.WriteString("# Python Bytecode Disassembly\n")
	sb.WriteString(fmt.Sprintf("- **Inferred Python Version:** %s\n", pyVersion))
	sb.WriteString(fmt.Sprintf("- **Magic Number:** 0x%08X (%d)\n\n", magic, magic&0xFFFF))

	// Simple Python Marshal Parser
	reader := bytes.NewReader(marshalData)
	obj, err := parsePythonMarshal(reader)
	if err != nil {
		return "", fmt.Errorf("failed to marshal decode Python payload: %w", err)
	}

	sb.WriteString("## Decoded Code Constants & Names\n")
	renderCodeObjectStructure(&sb, obj, 0)

	return sb.String(), nil
}

func lookupPythonVersion(magic uint32) string {
	magicVal := magic & 0xFFFF
	switch magicVal {
	case 3394:
		return "Python 3.6"
	case 3410, 3411, 3413:
		return "Python 3.7"
	case 3414:
		return "Python 3.8"
	case 3420, 3421, 3422, 3423, 3424, 3425:
		return "Python 3.9"
	case 3430, 3431, 3432, 3433, 3434, 3435, 3436, 3437, 3438, 3439:
		return "Python 3.10"
	case 3495, 3515:
		return "Python 3.11"
	case 3531:
		return "Python 3.12"
	case 3550:
		return "Python 3.13"
	default:
		// Attempt historical/rough classification
		if magicVal >= 3500 {
			return "Python 3.11+"
		} else if magicVal >= 3400 {
			return "Python 3.7 - 3.10"
		} else if magicVal >= 3000 {
			return "Python 3.x"
		}
		return "Python 2.x or Older"
	}
}

type pyObject interface{}

type pyCodeObject struct {
	argCount       int
	kwOnlyArgCount int
	nLocals        int
	stackSize      int
	flags          int
	code           []byte
	consts         []pyObject
	names          []string
	varNames       []string
	freeVars       []string
	cellVars       []string
	fileName       string
	name           string
	firstLineNo    int
}

// parsePythonMarshal implements a subset of python marshal serialization format
func parsePythonMarshal(r *bytes.Reader) (pyObject, error) {
	tag, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	// Flag for interned strings is represented by top-bit (0x80)
	tagType := tag & 0x7F

	switch tagType {
	case '0', 'N': // None
		return nil, nil
	case 'T': // True
		return true, nil
	case 'F': // False
		return false, nil
	case 'i': // Int32
		var val int32
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return int(val), nil
	case 'g': // Binary float
		var val float64
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return val, nil
	case 's': // String / Bytes
		var size int32
		binary.Read(r, binary.LittleEndian, &size)
		buf := make([]byte, size)
		r.Read(buf)
		return string(buf), nil
	case 'u', 't', 'z': // Unicode, ASCII, Short ASCII string
		var size int32
		if tagType == 'z' {
			var shortSize uint8
			binary.Read(r, binary.LittleEndian, &shortSize)
			size = int32(shortSize)
		} else {
			binary.Read(r, binary.LittleEndian, &size)
		}
		buf := make([]byte, size)
		r.Read(buf)
		return string(buf), nil
	case '(', '[': // Tuple, List
		var size int32
		binary.Read(r, binary.LittleEndian, &size)
		items := make([]pyObject, size)
		for i := 0; i < int(size); i++ {
			it, err := parsePythonMarshal(r)
			if err != nil {
				return nil, err
			}
			items[i] = it
		}
		return items, nil
	case 'c': // Code Object!
		code := &pyCodeObject{}
		var argCount, kwOnlyArgCount, posOnlyArgCount, nLocals, stackSize, flags, firstLineNo int32

		binary.Read(r, binary.LittleEndian, &argCount)
		// Python 3.8+ introduced position-only arguments
		binary.Read(r, binary.LittleEndian, &posOnlyArgCount)
		binary.Read(r, binary.LittleEndian, &kwOnlyArgCount)
		binary.Read(r, binary.LittleEndian, &nLocals)
		binary.Read(r, binary.LittleEndian, &stackSize)
		binary.Read(r, binary.LittleEndian, &flags)

		code.argCount = int(argCount)
		code.kwOnlyArgCount = int(kwOnlyArgCount)
		code.nLocals = int(nLocals)
		code.stackSize = int(stackSize)
		code.flags = int(flags)

		// Code payload
		rawCodeObj, err := parsePythonMarshal(r)
		if err != nil {
			return nil, err
		}
		if b, ok := rawCodeObj.(string); ok {
			code.code = []byte(b)
		} else if b, ok := rawCodeObj.([]byte); ok {
			code.code = b
		}

		// Consts
		rawConsts, err := parsePythonMarshal(r)
		if err != nil {
			return nil, err
		}
		if list, ok := rawConsts.([]pyObject); ok {
			code.consts = list
		}

		// Names
		rawNames, err := parsePythonMarshal(r)
		if err != nil {
			return nil, err
		}
		code.names = resolveStringList(rawNames)

		// Varnames
		rawVarnames, err := parsePythonMarshal(r)
		if err != nil {
			return nil, err
		}
		code.varNames = resolveStringList(rawVarnames)

		// Freevars
		rawFreevars, err := parsePythonMarshal(r)
		if err != nil {
			return nil, err
		}
		code.freeVars = resolveStringList(rawFreevars)

		// Cellvars
		rawCellvars, err := parsePythonMarshal(r)
		if err != nil {
			return nil, err
		}
		code.cellVars = resolveStringList(rawCellvars)

		// Filename, Name
		rawFilename, _ := parsePythonMarshal(r)
		if s, ok := rawFilename.(string); ok {
			code.fileName = s
		}
		rawName, _ := parsePythonMarshal(r)
		if s, ok := rawName.(string); ok {
			code.name = s
		}

		binary.Read(r, binary.LittleEndian, &firstLineNo)
		code.firstLineNo = int(firstLineNo)

		// Skip remaining code attributes (lnotab, etc.)
		parsePythonMarshal(r)

		return code, nil
	default:
		return nil, fmt.Errorf("unsupported marshal tag '%c' (0x%x)", tagType, tagType)
	}
}

func resolveStringList(obj pyObject) []string {
	var list []string
	if items, ok := obj.([]pyObject); ok {
		for _, item := range items {
			if s, ok := item.(string); ok {
				list = append(list, s)
			}
		}
	}
	return list
}

func renderCodeObjectStructure(sb *strings.Builder, obj pyObject, depth int) {
	code, ok := obj.(*pyCodeObject)
	if !ok {
		return
	}

	indent := strings.Repeat("  ", depth)
	sb.WriteString(fmt.Sprintf("%s### Code Block: `%s` (file: `%s`, line: %d)\n", indent, code.name, filepath.Base(code.fileName), code.firstLineNo))
	sb.WriteString(fmt.Sprintf("%s- **Arguments:** %d args, %d keyword-only args\n", indent, code.argCount, code.kwOnlyArgCount))
	sb.WriteString(fmt.Sprintf("%s- **Local Variables:** %s\n", indent, formatStringSlice(code.varNames)))
	sb.WriteString(fmt.Sprintf("%s- **Global/Attribute Names Referenced:** %s\n", indent, formatStringSlice(code.names)))

	// Constants listing
	sb.WriteString(fmt.Sprintf("%s- **Constants (`co_consts`):**\n", indent))
	for i, c := range code.consts {
		if nestedCode, ok := c.(*pyCodeObject); ok {
			sb.WriteString(fmt.Sprintf("%s  - [%d] *Nested Code Object:* `%s`\n", indent, i, nestedCode.name))
		} else {
			sb.WriteString(fmt.Sprintf("%s  - [%d] `%v`\n", indent, i, c))
		}
	}

	// Bytecode Disassembly
	sb.WriteString(fmt.Sprintf("\n%s```python\n", indent))
	sb.WriteString(fmt.Sprintf("%s# Disassembly of '%s':\n", indent, code.name))
	sb.WriteString(disassemblePythonBytes(code.code, code.consts, code.names, code.varNames, indent))
	sb.WriteString(fmt.Sprintf("%s```\n\n", indent))

	// Recurse into nested code constants
	for _, c := range code.consts {
		if _, isNested := c.(*pyCodeObject); isNested {
			renderCodeObjectStructure(sb, c, depth+1)
		}
	}
}

func formatStringSlice(slice []string) string {
	if len(slice) == 0 {
		return "*(none)*"
	}
	var quoted []string
	for _, s := range slice {
		quoted = append(quoted, "`"+s+"`")
	}
	return strings.Join(quoted, ", ")
}

// disassemblePythonBytes processes raw python instructions and decodes them to human readable instructions
func disassemblePythonBytes(code []byte, consts []pyObject, names, varNames []string, prefix string) string {
	var out strings.Builder
	length := len(code)

	// Opcode constants
	const (
		POP_TOP       = 1
		ROT_TWO       = 2
		ROT_THREE     = 3
		DUP_TOP       = 4
		NOP           = 9
		UNARY_POSITIVE = 10
		UNARY_NEGATIVE = 11
		UNARY_NOT      = 12
		UNARY_INVERT   = 15
		BINARY_POWER   = 19
		BINARY_MULTIPLY = 20
		BINARY_MODULO   = 22
		BINARY_ADD      = 23
		BINARY_SUBTRACT = 24
		BINARY_SUBSCR   = 25
		BINARY_TRUE_DIVIDE = 27
		STORE_MAP       = 54
		RETURN_VALUE    = 83
		POP_BLOCK       = 87
		HAVE_ARGUMENT   = 90 // Opcodes >= 90 have a 1-byte argument (Python <= 3.10)
		STORE_NAME      = 90
		DELETE_NAME     = 91
		LOAD_CONST      = 100
		LOAD_NAME       = 101
		BUILD_TUPLE     = 102
		BUILD_LIST      = 103
		BUILD_SET       = 104
		BUILD_MAP       = 105
		LOAD_ATTR       = 106
		COMPARE_OP      = 107
		IMPORT_NAME     = 108
		IMPORT_FROM     = 109
		JUMP_FORWARD    = 110
		JUMP_IF_FALSE_OR_POP = 111
		JUMP_IF_TRUE_OR_POP  = 112
		JUMP_ABSOLUTE   = 113
		POP_JUMP_IF_FALSE = 114
		POP_JUMP_IF_TRUE  = 115
		LOAD_GLOBAL     = 116
		LOAD_FAST       = 124
		STORE_FAST      = 125
		DELETE_FAST     = 126
		CALL_FUNCTION   = 131
		MAKE_FUNCTION   = 132
	)

	opName := func(op uint8) string {
		switch op {
		case POP_TOP:
			return "POP_TOP"
		case ROT_TWO:
			return "ROT_TWO"
		case ROT_THREE:
			return "ROT_THREE"
		case DUP_TOP:
			return "DUP_TOP"
		case NOP:
			return "NOP"
		case UNARY_POSITIVE:
			return "UNARY_POSITIVE"
		case UNARY_NEGATIVE:
			return "UNARY_NEGATIVE"
		case UNARY_NOT:
			return "UNARY_NOT"
		case UNARY_INVERT:
			return "UNARY_INVERT"
		case BINARY_POWER:
			return "BINARY_POWER"
		case BINARY_MULTIPLY:
			return "BINARY_MULTIPLY"
		case BINARY_MODULO:
			return "BINARY_MODULO"
		case BINARY_ADD:
			return "BINARY_ADD"
		case BINARY_SUBTRACT:
			return "BINARY_SUBTRACT"
		case BINARY_SUBSCR:
			return "BINARY_SUBSCR"
		case BINARY_TRUE_DIVIDE:
			return "BINARY_TRUE_DIVIDE"
		case STORE_MAP:
			return "STORE_MAP"
		case RETURN_VALUE:
			return "RETURN_VALUE"
		case POP_BLOCK:
			return "POP_BLOCK"
		case STORE_NAME:
			return "STORE_NAME"
		case DELETE_NAME:
			return "DELETE_NAME"
		case LOAD_CONST:
			return "LOAD_CONST"
		case LOAD_NAME:
			return "LOAD_NAME"
		case BUILD_TUPLE:
			return "BUILD_TUPLE"
		case BUILD_LIST:
			return "BUILD_LIST"
		case BUILD_SET:
			return "BUILD_SET"
		case BUILD_MAP:
			return "BUILD_MAP"
		case LOAD_ATTR:
			return "LOAD_ATTR"
		case COMPARE_OP:
			return "COMPARE_OP"
		case IMPORT_NAME:
			return "IMPORT_NAME"
		case IMPORT_FROM:
			return "IMPORT_FROM"
		case JUMP_FORWARD:
			return "JUMP_FORWARD"
		case JUMP_IF_FALSE_OR_POP:
			return "JUMP_IF_FALSE_OR_POP"
		case JUMP_IF_TRUE_OR_POP:
			return "JUMP_IF_TRUE_OR_POP"
		case JUMP_ABSOLUTE:
			return "JUMP_ABSOLUTE"
		case POP_JUMP_IF_FALSE:
			return "POP_JUMP_IF_FALSE"
		case POP_JUMP_IF_TRUE:
			return "POP_JUMP_IF_TRUE"
		case LOAD_GLOBAL:
			return "LOAD_GLOBAL"
		case LOAD_FAST:
			return "LOAD_FAST"
		case STORE_FAST:
			return "STORE_FAST"
		case DELETE_FAST:
			return "DELETE_FAST"
		case CALL_FUNCTION:
			return "CALL_FUNCTION"
		case MAKE_FUNCTION:
			return "MAKE_FUNCTION"
		default:
			return fmt.Sprintf("OPCODE_%d", op)
		}
	}

	// Python 3.6+ uses 16-bit wordcode (1-byte opcode, 1-byte argument)
	for i := 0; i < length; i += 2 {
		if i+1 >= length {
			break
		}
		op := code[i]
		arg := code[i+1]

		argRep := ""
		if op >= HAVE_ARGUMENT {
			switch op {
			case LOAD_CONST:
				if int(arg) < len(consts) {
					if nested, ok := consts[arg].(*pyCodeObject); ok {
						argRep = fmt.Sprintf("  # constant nested function '%s'", nested.name)
					} else {
						argRep = fmt.Sprintf("  # constant '%v'", consts[arg])
					}
				}
			case LOAD_NAME, STORE_NAME, LOAD_GLOBAL, LOAD_ATTR, IMPORT_NAME, IMPORT_FROM:
				if int(arg) < len(names) {
					argRep = fmt.Sprintf("  # name '%s'", names[arg])
				}
			case LOAD_FAST, STORE_FAST, DELETE_FAST:
				if int(arg) < len(varNames) {
					argRep = fmt.Sprintf("  # variable '%s'", varNames[arg])
				}
			case CALL_FUNCTION:
				argRep = fmt.Sprintf("  # %d arguments", arg)
			case COMPARE_OP:
				ops := []string{"<", "<=", "==", "!=", ">", ">=", "in", "not in", "is", "is not", "exception match", "BAD"}
				if int(arg) < len(ops) {
					argRep = fmt.Sprintf("  # operator '%s'", ops[arg])
				}
			case JUMP_FORWARD, JUMP_ABSOLUTE, POP_JUMP_IF_FALSE, POP_JUMP_IF_TRUE:
				argRep = fmt.Sprintf("  # target index %d", arg*2)
			}
		}

		if op >= HAVE_ARGUMENT {
			out.WriteString(fmt.Sprintf("%s  %-4d %-20s %-3d%s\n", prefix, i, opName(op), arg, argRep))
		} else {
			out.WriteString(fmt.Sprintf("%s  %-4d %-20s\n", prefix, i, opName(op)))
		}
	}

	return out.String()
}

// ── .NET CLI Metadata Assembly Parser & C# Decompiler ───────────────────────

type fieldInfo struct {
	flags uint16
	name  string
	desc  string
}

type methodInfo struct {
	flags uint16
	name  string
	desc  string
}

type typeInfo struct {
	flags     uint32
	name      string
	namespace string
	fields    []fieldInfo
	methods   []methodInfo
}

func isDotNetAssembly(data []byte) bool {
	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return false
	}
	defer f.Close()

	var clrRVA, clrSize uint32
	switch oh := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		if len(oh.DataDirectory) > 14 {
			clrRVA = oh.DataDirectory[14].VirtualAddress
			clrSize = oh.DataDirectory[14].Size
		}
	case *pe.OptionalHeader64:
		if len(oh.DataDirectory) > 14 {
			clrRVA = oh.DataDirectory[14].VirtualAddress
			clrSize = oh.DataDirectory[14].Size
		}
	}

	if clrRVA == 0 || clrSize == 0 {
		return false
	}

	clrOffset := rvaToOffset(f, clrRVA)
	if clrOffset == 0 || clrOffset+72 > uint32(len(data)) {
		return false
	}

	metaRVA := binary.LittleEndian.Uint32(data[clrOffset+8 : clrOffset+12])
	metaSize := binary.LittleEndian.Uint32(data[clrOffset+12 : clrOffset+16])
	if metaRVA == 0 || metaSize == 0 {
		return false
	}

	metaOffset := rvaToOffset(f, metaRVA)
	if metaOffset == 0 || metaOffset+4 > uint32(len(data)) {
		return false
	}

	sig := binary.BigEndian.Uint32(data[metaOffset : metaOffset+4])
	return sig == 0x42534A42 // "BSJB"
}

func decompileDotNetAssemblyDirect(data []byte) (string, error) {
	decompiled, err := decompileDotNetAssembly(data)
	if err != nil {
		return "", err
	}
	return "# .NET Assembly Decompilation\n\n```csharp\n" + decompiled + "\n```\n", nil
}

func decompileDotNetAssembly(data []byte) (string, error) {
	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer f.Close()

	var clrRVA, clrSize uint32
	switch oh := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		if len(oh.DataDirectory) > 14 {
			clrRVA = oh.DataDirectory[14].VirtualAddress
			clrSize = oh.DataDirectory[14].Size
		}
	case *pe.OptionalHeader64:
		if len(oh.DataDirectory) > 14 {
			clrRVA = oh.DataDirectory[14].VirtualAddress
			clrSize = oh.DataDirectory[14].Size
		}
	}

	if clrRVA == 0 || clrSize == 0 {
		return "", fmt.Errorf("not a .NET assembly (no CLR directory)")
	}

	clrOffset := rvaToOffset(f, clrRVA)
	if clrOffset == 0 || clrOffset+72 > uint32(len(data)) {
		return "", fmt.Errorf("invalid CLR header offset")
	}

	metaRVA := binary.LittleEndian.Uint32(data[clrOffset+8 : clrOffset+12])
	metaSize := binary.LittleEndian.Uint32(data[clrOffset+12 : clrOffset+16])
	if metaRVA == 0 || metaSize == 0 {
		return "", fmt.Errorf("invalid Metadata Directory")
	}

	metaOffset := rvaToOffset(f, metaRVA)
	if metaOffset == 0 || metaOffset+metaSize > uint32(len(data)) {
		return "", fmt.Errorf("invalid Metadata offset")
	}

	versionLen := binary.LittleEndian.Uint32(data[metaOffset+12 : metaOffset+16])
	if metaOffset+16+versionLen > uint32(len(data)) {
		return "", fmt.Errorf("invalid Metadata version string length")
	}

	alignedVersionLen := (versionLen + 3) &^ 3
	offsetStreamsCount := metaOffset + 16 + alignedVersionLen
	if offsetStreamsCount+4 > uint32(len(data)) {
		return "", fmt.Errorf("invalid streams count offset")
	}

	streamsCount := binary.LittleEndian.Uint16(data[offsetStreamsCount+2 : offsetStreamsCount+4])
	streamHeadersOffset := offsetStreamsCount + 4

	var stringsHeap, blobHeap, tablesStream []byte
	currOffset := streamHeadersOffset

	for s := 0; s < int(streamsCount); s++ {
		if currOffset+8 > uint32(len(data)) {
			break
		}
		sOffset := binary.LittleEndian.Uint32(data[currOffset : currOffset+4])
		sSize := binary.LittleEndian.Uint32(data[currOffset+4 : currOffset+8])
		currOffset += 8

		var nameBytes []byte
		for currOffset < uint32(len(data)) {
			b := data[currOffset]
			currOffset++
			if b == 0 {
				break
			}
			nameBytes = append(nameBytes, b)
		}
		currOffset = (currOffset + 3) &^ 3

		sName := string(nameBytes)
		start := metaOffset + sOffset
		end := start + sSize
		if end > uint32(len(data)) {
			continue
		}
		sData := data[start:end]

		switch sName {
		case "#Strings":
			stringsHeap = sData
		case "#Blob":
			blobHeap = sData
		case "#~", "#-":
			tablesStream = sData
		}
	}

	if len(tablesStream) < 24 {
		return "", fmt.Errorf("invalid or missing #~ tables stream")
	}

	heapSizes := tablesStream[6]
	valid := binary.LittleEndian.Uint64(tablesStream[8:16])

	var rowCounts [64]uint32
	rcOffset := uint32(24)
	for i := 0; i < 64; i++ {
		if (valid & (1 << i)) != 0 {
			if rcOffset+4 > uint32(len(tablesStream)) {
				break
			}
			rowCounts[i] = binary.LittleEndian.Uint32(tablesStream[rcOffset : rcOffset+4])
			rcOffset += 4
		}
	}

	var tableData = make(map[int][]byte)
	tbOffset := rcOffset

	stringIdx := uint32(2)
	if (heapSizes & 0x01) != 0 {
		stringIdx = 4
	}
	guidIdx := uint32(2)
	if (heapSizes & 0x02) != 0 {
		guidIdx = 4
	}
	blobIdx := uint32(2)
	if (heapSizes & 0x04) != 0 {
		blobIdx = 4
	}

	for i := 0; i < 64; i++ {
		if rowCounts[i] == 0 {
			continue
		}
		rowSize := getTableRowSize(i, stringIdx, blobIdx, guidIdx, rowCounts)
		totalSize := uint32(rowCounts[i]) * rowSize
		if tbOffset+totalSize <= uint32(len(tablesStream)) {
			tableData[i] = tablesStream[tbOffset : tbOffset+totalSize]
		}
		tbOffset += totalSize
	}

	maxRows := rowCounts[0x02]
	if rowCounts[0x01] > maxRows {
		maxRows = rowCounts[0x01]
	}
	if rowCounts[0x1B] > maxRows {
		maxRows = rowCounts[0x1B]
	}
	typeDefOrRefSize := uint32(2)
	if maxRows >= 16384 {
		typeDefOrRefSize = 4
	}

	fieldIdxSize := uint32(2)
	if rowCounts[0x04] >= 65536 {
		fieldIdxSize = 4
	}
	methodIdxSize := uint32(2)
	if rowCounts[0x06] >= 65536 {
		methodIdxSize = 4
	}

	typeDefRowSize := getTableRowSize(0x02, stringIdx, blobIdx, guidIdx, rowCounts)
	methodRowSize := getTableRowSize(0x06, stringIdx, blobIdx, guidIdx, rowCounts)
	fieldRowSize := getTableRowSize(0x04, stringIdx, blobIdx, guidIdx, rowCounts)

	typeDefData := tableData[0x02]
	methodDefData := tableData[0x06]
	fieldData := tableData[0x04]

	var types []typeInfo

	for i := 0; i < int(rowCounts[0x02]); i++ {
		startOffset := uint32(i) * typeDefRowSize
		if startOffset+typeDefRowSize > uint32(len(typeDefData)) {
			break
		}
		row := typeDefData[startOffset : startOffset+typeDefRowSize]

		flags := binary.LittleEndian.Uint32(row[0:4])
		typeNameIdx := readIndex(row[4:], stringIdx)
		typeNamespaceIdx := readIndex(row[4+stringIdx:], stringIdx)

		fieldListIdx := readIndex(row[4+stringIdx*2+typeDefOrRefSize:], fieldIdxSize)
		methodListIdx := readIndex(row[4+stringIdx*2+typeDefOrRefSize+fieldIdxSize:], methodIdxSize)

		typeName := readHeapString(stringsHeap, typeNameIdx)
		typeNamespace := readHeapString(stringsHeap, typeNamespaceIdx)

		if typeName == "<Module>" || typeName == "" {
			continue
		}

		startField := int(fieldListIdx) - 1
		endField := int(rowCounts[0x04])
		if i+1 < int(rowCounts[0x02]) {
			nextRowOffset := uint32(i+1) * typeDefRowSize
			if nextRowOffset+typeDefRowSize <= uint32(len(typeDefData)) {
				nextRow := typeDefData[nextRowOffset : nextRowOffset+typeDefRowSize]
				nextFieldListIdx := readIndex(nextRow[4+stringIdx*2+typeDefOrRefSize:], fieldIdxSize)
				endField = int(nextFieldListIdx) - 1
			}
		}

		var fields []fieldInfo
		for f := startField; f < endField; f++ {
			if f < 0 || f >= int(rowCounts[0x04]) {
				continue
			}
			fStart := uint32(f) * fieldRowSize
			if fStart+fieldRowSize > uint32(len(fieldData)) {
				continue
			}
			fRow := fieldData[fStart : fStart+fieldRowSize]

			fFlags := binary.LittleEndian.Uint16(fRow[0:2])
			fNameIdx := readIndex(fRow[2:], stringIdx)
			fSigIdx := readIndex(fRow[2+stringIdx:], blobIdx)

			fName := readHeapString(stringsHeap, fNameIdx)
			fSigBlob := readBlob(blobHeap, fSigIdx)
			fType := decodeFieldSignature(fSigBlob)

			fields = append(fields, fieldInfo{
				flags: fFlags,
				name:  fName,
				desc:  fType,
			})
		}

		startMethod := int(methodListIdx) - 1
		endMethod := int(rowCounts[0x06])
		if i+1 < int(rowCounts[0x02]) {
			nextRowOffset := uint32(i+1) * typeDefRowSize
			if nextRowOffset+typeDefRowSize <= uint32(len(typeDefData)) {
				nextRow := typeDefData[nextRowOffset : nextRowOffset+typeDefRowSize]
				nextMethodListIdx := readIndex(nextRow[4+stringIdx*2+typeDefOrRefSize+fieldIdxSize:], methodIdxSize)
				endMethod = int(nextMethodListIdx) - 1
			}
		}

		var methods []methodInfo
		for m := startMethod; m < endMethod; m++ {
			if m < 0 || m >= int(rowCounts[0x06]) {
				continue
			}
			mStart := uint32(m) * methodRowSize
			if mStart+methodRowSize > uint32(len(methodDefData)) {
				continue
			}
			mRow := methodDefData[mStart : mStart+methodRowSize]

			mFlags := binary.LittleEndian.Uint16(mRow[6:8])
			mNameIdx := readIndex(mRow[8:], stringIdx)
			mSigIdx := readIndex(mRow[8+stringIdx:], blobIdx)

			mName := readHeapString(stringsHeap, mNameIdx)
			mSigBlob := readBlob(blobHeap, mSigIdx)
			args, ret := decodeMethodSignature(mSigBlob)

			var argDescs []string
			for j, arg := range args {
				argDescs = append(argDescs, fmt.Sprintf("%s param%d", arg, j+1))
			}
			methodDesc := fmt.Sprintf("%s(%s)", mName, strings.Join(argDescs, ", "))
			if ret != "" {
				methodDesc = ret + " " + methodDesc
			}

			methods = append(methods, methodInfo{
				flags: mFlags,
				name:  mName,
				desc:  methodDesc,
			})
		}

		types = append(types, typeInfo{
			flags:     flags,
			name:      typeName,
			namespace: typeNamespace,
			fields:    fields,
			methods:   methods,
		})
	}

	var cs strings.Builder
	cs.WriteString("// .NET Assembly Decompilation (C#)\n")

	if rowCounts[0x20] > 0 && len(tableData[0x20]) > 0 {
		row := tableData[0x20][0:getTableRowSize(0x20, stringIdx, blobIdx, guidIdx, rowCounts)]
		nameIdx := readIndex(row[16+blobIdx:], stringIdx)
		assemblyName := readHeapString(stringsHeap, nameIdx)

		major := binary.LittleEndian.Uint16(row[4:6])
		minor := binary.LittleEndian.Uint16(row[6:8])
		build := binary.LittleEndian.Uint16(row[8:10])
		rev := binary.LittleEndian.Uint16(row[10:12])
		cs.WriteString(fmt.Sprintf("// Assembly: %s, Version %d.%d.%d.%d\n\n", assemblyName, major, minor, build, rev))
	} else {
		cs.WriteString("// Assembly: Unknown\n\n")
	}

	byNamespace := make(map[string][]typeInfo)
	for _, t := range types {
		byNamespace[t.namespace] = append(byNamespace[t.namespace], t)
	}

	var namespaces []string
	for ns := range byNamespace {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	for _, ns := range namespaces {
		hasNamespace := ns != ""
		indent := ""
		if hasNamespace {
			cs.WriteString(fmt.Sprintf("namespace %s\n{\n", ns))
			indent = "    "
		}

		for _, t := range byNamespace[ns] {
			classAccess := ""
			acc := t.flags & 0x00000007
			switch acc {
			case 0, 3:
				classAccess = "private"
			case 1, 2:
				classAccess = "public"
			case 4:
				classAccess = "protected"
			case 5:
				classAccess = "internal"
			default:
				classAccess = "public"
			}

			classKind := "class"
			if t.flags&0x00000020 != 0 {
				if t.flags&0x00000100 != 0 {
					classAccess += " static"
				} else {
					classAccess += " abstract"
				}
			} else if t.flags&0x00000100 != 0 {
				classAccess += " sealed"
			}
			if t.flags&0x00000080 != 0 {
				classKind = "interface"
			}

			cs.WriteString(fmt.Sprintf("%s%s %s %s\n%s{\n", indent, classAccess, classKind, t.name, indent))

			for _, f := range t.fields {
				fAccess := "private"
				switch f.flags & 0x0007 {
				case 1:
					fAccess = "private"
				case 3:
					fAccess = "internal"
				case 4:
					fAccess = "protected"
				case 6:
					fAccess = "public"
				}

				fMods := ""
				if f.flags&0x0010 != 0 {
					fMods += " static"
				}
				if f.flags&0x0040 != 0 {
					fMods += " readonly"
				}
				if f.flags&0x8000 != 0 {
					fAccess = "public"
					fMods = " const"
				}

				cs.WriteString(fmt.Sprintf("%s    %s%s %s;\n", indent, fAccess, fMods, f.desc))
			}

			if len(t.fields) > 0 && len(t.methods) > 0 {
				cs.WriteString("\n")
			}

			for _, m := range t.methods {
				mAccess := "private"
				switch m.flags & 0x0007 {
				case 1:
					mAccess = "private"
				case 3:
					mAccess = "internal"
				case 4:
					mAccess = "protected"
				case 6:
					mAccess = "public"
				}

				mMods := ""
				if m.flags&0x0010 != 0 {
					mMods += " static"
				}
				if m.flags&0x0040 != 0 {
					mMods += " virtual"
				}
				if m.flags&0x0400 != 0 {
					mMods += " abstract"
				}

				mNameClean := m.name
				if mNameClean == ".ctor" {
					mNameClean = t.name
					parts := strings.Split(m.desc, " ")
					if len(parts) > 1 {
						m.desc = strings.Join(parts[1:], " ")
					}
					m.desc = strings.Replace(m.desc, ".ctor", mNameClean, 1)
				} else if mNameClean == ".cctor" {
					cs.WriteString(fmt.Sprintf("%s    static %s()\n%s    {\n%s        // Static constructor\n%s    }\n\n", indent, t.name, indent, indent, indent))
					continue
				}

				cs.WriteString(fmt.Sprintf("%s    %s%s %s\n%s    {\n%s        // Intermediate Language (CIL) bytecode omitted\n%s    }\n\n", indent, mAccess, mMods, m.desc, indent, indent, indent))
			}

			cs.WriteString(fmt.Sprintf("%s}\n\n", indent))
		}

		if hasNamespace {
			cs.WriteString("}\n")
		}
	}

	return cs.String(), nil
}

func rvaToOffset(f *pe.File, rva uint32) uint32 {
	for _, sec := range f.Sections {
		if rva >= sec.VirtualAddress && rva < sec.VirtualAddress+sec.VirtualSize {
			return rva - sec.VirtualAddress + sec.Offset
		}
	}
	return 0
}

func readIndex(buf []byte, size uint32) uint32 {
	if len(buf) < int(size) {
		return 0
	}
	if size == 2 {
		return uint32(binary.LittleEndian.Uint16(buf[:2]))
	}
	return binary.LittleEndian.Uint32(buf[:4])
}

func readHeapString(stringsHeap []byte, offset uint32) string {
	if offset >= uint32(len(stringsHeap)) {
		return ""
	}
	end := offset
	for end < uint32(len(stringsHeap)) && stringsHeap[end] != 0 {
		end++
	}
	return string(stringsHeap[offset:end])
}

func readBlob(blobStream []byte, offset uint32) []byte {
	if offset == 0 || offset >= uint32(len(blobStream)) {
		return nil
	}
	length, rest := readCompressedInt(blobStream[offset:])
	if length == 0 || uint32(len(rest)) < length {
		return nil
	}
	return rest[:length]
}

func readCompressedInt(data []byte) (uint32, []byte) {
	if len(data) == 0 {
		return 0, nil
	}
	b0 := data[0]
	if b0 < 0x80 {
		return uint32(b0), data[1:]
	} else if b0 < 0xC0 {
		if len(data) < 2 {
			return 0, nil
		}
		return uint32(b0&0x3F)<<8 | uint32(data[1]), data[2:]
	} else {
		if len(data) < 4 {
			return 0, nil
		}
		return uint32(b0&0x1F)<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]), data[4:]
	}
}

func decodeDotNetType(blob []byte) (string, []byte) {
	if len(blob) == 0 {
		return "object", nil
	}
	etype := blob[0]
	blob = blob[1:]

	switch etype {
	case 0x01:
		return "void", blob
	case 0x02:
		return "bool", blob
	case 0x03:
		return "char", blob
	case 0x04:
		return "sbyte", blob
	case 0x05:
		return "byte", blob
	case 0x06:
		return "short", blob
	case 0x07:
		return "ushort", blob
	case 0x08:
		return "int", blob
	case 0x09:
		return "uint", blob
	case 0x0a:
		return "long", blob
	case 0x0b:
		return "ulong", blob
	case 0x0c:
		return "float", blob
	case 0x0d:
		return "double", blob
	case 0x0e:
		return "string", blob
	case 0x1c:
		return "object", blob
	case 0x1d: // SzArray
		element, rest := decodeDotNetType(blob)
		return element + "[]", rest
	case 0x11, 0x12: // ValueType, Class
		_, rest := readCompressedInt(blob)
		return "Type", rest
	case 0x15: // GenericInst
		if len(blob) == 0 {
			return "GenericType", blob
		}
		_, rest := decodeDotNetType(blob)
		count, rest2 := readCompressedInt(rest)
		var args []string
		curr := rest2
		for i := 0; i < int(count); i++ {
			var arg string
			arg, curr = decodeDotNetType(curr)
			args = append(args, arg)
		}
		return "GenericType<" + strings.Join(args, ", ") + ">", curr
	default:
		return "object", blob
	}
}

func decodeMethodSignature(blob []byte) (args []string, ret string) {
	if len(blob) == 0 {
		return nil, "void"
	}
	blob = blob[1:] // Calling convention
	paramCount, rest := readCompressedInt(blob)

	ret, rest = decodeDotNetType(rest)
	curr := rest
	for i := 0; i < int(paramCount); i++ {
		if len(curr) == 0 {
			break
		}
		var arg string
		arg, curr = decodeDotNetType(curr)
		args = append(args, arg)
	}
	return args, ret
}

func decodeFieldSignature(blob []byte) string {
	if len(blob) < 2 {
		return "object"
	}
	if blob[0] != 0x06 {
		return "object"
	}
	fType, _ := decodeDotNetType(blob[1:])
	return fType
}

func getTableRowSize(tableID int, stringIdx, blobIdx, guidIdx uint32, rowCounts [64]uint32) uint32 {
	codedIdxSize := func(tables []int, tagBits int) uint32 {
		maxRows := uint32(0)
		for _, t := range tables {
			if rowCounts[t] > maxRows {
				maxRows = rowCounts[t]
			}
		}
		if maxRows < (65536 >> tagBits) {
			return 2
		}
		return 4
	}

	tableIdxSize := func(t int) uint32 {
		if rowCounts[t] < 65536 {
			return 2
		}
		return 4
	}

	switch tableID {
	case 0x00: // Module
		return 2 + stringIdx + guidIdx*3
	case 0x01: // TypeRef
		resScopeSize := codedIdxSize([]int{0x00, 0x1A, 0x23, 0x01}, 2)
		return resScopeSize + stringIdx*2
	case 0x02: // TypeDef
		typeDefOrRefSize := codedIdxSize([]int{0x02, 0x01, 0x1B}, 2)
		return 4 + stringIdx*2 + typeDefOrRefSize + tableIdxSize(0x04) + tableIdxSize(0x06)
	case 0x04: // Field
		return 2 + stringIdx + blobIdx
	case 0x06: // MethodDef
		return 4 + 2 + 2 + stringIdx + blobIdx + tableIdxSize(0x08)
	case 0x08: // Param
		return 2 + 2 + stringIdx
	case 0x09: // InterfaceImpl
		typeDefOrRefSize := codedIdxSize([]int{0x02, 0x01, 0x1B}, 2)
		return tableIdxSize(0x02) + typeDefOrRefSize
	case 0x0A: // MemberRef
		parentSize := codedIdxSize([]int{0x01, 0x02, 0x06, 0x1A, 0x1B}, 3)
		return parentSize + stringIdx + blobIdx
	case 0x0C: // CustomAttribute
		parentSize := uint32(2)
		for _, t := range []int{0x00, 0x01, 0x02, 0x04, 0x06, 0x08, 0x20, 0x23} {
			if rowCounts[t] >= 2048 {
				parentSize = 4
				break
			}
		}
		typeSize := codedIdxSize([]int{0x06, 0x0A}, 3)
		return parentSize + typeSize + blobIdx
	case 0x1B: // TypeSpec
		return blobIdx
	case 0x20: // Assembly
		return 4 + 2*4 + 4 + blobIdx + stringIdx*2
	case 0x23: // AssemblyRef
		return 2*4 + 4 + blobIdx*2 + stringIdx*2
	default:
		return 2
	}
}
