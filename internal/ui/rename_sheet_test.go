package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"shtui/internal/workbook"
)

func TestRenameSheetCommandUpdatesReferencesAndUndo(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if err := workbook.AddSheet(session, "Data"); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetCell(session, workbook.CellAddress{Sheet: "Data", Row: 0, Col: 0}, "7"); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetCell(session, workbook.CellAddress{Sheet: "Sheet1", Row: 0, Col: 1}, "=SUM(Data!A1)"); err != nil {
		t.Fatal(err)
	}

	model := NewModel(session)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	model.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, character := range "rename sheet" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{character}})
	}
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if view := ansi.Strip(model.View()); !strings.Contains(view, "Rename sheet") || !strings.Contains(view, "Sheet name: Data") {
		t.Fatalf("rename command did not open a prefilled dialog: %q", view)
	}
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Sales Data")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if got := session.Workbook.Sheets[1].Name; got != "Sales Data" {
		t.Fatalf("renamed sheet = %q, want Sales Data", got)
	}
	formula := session.Workbook.Sheets[0].Cells[workbook.Coord{Row: 0, Col: 1}]
	if formula.Input != "=SUM('Sales Data'!A1)" || formula.Result != "7" || formula.CalculationState != workbook.Current {
		t.Fatalf("formula after rename = (%q, %q, %q), want updated source and current result 7", formula.Input, formula.Result, formula.CalculationState)
	}
	if got := model.TakeActionSummary(); got != "Renamed sheet to Sales Data" {
		t.Fatalf("rename action summary = %q", got)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if got := session.Workbook.Sheets[1].Name; got != "Data" {
		t.Fatalf("sheet name after undo = %q, want Data", got)
	}
	formula = session.Workbook.Sheets[0].Cells[workbook.Coord{Row: 0, Col: 1}]
	if formula.Input != "=SUM(Data!A1)" || formula.Result != "7" || formula.CalculationState != workbook.Current {
		t.Fatalf("formula after undo = (%q, %q, %q), want restored source and current result 7", formula.Input, formula.Result, formula.CalculationState)
	}
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	if got := session.Workbook.Sheets[1].Name; got != "Sales Data" {
		t.Fatalf("sheet name after redo = %q, want Sales Data", got)
	}
	formula = session.Workbook.Sheets[0].Cells[workbook.Coord{Row: 0, Col: 1}]
	if formula.Input != "=SUM('Sales Data'!A1)" || formula.Result != "7" || formula.CalculationState != workbook.Current {
		t.Fatalf("formula after redo = (%q, %q, %q), want renamed source and current result 7", formula.Input, formula.Result, formula.CalculationState)
	}
}

func TestRenameSheetDialogMouseRename(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if err := workbook.AddSheet(session, "Data"); err != nil {
		t.Fatal(err)
	}
	model := NewModel(session)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model.runCommand("rename-sheet")
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Archive")})
	view := ansi.Strip(model.View())
	x, y := renderedLastActionPoint(t, view, "Rename")
	model.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if got := session.Workbook.Sheets[1].Name; got != "Archive" {
		t.Fatalf("mouse Rename left sheet named %q, want Archive", got)
	}
}

func TestRenameSheetRejectsDuplicateAndMouseCancelPreservesSheet(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if err := workbook.AddSheet(session, "Data"); err != nil {
		t.Fatal(err)
	}
	model := NewModel(session)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model.runCommand("rename-sheet")
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Sheet1")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := ansi.Strip(model.View())
	if !strings.Contains(view, `sheet "Sheet1" already exists`) {
		t.Fatalf("duplicate name did not leave a visible validation error: %q", view)
	}
	if got := session.Workbook.Sheets[1].Name; got != "Data" {
		t.Fatalf("invalid rename changed sheet name to %q", got)
	}
	x, y := renderedActionPoint(t, view, "Cancel")
	model.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if strings.Contains(ansi.Strip(model.View()), "already exists") {
		t.Fatalf("mouse Cancel left the rename dialog open: %q", ansi.Strip(model.View()))
	}
	if got := session.Workbook.Sheets[1].Name; got != "Data" {
		t.Fatalf("mouse Cancel changed sheet name to %q", got)
	}
}

func TestRenameSheetEscapeCancelsWithoutMutation(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if err := workbook.AddSheet(session, "Data"); err != nil {
		t.Fatal(err)
	}
	model := NewModel(session)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model.runCommand("rename-sheet")
	model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := session.Workbook.Sheets[1].Name; got != "Data" {
		t.Fatalf("Esc changed sheet name to %q", got)
	}
	if strings.Contains(ansi.Strip(model.View()), "Rename sheet") {
		t.Fatalf("Esc left rename dialog open: %q", ansi.Strip(model.View()))
	}
}

func TestRenameSheetDialogUnchangedNameDoesNotDirtyWorkbook(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	model := NewModel(session)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model.runCommand("rename-sheet")
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if session.Workbook.Dirty {
		t.Fatal("submitting the unchanged sheet name marked the workbook dirty")
	}
	if got := model.TakeActionSummary(); got != "" {
		t.Fatalf("unchanged name recorded action summary %q", got)
	}
}

func renderedLastActionPoint(t *testing.T, view, label string) (int, int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	for y := len(lines) - 1; y >= 0; y-- {
		if x := strings.LastIndex(lines[y], label); x >= 0 {
			return ansi.StringWidth(lines[y][:x]), y
		}
	}
	t.Fatalf("rendered view has no %q action: %q", label, view)
	return 0, 0
}
