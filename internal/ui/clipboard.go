package ui

import (
	"bytes"
	"encoding/csv"
	"strings"

	"github.com/atotto/clipboard"
	"shtui/internal/workbook"
)

type Clipboard interface {
	Read() (string, error)
	Write(string) error
}

type systemClipboard struct{}

func (systemClipboard) Read() (string, error)   { return clipboard.ReadAll() }
func (systemClipboard) Write(text string) error { return clipboard.WriteAll(text) }

func matrixTSV(matrix [][]string) string {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.Comma = '\t'
	for _, row := range matrix {
		_ = writer.Write(row)
	}
	writer.Flush()
	return strings.TrimSuffix(buffer.String(), "\n")
}

func parseTSV(text string) ([][]string, error) {
	if text == "" {
		return [][]string{{""}}, nil
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	return reader.ReadAll()
}

func (model *AppModel) clipboardForUse() Clipboard {
	if model.clipboard == nil {
		model.clipboard = systemClipboard{}
	}
	return model.clipboard
}

func (model *AppModel) copySelection(cut bool) {
	sheet := model.activeSheet()
	if sheet == nil {
		return
	}
	bounds := model.selectedRange()
	matrix, err := workbook.CopyRange(model.session, sheet.Name, bounds)
	if err == nil {
		err = model.clipboardForUse().Write(matrixTSV(matrix))
	}
	if err == nil && cut {
		err = workbook.ClearRange(model.session, sheet.Name, bounds)
	}
	model.setStatus(err)
	if err == nil && cut {
		model.recordActionSummary("Cut cells")
	}
}

func (model *AppModel) pasteSelection() {
	sheet := model.activeSheet()
	if sheet == nil {
		return
	}
	text, err := model.clipboardForUse().Read()
	if err == nil {
		matrix, parseErr := parseTSV(text)
		if parseErr != nil {
			err = parseErr
		} else {
			err = workbook.PasteRange(model.session, workbook.CellAddress{
				Sheet: sheet.Name, Row: model.Grid.Selected.Row, Col: model.Grid.Selected.Col,
			}, matrix)
		}
	}
	model.setStatus(err)
	if err == nil {
		model.recordActionSummary("Pasted cells")
	}
}

func (model *AppModel) copyRow(cut bool) {
	sheet := model.activeSheet()
	if sheet == nil {
		return
	}
	maxCol := -1
	for coord := range sheet.Cells {
		if coord.Row == model.Grid.Selected.Row && coord.Col > maxCol {
			maxCol = coord.Col
		}
	}
	if maxCol < 0 {
		model.status = "Active row has no used cells"
		return
	}
	bounds := workbook.CellRange{
		Start: workbook.Coord{Row: model.Grid.Selected.Row},
		End:   workbook.Coord{Row: model.Grid.Selected.Row, Col: maxCol},
	}
	matrix, err := workbook.CopyRange(model.session, sheet.Name, bounds)
	if err == nil {
		err = model.clipboardForUse().Write(matrixTSV(matrix))
	}
	if err == nil && cut {
		err = workbook.ClearRange(model.session, sheet.Name, bounds)
	}
	model.setStatus(err)
	if err == nil && cut {
		model.recordActionSummary("Cut row")
	}
}

func (model *AppModel) uniqueDuplicateSheetName() string {
	nameIndex := len(model.session.Workbook.Sheets) + 1
	for {
		name := "Sheet" + itoa(nameIndex)
		used := false
		for _, sheet := range model.session.Workbook.Sheets {
			if sheet.Name == name {
				used = true
				break
			}
		}
		if !used {
			return name
		}
		nameIndex++
	}
}
