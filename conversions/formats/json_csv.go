package formats

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(jsonToCSV{})
	conversions.Register(csvToJSON{})
}

type jsonToCSV struct{}

func (jsonToCSV) Name() string        { return "json-csv" }
func (jsonToCSV) Category() string    { return "formats" }
func (jsonToCSV) Description() string { return "Convert JSON array of objects to CSV" }

func (jsonToCSV) Convert(input string) (string, error) {
	var rows []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &rows); err != nil {
		return "", fmt.Errorf("input must be a JSON array of objects: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}

	// collect and sort headers for deterministic output
	headerSet := map[string]struct{}{}
	for _, row := range rows {
		for k := range row {
			headerSet[k] = struct{}{}
		}
	}
	headers := make([]string, 0, len(headerSet))
	for k := range headerSet {
		headers = append(headers, k)
	}
	sort.Strings(headers)

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(headers); err != nil {
		return "", err
	}
	for _, row := range rows {
		record := make([]string, len(headers))
		for i, h := range headers {
			if v, ok := row[h]; ok {
				record[i] = fmt.Sprintf("%v", v)
			}
		}
		if err := w.Write(record); err != nil {
			return "", err
		}
	}
	w.Flush()
	return buf.String(), w.Error()
}

type csvToJSON struct{}

func (csvToJSON) Name() string        { return "csv-json" }
func (csvToJSON) Category() string    { return "formats" }
func (csvToJSON) Description() string { return "Convert CSV (with header row) to JSON array" }

func (csvToJSON) Convert(input string) (string, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimSpace(input)))
	records, err := r.ReadAll()
	if err != nil {
		return "", fmt.Errorf("invalid CSV: %w", err)
	}
	if len(records) < 2 {
		return "[]", nil
	}
	headers := records[0]
	var rows []map[string]any
	for _, record := range records[1:] {
		row := map[string]any{}
		for i, h := range headers {
			if i < len(record) {
				row[h] = record[i]
			}
		}
		rows = append(rows, row)
	}
	out, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}
