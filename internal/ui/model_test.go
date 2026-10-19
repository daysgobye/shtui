package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"omasheet/internal/workbook"
)

func TestCellEditorAcceptsSpaceUnicodeAndPaste(t *testing.T) {
	model := newInteractionModel(t, 1)
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Quarterly")})
	model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("plan 東京 📊"), Paste: true})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	got := model.session.Workbook.Sheets[0].Cells[workbook.Coord{}].Input
	if got != "Quarterly plan 東京 📊" {
		t.Fatalf("cell input = %q, want %q", got, "Quarterly plan 東京 📊")
	}
}

func TestEnterCommitsAndMovesDown(t *testing.T) {
	model := newInteractionModel(t, 1)
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("first")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if got := model.session.Workbook.Sheets[0].Cells[workbook.Coord{}].Input; got != "first" {
		t.Fatalf("committed A1 = %q, want first", got)
	}
	if model.Grid.Selected != (workbook.Coord{Row: 1, Col: 0}) || model.editing {
		t.Fatalf("after Enter: selected=%+v editing=%t, want A2 outside edit mode", model.Grid.Selected, model.editing)
	}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if !model.editing || !strings.HasSuffix(model.formulaBar(), "n") {
		t.Fatalf("next typed value did not start a fresh edit: editing=%t formula=%q", model.editing, model.formulaBar())
	}
}

func TestTabCommitsAndMovesRight(t *testing.T) {
	model := newInteractionModel(t, 1)
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("across")})
	model.Update(tea.KeyMsg{Type: tea.KeyTab})

	if got := model.session.Workbook.Sheets[0].Cells[workbook.Coord{}].Input; got != "across" {
		t.Fatalf("committed A1 = %q, want across", got)
	}
	if model.Grid.Selected != (workbook.Coord{Row: 0, Col: 1}) || model.editing {
		t.Fatalf("after Tab: selected=%+v editing=%t, want B1 outside edit mode", model.Grid.Selected, model.editing)
	}
}

func TestSheetNavigationSupportsKeyboardAndMouseTabs(t *testing.T) {
	model := newInteractionModel(t, 2)
	model.session.Workbook.ActiveSheet = 0
	model.Grid.Selected = workbook.Coord{Row: 7, Col: 4}
	model.Grid.ScrollRow = 5
	model.Grid.ScrollCol = 2
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlPgDown})
	if model.session.Workbook.ActiveSheet != 1 {
		t.Errorf("Ctrl+PageDown active sheet = %d, want 1", model.session.Workbook.ActiveSheet)
	}
	if model.Grid.Selected != (workbook.Coord{}) || model.Grid.ScrollRow != 0 || model.Grid.ScrollCol != 0 {
		t.Errorf("keyboard sheet change did not reset navigation: selected=%+v scroll=(%d,%d)", model.Grid.Selected, model.Grid.ScrollRow, model.Grid.ScrollCol)
	}
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlPgUp})
	if model.session.Workbook.ActiveSheet != 0 {
		t.Errorf("Ctrl+PageUp active sheet = %d, want 0", model.session.Workbook.ActiveSheet)
	}

	model.Grid.Selected = workbook.Coord{Row: 3, Col: 2}
	model.Grid.ScrollRow = 2
	model.session.Workbook.ActiveSheet = 0
	model.handleMouse(tea.MouseMsg(tea.MouseEvent{X: 10, Y: model.height - 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if model.session.Workbook.ActiveSheet != 1 || model.Grid.Selected != (workbook.Coord{}) || model.Grid.ScrollRow != 0 {
		t.Fatalf("mouse tab change lost its behavior: sheet=%d selected=%+v scroll=%d", model.session.Workbook.ActiveSheet, model.Grid.Selected, model.Grid.ScrollRow)
	}
}

func TestFormulaEditorCalculatesSUMRange(t *testing.T) {
	model := newInteractionModel(t, 1)
	for row := 0; row < 5; row++ {
		if err := workbook.SetCell(model.session, workbook.CellAddress{Sheet: "Sheet1", Row: row, Col: 0}, itoa(row+1)); err != nil {
			t.Fatal(err)
		}
	}
	model.Grid.Selected = workbook.Coord{Row: 5}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("=SUM(A1:A5)")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	cell := model.session.Workbook.Sheets[0].Cells[workbook.Coord{Row: 5}]
	if cell.Input != "=SUM(A1:A5)" || cell.Result != "15" {
		t.Fatalf("formula cell = (%q, %q), want (=SUM(A1:A5), 15)", cell.Input, cell.Result)
	}
}

func TestFunctionPickerPrefillsSelectedRange(t *testing.T) {
	model := newInteractionModel(t, 1)
	for row := 0; row < 5; row++ {
		if err := workbook.SetCell(model.session, workbook.CellAddress{Sheet: "Sheet1", Row: row, Col: 0}, itoa(row+1)); err != nil {
			t.Fatal(err)
		}
	}
	model.Grid.Selected = workbook.Coord{Row: 4}
	model.Grid.Selection = workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 4}}
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("insert function")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !model.FunctionPicker.Open {
		t.Fatalf("Insert function command did not open: command=%q palette=%t filter=%q", model.lastCommand, model.Palette.Open, model.Palette.Query)
	}
	view := model.View()
	for _, expected := range []string{"Insert function", "SUM(range)", "A1:A5", "A6", "=SUM(A1:A5)"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("function picker view is missing %q: %s", expected, view)
		}
	}
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sheet := model.session.Workbook.Sheets[0]
	if got := sheet.Cells[workbook.Coord{Row: 4}].Input; got != "5" {
		t.Fatalf("source A5 was overwritten: input=%q", got)
	}
	result := sheet.Cells[workbook.Coord{Row: 5}]
	if result.Input != "=SUM(A1:A5)" || result.Result != "15" {
		t.Fatalf("function result A6 = (%q, %q), want (=SUM(A1:A5), 15)", result.Input, result.Result)
	}
}

func TestFunctionPickerAcceptsAllThreeIFArguments(t *testing.T) {
	model := newInteractionModel(t, 1)
	if err := workbook.SetCell(model.session, workbook.CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "1"); err != nil {
		t.Fatal(err)
	}
	model.runCommand("insert-function")
	for range 5 {
		model.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if got := model.FunctionPicker.Syntax(); got != "IF(condition, true value, false value)" {
		t.Fatalf("selected function syntax = %q, want IF", got)
	}
	if model.FunctionPicker.RangeInput.Value() != "" || model.FunctionPicker.RangeInput.Prompt != "Arguments: " {
		t.Fatalf("IF picker retained range prompt/value: prompt=%q value=%q", model.FunctionPicker.RangeInput.Prompt, model.FunctionPicker.RangeInput.Value())
	}
	model.FunctionPicker.RangeInput.SetValue(`A1>0,"yes","no"`)
	model.FunctionPicker.TargetInput.SetValue("B1")
	model.FunctionPicker.Focus = 2
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	result := model.session.Workbook.Sheets[0].Cells[workbook.Coord{Col: 1}]
	if result.Input != `=IF(A1>0,"yes","no")` || result.Result != "yes" {
		t.Fatalf("IF result = (%q, %q), want (=IF(A1>0,\"yes\",\"no\"), yes)", result.Input, result.Result)
	}
}

func newInteractionModel(t *testing.T, sheetCount int) AppModel {
	t.Helper()
	session, err := workbook.NewWorkbook(workbook.CSV)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	for index := 1; index < sheetCount; index++ {
		if err := workbook.AddSheet(session, "Sheet"+itoa(index+1)); err != nil {
			t.Fatal(err)
		}
	}
	session.Workbook.ActiveSheet = 0
	return NewModel(session)
}
