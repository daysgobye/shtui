package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"omasheet/internal/workbook"
)

func TestSortAndFilterCommandsOperateOnWorkbook(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.CSV)
	if err != nil {
		t.Fatal(err)
	}
	sheet := session.Workbook.Sheets[0].Name
	for _, item := range []struct {
		row, col int
		value    string
	}{{0, 0, "name"}, {1, 0, "pear"}, {2, 0, "apple"}} {
		if err := workbook.SetCell(session, workbook.CellAddress{Sheet: sheet, Row: item.row, Col: item.col}, item.value); err != nil {
			t.Fatal(err)
		}
	}
	model := NewModel(session)
	model.runCommand("sort")
	model.applyOperation("sort", "a")
	if got := session.Workbook.Sheets[0].Cells[workbook.Coord{Row: 1}].Input; got != "apple" {
		t.Fatalf("sorted A2 = %q, want apple", got)
	}
	if got := session.Workbook.Sheets[0].Cells[workbook.Coord{Row: 2}].Input; got != "pear" {
		t.Fatalf("sorted A3 = %q, want pear", got)
	}
	model.runCommand("filter")
	model.applyOperation("filter", "apple")
	model.layout()
	wantRows := []int{0, 1}
	if len(model.Grid.VisibleRowIndexes) != len(wantRows) {
		t.Fatalf("visible rows = %v, want %v", model.Grid.VisibleRowIndexes, wantRows)
	}
	for index, row := range wantRows {
		if model.Grid.VisibleRowIndexes[index] != row {
			t.Fatalf("visible rows = %v, want %v", model.Grid.VisibleRowIndexes, wantRows)
		}
	}
	model.applyOperation("filter", "")
	model.layout()
	if len(model.Grid.VisibleRowIndexes) != 0 {
		t.Fatalf("cleared filter left visible-row mapping %v", model.Grid.VisibleRowIndexes)
	}
}

func TestMergeConfirmationAndConditionalFormattingCommands(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	sheet := session.Workbook.Sheets[0].Name
	for _, item := range []struct {
		row, col int
		value    string
	}{{0, 0, "keep"}, {0, 1, "discard"}} {
		if err := workbook.SetCell(session, workbook.CellAddress{Sheet: sheet, Row: item.row, Col: item.col}, item.value); err != nil {
			t.Fatal(err)
		}
	}
	model := NewModel(session)
	model.Grid.Selection = workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 0, Col: 1}}
	model.runCommand("merge")
	if model.operation.Mode != "merge-confirm" {
		t.Fatalf("merge did not request value-loss confirmation: mode=%q", model.operation.Mode)
	}
	model.handleOperationKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model.handleOperationKey(tea.KeyMsg{Type: tea.KeyEnter})
	if len(session.Workbook.Sheets[0].Merges) != 1 {
		t.Fatalf("confirmed merge count = %d, want 1", len(session.Workbook.Sheets[0].Merges))
	}
	model.Grid.Selection = workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1, Col: 1}}
	model.runCommand("conditional-format")
	model.applyOperation("conditional", ">1")
	if !model.ColorPicker.Open || model.ColorPicker.Target != conditionalColorTarget {
		t.Fatal("conditional-format command did not open theme color picker")
	}
	model.applyColor(RGB{R: 20, G: 40, B: 60})
	if len(session.Workbook.Sheets[0].ConditionalRules) != 1 {
		t.Fatalf("conditional rule count = %d, want 1", len(session.Workbook.Sheets[0].ConditionalRules))
	}
	if got := session.Workbook.Sheets[0].ConditionalRules[0].FillRGB; got != "14283C" {
		t.Fatalf("conditional fill = %q, want 14283C", got)
	}
}

func TestPaletteIncludesCoreWorkbookOperations(t *testing.T) {
	palette := NewCommandPalette(DefaultKeyMap())
	for _, expected := range []string{"sort", "filter", "border", "merge", "unmerge", "conditional-format", "toggle-bold", "add-sheet", "open-file", "save-as"} {
		found := false
		for _, item := range palette.list.Items() {
			if item, ok := item.(paletteItem); ok && item.command.ID == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("command palette missing %q", expected)
		}
	}
}

func TestPaletteCanFormatAndAddSheets(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.CSV)
	if err != nil {
		t.Fatal(err)
	}
	model := NewModel(session)
	model.runCommand("toggle-bold")
	if !session.Workbook.Sheets[0].Cells[workbook.Coord{}].Style.Bold {
		t.Fatal("toggle-bold did not format the selected cell")
	}
	model.runCommand("add-sheet")
	if len(session.Workbook.Sheets) != 2 || session.Workbook.ActiveSheet != 1 || session.Workbook.Sheets[1].Name != "Sheet2" {
		t.Fatalf("new sheet state = sheets %#v active %d", session.Workbook.Sheets, session.Workbook.ActiveSheet)
	}
}

func TestFilteredNavigationSkipsHiddenRows(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.CSV)
	if err != nil {
		t.Fatal(err)
	}
	sheet := session.Workbook.Sheets[0].Name
	for row, value := range []string{"fruit", "pear", "apple"} {
		if err := workbook.SetCell(session, workbook.CellAddress{Sheet: sheet, Row: row, Col: 0}, value); err != nil {
			t.Fatal(err)
		}
	}
	model := NewModel(session)
	model.runCommand("filter")
	model.applyOperation("filter", "apple")
	model.layout()
	model.moveSelection(1, 0)
	if model.Grid.Selected.Row != 2 {
		t.Fatalf("Down selected row %d, want visible row 3 (zero-based 2)", model.Grid.Selected.Row)
	}
}
