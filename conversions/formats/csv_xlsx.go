package formats

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/wingitman/atob/conversions"
	"github.com/xuri/excelize/v2"
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

	xl := excelize.NewFile()
	defer xl.Close()
	sheet := "Sheet1"
	for i, record := range records {
		for j, val := range record {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			xl.SetCellValue(sheet, cell, val)
		}
	}
	return xl.SaveAs(outputPath)
}

type xlsxToCSV struct{}

func (xlsxToCSV) Name() string        { return "xlsx-csv" }
func (xlsxToCSV) Category() string    { return "formats" }
func (xlsxToCSV) Description() string { return "Convert XLSX file to CSV (file path required)" }

func (xlsxToCSV) ConvertFile(inputPath, outputPath string) error {
	xl, err := excelize.OpenFile(inputPath)
	if err != nil {
		return fmt.Errorf("cannot open XLSX file: %w", err)
	}
	defer xl.Close()

	sheets := xl.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("no sheets found in XLSX file")
	}

	rows, err := xl.GetRows(sheets[0])
	if err != nil {
		return fmt.Errorf("cannot read sheet: %w", err)
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
