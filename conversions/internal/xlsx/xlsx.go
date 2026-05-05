// Package xlsx provides minimal XLSX (Office Open XML spreadsheet) read and
// write support using only the Go standard library (archive/zip + encoding/xml).
//
// Writing: creates a valid .xlsx file with a single sheet from a 2-D string
// table ([][]string).
//
// Reading: opens a .xlsx file and returns the cell values of the first
// worksheet as a [][]string table.
//
// Only string-typed cells are supported for round-trip fidelity of CSV data.
// Numeric cells are returned as their string representation.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ── Write ─────────────────────────────────────────────────────────────────────

// Write creates an XLSX file (written to w) from a 2-D string table.
func Write(w io.Writer, rows [][]string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	// Shared strings table: all cell values stored here to keep cell XML small.
	ss := &sharedStrings{}
	sheetXML := buildSheetXML(rows, ss)

	files := map[string][]byte{
		"[Content_Types].xml":            contentTypesXML(),
		"_rels/.rels":                    rootRelsXML(),
		"xl/workbook.xml":                workbookXML(),
		"xl/_rels/workbook.xml.rels":     workbookRelsXML(),
		"xl/sharedStrings.xml":           ss.xml(),
		"xl/worksheets/sheet1.xml":       sheetXML,
		"xl/styles.xml":                  stylesXML(),
		"docProps/app.xml":               appXML(),
	}

	// Fixed order to keep the zip reproducible.
	order := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"xl/workbook.xml",
		"xl/_rels/workbook.xml.rels",
		"xl/sharedStrings.xml",
		"xl/worksheets/sheet1.xml",
		"xl/styles.xml",
		"docProps/app.xml",
	}

	for _, name := range order {
		f, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("xlsx write %s: %w", name, err)
		}
		if _, err := f.Write(files[name]); err != nil {
			return fmt.Errorf("xlsx write %s content: %w", name, err)
		}
	}
	return nil
}

// ── Read ──────────────────────────────────────────────────────────────────────

// Read opens an XLSX file from r (size bytes) and returns the first sheet's
// rows as a 2-D string slice.
func Read(r io.ReaderAt, size int64) ([][]string, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("xlsx: not a valid zip/xlsx: %w", err)
	}

	// Load shared strings table.
	sharedStrs, err := readSharedStrings(zr)
	if err != nil {
		return nil, err
	}

	// Find the first sheet relationship.
	sheetPath, err := findFirstSheetPath(zr)
	if err != nil {
		return nil, err
	}

	return readSheet(zr, sheetPath, sharedStrs)
}

// ── Shared strings ────────────────────────────────────────────────────────────

type sharedStrings struct {
	strs  []string
	index map[string]int
}

func (ss *sharedStrings) add(s string) int {
	if ss.index == nil {
		ss.index = make(map[string]int)
	}
	if i, ok := ss.index[s]; ok {
		return i
	}
	i := len(ss.strs)
	ss.strs = append(ss.strs, s)
	ss.index[s] = i
	return i
}

func (ss *sharedStrings) xml() []byte {
	var b strings.Builder
	b.WriteString(xml.Header)
	count := len(ss.strs)
	fmt.Fprintf(&b, `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">`, count, count)
	for _, s := range ss.strs {
		b.WriteString("<si><t>")
		xml.EscapeText(&b, []byte(s))
		b.WriteString("</t></si>")
	}
	b.WriteString("</sst>")
	return []byte(b.String())
}

// ── Sheet XML builder ─────────────────────────────────────────────────────────

func buildSheetXML(rows [][]string, ss *sharedStrings) []byte {
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	b.WriteString("<sheetData>")

	for ri, row := range rows {
		rowNum := ri + 1
		fmt.Fprintf(&b, `<row r="%d">`, rowNum)
		for ci, val := range row {
			cellRef := cellName(ci, rowNum)
			si := ss.add(val)
			// t="s" means shared string type
			fmt.Fprintf(&b, `<c r="%s" t="s"><v>%d</v></c>`, cellRef, si)
		}
		b.WriteString("</row>")
	}

	b.WriteString("</sheetData></worksheet>")
	return []byte(b.String())
}

// cellName converts 0-based column and 1-based row to Excel cell reference (e.g. A1, B2).
func cellName(col, row int) string {
	return colName(col) + strconv.Itoa(row)
}

// colName converts a 0-based column index to an Excel column letter (A, B, …, Z, AA, …).
func colName(n int) string {
	name := ""
	for n >= 0 {
		name = string(rune('A'+n%26)) + name
		n = n/26 - 1
	}
	return name
}

// ── Static XML fragments ──────────────────────────────────────────────────────

func contentTypesXML() []byte {
	return []byte(xml.Header + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`)
}

func rootRelsXML() []byte {
	return []byte(xml.Header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`)
}

func workbookXML() []byte {
	return []byte(xml.Header + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`)
}

func workbookRelsXML() []byte {
	return []byte(xml.Header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`)
}

func stylesXML() []byte {
	return []byte(xml.Header + `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>
  <fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>
  <borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs>
</styleSheet>`)
}

func appXML() []byte {
	return []byte(xml.Header + `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>atob</Application>
</Properties>`)
}

// ── Reader helpers ────────────────────────────────────────────────────────────

// readSharedStrings reads xl/sharedStrings.xml from the zip and returns a
// slice of strings indexed by position.
func readSharedStrings(zr *zip.Reader) ([]string, error) {
	f := findZipFile(zr, "xl/sharedStrings.xml")
	if f == nil {
		return nil, nil // no shared strings — inline values only
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	type si struct {
		T string `xml:"t"`
	}
	type sst struct {
		Items []si `xml:"si"`
	}
	var s sst
	if err := xml.NewDecoder(rc).Decode(&s); err != nil {
		return nil, fmt.Errorf("xlsx: cannot parse sharedStrings: %w", err)
	}
	strs := make([]string, len(s.Items))
	for i, item := range s.Items {
		strs[i] = item.T
	}
	return strs, nil
}

// findFirstSheetPath resolves the first sheet's path from workbook.xml.rels.
func findFirstSheetPath(zr *zip.Reader) (string, error) {
	// Parse xl/_rels/workbook.xml.rels to find the sheet.
	f := findZipFile(zr, "xl/_rels/workbook.xml.rels")
	if f == nil {
		// Fall back to default path.
		return "xl/worksheets/sheet1.xml", nil
	}
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	type rel struct {
		Type   string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
	}
	type rels struct {
		Items []rel `xml:"Relationship"`
	}
	var r rels
	if err := xml.NewDecoder(rc).Decode(&r); err != nil {
		return "xl/worksheets/sheet1.xml", nil
	}
	for _, rel := range r.Items {
		if strings.Contains(rel.Type, "worksheet") {
			target := rel.Target
			if !strings.HasPrefix(target, "xl/") {
				target = "xl/" + strings.TrimPrefix(target, "/")
			}
			return target, nil
		}
	}
	return "xl/worksheets/sheet1.xml", nil
}

// readSheet parses the worksheet XML and returns rows as [][]string.
func readSheet(zr *zip.Reader, path string, sharedStrs []string) ([][]string, error) {
	f := findZipFile(zr, path)
	if f == nil {
		return nil, fmt.Errorf("xlsx: sheet not found: %s", path)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// Streaming parse: row → c (cell) elements.
	type cell struct {
		Ref  string `xml:"r,attr"`
		Type string `xml:"t,attr"`
		V    string `xml:"v"`
	}
	type row struct {
		R     int    `xml:"r,attr"`
		Cells []cell `xml:"c"`
	}

	var rows [][]string
	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xlsx: XML parse error: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "row" {
			continue
		}
		var r row
		if err := dec.DecodeElement(&r, &se); err != nil {
			return nil, err
		}
		if len(r.Cells) == 0 {
			continue
		}

		// Determine max column index in this row.
		maxCol := 0
		for _, c := range r.Cells {
			col := colIndex(c.Ref)
			if col > maxCol {
				maxCol = col
			}
		}

		rowData := make([]string, maxCol+1)
		for _, c := range r.Cells {
			col := colIndex(c.Ref)
			val := c.V
			if c.Type == "s" {
				// Shared string
				idx, err := strconv.Atoi(val)
				if err == nil && idx >= 0 && idx < len(sharedStrs) {
					val = sharedStrs[idx]
				}
			}
			rowData[col] = val
		}
		rows = append(rows, rowData)
	}
	return rows, nil
}

// colIndex converts a cell reference like "A1" or "BC42" to a 0-based column index.
func colIndex(ref string) int {
	n := 0
	for _, c := range ref {
		if c < 'A' || c > 'Z' {
			break
		}
		n = n*26 + int(c-'A') + 1
	}
	return n - 1
}

// findZipFile finds a file in the zip by name (case-insensitive).
func findZipFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, name) {
			return f
		}
	}
	return nil
}

// WriteToBytes is a convenience helper that writes an XLSX file to a []byte.
func WriteToBytes(rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	if err := Write(&buf, rows); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
