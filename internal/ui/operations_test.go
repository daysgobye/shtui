package ui

import (
	"strings"
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
	merged := session.Workbook.Sheets[0]
	if got := merged.Cells[workbook.Coord{}].Input; got != "keep" {
		t.Fatalf("merged top-left input = %q, want keep", got)
	}
	if got := merged.Cells[workbook.Coord{Col: 1}].Input; got != "" {
		t.Fatalf("discarded merged input = %q, want blank", got)
	}
	if err := workbook.Undo(session); err != nil {
		t.Fatal(err)
	}
	if len(session.Workbook.Sheets[0].Merges) != 0 || session.Workbook.Sheets[0].Cells[workbook.Coord{Col: 1}].Input != "discard" {
		t.Fatal("undo did not restore merged values and metadata")
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

func TestBorderMenuAppliesPresetToSelectedRange(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	model := NewModel(session)
	model.Grid.Selection = workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1, Col: 1}}
	if err := workbook.ApplyStyle(session, session.Workbook.Sheets[0].Name, workbook.CellRange{}, workbook.CellStyle{FillRGB: "112233", Bold: true}); err != nil {
		t.Fatal(err)
	}

	model.runCommand("border-all")
	for row := 0; row <= 1; row++ {
		for col := 0; col <= 1; col++ {
			borders := session.Workbook.Sheets[0].Cells[workbook.Coord{Row: row, Col: col}].Style.Borders
			if borders.Top.Preset != "thin" || borders.Right.Preset != "thin" || borders.Bottom.Preset != "thin" || borders.Left.Preset != "thin" {
				t.Errorf("All preset at (%d,%d) = %#v", row, col, borders)
			}
		}
	}
	if cell := session.Workbook.Sheets[0].Cells[workbook.Coord{}]; cell.Style.FillRGB != "112233" || !cell.Style.Bold {
		t.Fatalf("All border preset replaced non-border formatting: %#v", cell.Style)
	}
	if err := workbook.Undo(session); err != nil {
		t.Fatal(err)
	}
	for row := 0; row <= 1; row++ {
		for col := 0; col <= 1; col++ {
			if borders := session.Workbook.Sheets[0].Cells[workbook.Coord{Row: row, Col: col}].Style.Borders; borders != (workbook.BorderSet{}) {
				t.Fatalf("undo of border preset left borders at (%d,%d): %#v", row, col, borders)
			}
		}
	}
	if cell := session.Workbook.Sheets[0].Cells[workbook.Coord{}]; cell.Style.FillRGB != "112233" || !cell.Style.Bold {
		t.Fatalf("undo of border preset changed fill/font style: %#v", cell.Style)
	}
	if err := workbook.Redo(session); err != nil {
		t.Fatal(err)
	}

	model.runCommand("border-none")
	for row := 0; row <= 1; row++ {
		for col := 0; col <= 1; col++ {
			borders := session.Workbook.Sheets[0].Cells[workbook.Coord{Row: row, Col: col}].Style.Borders
			if borders != (workbook.BorderSet{}) {
				t.Errorf("None preset at (%d,%d) = %#v, want no borders", row, col, borders)
			}
		}
	}
}

func TestBorderToolbarOpensPresetMenu(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.CSV)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	model := NewModel(session)
	borderX := -1
	for x := range 300 {
		if toolbarActionAt(x) == "border-menu" {
			borderX = x
			break
		}
	}
	if borderX < 0 {
		t.Fatal("toolbar has no Borders action")
	}
	model.handleMouse(tea.MouseMsg(tea.MouseEvent{X: borderX, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if !model.Palette.Open || model.Palette.Query != "border" {
		t.Fatalf("Borders action did not open filtered presets: open=%t query=%q", model.Palette.Open, model.Palette.Query)
	}
	for _, id := range []string{"border-all", "border-outside", "border-top", "border-bottom", "border-left", "border-right", "border-none"} {
		found := false
		for _, item := range model.Palette.list.VisibleItems() {
			if item, ok := item.(paletteItem); ok && item.command.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Borders menu does not show %q", id)
		}
	}
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	borders := session.Workbook.Sheets[0].Cells[workbook.Coord{}].Style.Borders
	if !strings.HasPrefix(model.lastCommand, "border-") || borders == (workbook.BorderSet{}) {
		t.Fatalf("choosing a border preset left command=%q borders=%#v", model.lastCommand, borders)
	}
}

func TestPaletteIncludesCoreWorkbookOperations(t *testing.T) {
	palette := NewCommandPalette(DefaultKeyMap())
	for _, expected := range []string{"sort", "filter", "border-all", "border-outside", "border-top", "border-bottom", "border-left", "border-right", "border-none", "merge", "unmerge", "conditional-format", "toggle-bold", "add-sheet", "open-file", "save-as"} {
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
