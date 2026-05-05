package formats

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"

	"github.com/wingitman/atob/conversions"
	internalxlsx "github.com/wingitman/atob/conversions/internal/xlsx"
)

func init() {
	conversions.RegisterFile(csvToXLSX{})
	conversions.RegisterFile(xlsxToCSV{})
}

type csvToXLSX struct{}

func (csvToXLSX) Name() string        { return "csv-xlsx" }
func (csvToXLSX) Category() string    { return "formats" }
func (csvToXLSX) Description() string { return "Convert CSV file to XLSX (file path required)" }

func (csvToXLSX) ConvertFile(inputPath, outputPath string) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("cannot open input file: %w", err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return fmt.Errorf("invalid CSV: %w", err)
	}

	data, err := internalxlsx.WriteToBytes(records)
	if err != nil {
		return fmt.Errorf("XLSX write error: %w", err)
	}
	return os.WriteFile(outputPath, data, 0o644)
}

type xlsxToCSV struct{}

func (xlsxToCSV) Name() string        { return "xlsx-csv" }
func (xlsxToCSV) Category() string    { return "formats" }
func (xlsxToCSV) Description() string { return "Convert XLSX file to CSV (file path required)" }

func (xlsxToCSV) ConvertFile(inputPath, outputPath string) error {
	fileData, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("cannot open XLSX file: %w", err)
	}

	rows, err := internalxlsx.Read(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return fmt.Errorf("cannot read XLSX: %w", err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("no data found in XLSX file")
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer out.Close()

	w := csv.NewWriter(out)
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
