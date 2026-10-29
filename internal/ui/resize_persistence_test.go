package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/xuri/excelize/v2"
	"shtui/internal/workbook"
)

func TestMouseResizedExistingXLSXColumnPersistsAfterSaveAndReopen(t *testing.T) {
	source := filepath.Join("..", "workbook", "testdata", "preservation", "chart.xlsx")
	path := filepath.Join(t.TempDir(), "chart.xlsx")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read chart fixture: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("copy chart fixture: %v", err)
	}

	session, err := workbook.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	model := NewModel(session)
	model.layout()
	sheet := &session.Workbook.Sheets[0]
	initialWidth := model.Grid.columnWidth(0, sheet)
	originalWidth := sheet.ColumnWidths[0]
	const drag = 4
	boundary := model.Grid.RowHeaderWidth + initialWidth
	y := model.Grid.OriginY
	for _, event := range []tea.MouseEvent{
		{X: boundary, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft},
		{X: boundary + drag, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft},
		{X: boundary + drag, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft},
	} {
		model.Update(tea.MouseMsg(event))
	}

	wantWidth := originalWidth + float64(drag)
	if got := sheet.ColumnWidths[0]; got != wantWidth {
		t.Fatalf("resize boundary: workbook model width for column A = %v, want %v", got, wantWidth)
	}
	if err := workbook.SaveFile(session, path); err != nil {
		t.Fatalf("save boundary: SaveFile: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close boundary: %v", err)
	}
	reopened, err := workbook.OpenFile(path)
	if err != nil {
		t.Fatalf("reopen boundary: OpenFile: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	reopenedSheet := &reopened.Workbook.Sheets[0]
	if got := reopenedSheet.ColumnWidths[0]; got != wantWidth {
		t.Fatalf("reopen boundary: workbook model width for column A = %v, want %v", got, wantWidth)
	}

	excelFile, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("Excelize open boundary: %v", err)
	}
	t.Cleanup(func() { _ = excelFile.Close() })
	gotExcelWidth, err := excelFile.GetColWidth(reopenedSheet.Name, "A")
	if err != nil {
		t.Fatalf("Excelize GetColWidth boundary: %v", err)
	}
	if gotExcelWidth != wantWidth {
		t.Fatalf("Excelize width boundary: column A width = %v, want %v", gotExcelWidth, wantWidth)
	}

	reopenedModel := NewModel(reopened)
	reopenedModel.layout()
	rendered := ansi.Strip(reopenedModel.Grid.View(reopenedSheet, reopenedModel.Grid.Height))
	columnBoundary := reopenedModel.Grid.RowHeaderWidth + int(wantWidth)
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 || ansi.StringWidth(lines[0]) < columnBoundary || ansi.Cut(lines[0], columnBoundary-1, columnBoundary) != "│" {
		t.Fatalf("render boundary: reopened grid column A does not end at column %d: %q", columnBoundary-1, rendered)
	}
}
func TestMouseResizedEmptyXLSXColumnPersistsAfterSaveAndReopen(t *testing.T) {
	source := filepath.Join("..", "workbook", "testdata", "preservation", "chart.xlsx")
	path := filepath.Join(t.TempDir(), "chart.xlsx")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read chart fixture: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("copy chart fixture: %v", err)
	}

	session, err := workbook.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	model := NewModel(session)
	model.layout()
	sheet := &session.Workbook.Sheets[0]
	column := sheet.Cols
	initialWidth := model.Grid.columnWidth(column, sheet)
	originalWidth := initialWidth
	const drag = 4
	boundary := model.Grid.RowHeaderWidth + initialWidth*(column+1) - 1
	y := model.Grid.OriginY
	for _, event := range []tea.MouseEvent{
		{X: boundary, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft},
		{X: boundary + drag, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft},
		{X: boundary + drag, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft},
	} {
		model.Update(tea.MouseMsg(event))
	}

	wantWidth := float64(originalWidth + drag)
	if got := sheet.ColumnWidths[column]; got != wantWidth {
		t.Fatalf("resize boundary: workbook model width for empty column %d = %v, want %v", column, got, wantWidth)
	}
	if err := workbook.SaveFile(session, path); err != nil {
		t.Fatalf("save boundary: SaveFile: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close boundary: %v", err)
	}

	excelFile, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("Excelize open boundary: %v", err)
	}
	columnName, err := excelize.ColumnNumberToName(column + 1)
	if err != nil {
		_ = excelFile.Close()
		t.Fatalf("Excelize column name boundary: %v", err)
	}
	gotExcelWidth, err := excelFile.GetColWidth(sheet.Name, columnName)
	if err != nil {
		_ = excelFile.Close()
		t.Fatalf("Excelize GetColWidth boundary: %v", err)
	}
	if err := excelFile.Close(); err != nil {
		t.Fatalf("Excelize close boundary: %v", err)
	}
	if gotExcelWidth != wantWidth {
		t.Fatalf("Excelize width boundary: empty column %d width = %v, want %v", column, gotExcelWidth, wantWidth)
	}

	reopened, err := workbook.OpenFile(path)
	if err != nil {
		t.Fatalf("reopen boundary: OpenFile: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	reopenedSheet := &reopened.Workbook.Sheets[0]
	if got := reopenedSheet.ColumnWidths[column]; got != wantWidth {
		t.Fatalf("reopen model boundary: empty column %d width = %v, want %v (Excelize reports %v)", column, got, wantWidth, gotExcelWidth)
	}
	reopenedModel := NewModel(reopened)
	reopenedModel.layout()
	rendered := ansi.Strip(reopenedModel.Grid.View(reopenedSheet, reopenedModel.Grid.Height))
	columnBoundary := reopenedModel.Grid.RowHeaderWidth + int(wantWidth)*(column+1)
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 || ansi.StringWidth(lines[0]) < columnBoundary || ansi.Cut(lines[0], columnBoundary-1, columnBoundary) != "│" {
		t.Fatalf("render boundary: reopened grid empty column %d does not end at column %d: %q", column, columnBoundary-1, rendered)
	}
}
