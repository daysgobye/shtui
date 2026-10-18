package workbook

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// ReadCSV loads CSV data into one sparse sheet. path is retained for later saves.
func ReadCSV(source io.Reader, path string) (*Workbook, error) {
	reader := csv.NewReader(source)
	reader.FieldsPerRecord = -1

	workbook := &Workbook{
		Format: CSV,
		Path:   path,
		Sheets: []Sheet{{Name: "Sheet1", Cells: make(map[Coord]Cell), ColumnWidths: make(map[int]float64)}},
	}
	sheet := &workbook.Sheets[0]
	for row := 0; ; row++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV %q: %w", path, err)
		}
		sheet.Rows++
		if len(record) > sheet.Cols {
			sheet.Cols = len(record)
		}
		for col, input := range record {
			if input == "" {
				continue
			}
			cell := Cell{Input: input, Result: input, CalculationState: Literal}
			if strings.HasPrefix(input, "=") {
				cell.Result = ""
				cell.CalculationState = Stale
			}
			sheet.Cells[Coord{Row: row, Col: col}] = cell
		}
	}
	return workbook, nil
}

// WriteCSV exports source input; formula cells remain formula text, not cached results.
func WriteCSV(destination io.Writer, workbook *Workbook, sheetIndex int) error {
	if workbook == nil {
		return fmt.Errorf("write CSV: workbook is nil")
	}
	if sheetIndex < 0 || sheetIndex >= len(workbook.Sheets) {
		return fmt.Errorf("write CSV: sheet index %d is out of range", sheetIndex)
	}

	sheet := &workbook.Sheets[sheetIndex]
	if sheet.Rows < 0 || sheet.Cols < 0 {
		return fmt.Errorf("write CSV: invalid sheet dimensions %dx%d", sheet.Rows, sheet.Cols)
	}
	writer := csv.NewWriter(destination)
	for row := range sheet.Rows {
		record := make([]string, sheet.Cols)
		for col := range sheet.Cols {
			if cell, exists := sheet.Cells[Coord{Row: row, Col: col}]; exists {
				record[col] = cell.Input
			}
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write CSV row %d: %w", row+1, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush CSV %q: %w", workbook.Path, err)
	}
	return nil
}
