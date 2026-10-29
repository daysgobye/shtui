package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"shtui/internal/workbook"
)

func TestWelcomeWordmarkIsProminentAndCompact(t *testing.T) {
	model := NewModel(nil)
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 30, Height: 10}} {
		model.Update(size)
		view := strings.TrimRight(ansi.Strip(model.View()), "\n")
		for _, action := range []string{"Open workbook", "New CSV", "New XLSX"} {
			if !strings.Contains(view, action) {
				t.Errorf("welcome screen lacks %q at %dx%d: %q", action, size.Width, size.Height, view)
			}
		}
		if !strings.Contains(view, "Ctrl+P") || !strings.Contains(view, "Enter") {
			t.Errorf("welcome screen lacks keyboard hints at %dx%d: %q", size.Width, size.Height, view)
		}
		if size.Width >= 42 && strings.Count(view, "█") < 40 {
			t.Errorf("normal welcome screen lacks the prominent SHTUI wordmark at %dx%d: %q", size.Width, size.Height, view)
		}
		if size.Width < 42 && !strings.Contains(view, "SHTUI") {
			t.Errorf("compact welcome screen lacks the SHTUI wordmark at %dx%d: %q", size.Width, size.Height, view)
		}
		lines := strings.Split(view, "\n")
		if len(lines) > size.Height {
			t.Errorf("welcome screen height = %d at %dx%d", len(lines), size.Width, size.Height)
		}
		for index, line := range lines {
			if width := lipgloss.Width(line); width > size.Width {
				t.Errorf("welcome screen line %d width = %d at %dx%d", index, width, size.Width, size.Height)
			}
		}
	}
}

func TestWelcomeScreenMouseSelectsWorkbookAction(t *testing.T) {
	model := NewModel(nil)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	x, y := renderedActionPoint(t, model.View(), "New CSV")
	model.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if action := model.TakeCommand(); action != "new-csv" {
		t.Fatalf("clicking New CSV emitted %q, want new-csv", action)
	}
}

func TestWelcomeScreenMouseIgnoresOutsidePanel(t *testing.T) {
	model := NewModel(nil)
	view := model.View()
	_, y := renderedActionPoint(t, view, "New CSV")
	panelLeft := -1
	for _, line := range strings.Split(ansi.Strip(view), "\n") {
		if left := strings.Index(line, "╭"); left >= 0 {
			panelLeft = left
			break
		}
	}
	if panelLeft <= 0 {
		t.Fatalf("welcome panel is not inset from the terminal edge: %q", ansi.Strip(view))
	}
	model.Update(tea.MouseMsg(tea.MouseEvent{X: panelLeft - 1, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if action := model.TakeCommand(); action != "" {
		t.Fatalf("click outside the visible panel emitted %q", action)
	}
}

func TestWelcomeScreenKeyboardChoosesSelectedAction(t *testing.T) {
	model := NewModel(nil)
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if action := model.TakeCommand(); action != "new-csv" {
		t.Fatalf("down then Enter emitted %q, want new-csv", action)
	}
}

func renderedActionPoint(t *testing.T, view, label string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(view), "\n") {
		if x := strings.Index(line, label); x >= 0 {
			return x, y
		}
	}
	t.Fatalf("rendered view has no %q action: %q", label, ansi.Strip(view))
	return 0, 0
}

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

func TestKeyboardMovementSkipsMergedInterior(t *testing.T) {
	model := newInteractionModel(t, 1)
	merge := workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 2, Col: 2}}
	model.activeSheet().Merges = []workbook.CellRange{merge}

	model.selectCell(workbook.Coord{Row: 1, Col: 1})
	if model.Grid.Selected != merge.Start {
		t.Fatalf("selectCell inside merge selected %v; want anchor %v", model.Grid.Selected, merge.Start)
	}
	model.Grid.Selected = merge.Start
	model.moveSelection(0, 1)
	if want := (workbook.Coord{Row: 0, Col: 3}); model.Grid.Selected != want {
		t.Fatalf("moving right from merge selected %v; want first cell after merge %v", model.Grid.Selected, want)
	}
	model.Grid.Selected = merge.Start
	model.moveSelection(1, 0)
	if want := (workbook.Coord{Row: 3, Col: 0}); model.Grid.Selected != want {
		t.Fatalf("moving down from merge selected %v; want first cell below merge %v", model.Grid.Selected, want)
	}
	model.Grid.Selected = workbook.Coord{Row: 0, Col: 3}
	model.moveSelection(0, -1)
	if model.Grid.Selected != merge.Start {
		t.Fatalf("moving left into merge selected %v; want anchor %v", model.Grid.Selected, merge.Start)
	}
	model.Grid.Selected = workbook.Coord{Row: 3, Col: 0}
	model.moveSelection(-1, 0)
	if model.Grid.Selected != merge.Start {
		t.Fatalf("moving up into merge selected %v; want anchor %v", model.Grid.Selected, merge.Start)
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

func TestSheetTabClickUsesRenderedRow(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		width, height int
		toolbar       bool
	}{
		{name: "toolbar visible normal", width: 80, height: 24, toolbar: true},
		{name: "toolbar hidden normal", width: 80, height: 24},
		{name: "toolbar visible compact", width: 30, height: 10, toolbar: true},
		{name: "toolbar hidden compact", width: 30, height: 10},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			model := newInteractionModel(t, 2)
			model.width, model.height = testCase.width, testCase.height
			model.toolbarVisible = testCase.toolbar
			model.Grid.Selected = workbook.Coord{Row: 7, Col: 4}
			model.Grid.ScrollRow = 5
			model.Grid.ScrollCol = 2

			lines := strings.Split(model.View(), "\n")
			tabY, tabX := -1, -1
			for y, line := range lines {
				if x := strings.Index(line, "Sheet2"); x >= 0 {
					tabY, tabX = y, lipgloss.Width(line[:x])
					break
				}
			}
			if tabY < 0 {
				t.Fatalf("rendered view has no Sheet2 tab: %q", lines)
			}

			model.Update(tea.MouseMsg(tea.MouseEvent{
				X: tabX, Y: tabY, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
			}))

			if model.session.Workbook.ActiveSheet != 1 {
				t.Fatalf("active sheet = %d, want 1 after clicking rendered tab row %d", model.session.Workbook.ActiveSheet, tabY)
			}
			if model.Grid.Selected != (workbook.Coord{}) || model.Grid.ScrollRow != 0 || model.Grid.ScrollCol != 0 {
				t.Fatalf("sheet click did not reset navigation: selected=%+v scroll=(%d,%d)", model.Grid.Selected, model.Grid.ScrollRow, model.Grid.ScrollCol)
			}
		})
	}
}

func TestAddSheetPlusClickCreatesAndActivatesSheet(t *testing.T) {
	model := newInteractionModel(t, 2)
	model.width, model.height = 80, 24

	lines := strings.Split(model.View(), "\n")
	tabY, tabX := -1, -1
	for y, line := range lines {
		if strings.Contains(line, "Sheet2") {
			if x := strings.Index(line, "+"); x >= 0 {
				tabY, tabX = y, lipgloss.Width(line[:x])
				break
			}
		}
	}
	if tabY < 0 {
		t.Fatalf("rendered view has no Add Sheet + target: %q", lines)
	}

	model.Update(tea.MouseMsg(tea.MouseEvent{
		X: tabX, Y: tabY, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))

	if len(model.session.Workbook.Sheets) != 3 {
		t.Fatalf("sheet count = %d, want 3 after clicking +", len(model.session.Workbook.Sheets))
	}
	if model.session.Workbook.Sheets[model.session.Workbook.ActiveSheet].Name != "Sheet3" {
		t.Fatalf("active sheet = %q, want Sheet3", model.session.Workbook.Sheets[model.session.Workbook.ActiveSheet].Name)
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
	for range len(supportedFunctions) {
		if model.FunctionPicker.selectedFunctionName() == "IF" {
			break
		}
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
type testClipboard struct {
	text      string
	readErr   error
	writeErr  error
	writeText string
}

func (clipboard *testClipboard) Read() (string, error) { return clipboard.text, clipboard.readErr }
func (clipboard *testClipboard) Write(text string) error {
	clipboard.writeText = text
	return clipboard.writeErr
}

func TestClipboardShortcutsOperateOnSelectedRange(t *testing.T) {
	model := newInteractionModel(t, 1)
	clipboard := &testClipboard{}
	model.clipboard = clipboard
	sheet := model.activeSheet()
	sheet.Cells[workbook.Coord{Row: 0, Col: 0}] = workbook.Cell{Input: "=1+2"}
	sheet.Cells[workbook.Coord{Row: 1, Col: 1}] = workbook.Cell{Input: "edge"}
	model.Grid.Selection = workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1, Col: 1}}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+shift+c")})
	if clipboard.writeText != "=1+2\t\n\tedge" {
		t.Fatalf("copied TSV = %q, want rectangular values with empty edges", clipboard.writeText)
	}
	model.selectCell(workbook.Coord{Row: 3, Col: 3})
	clipboard.text = "=A1\tB1\n\t"
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	if got := sheet.Cells[workbook.Coord{Row: 3, Col: 3}].Input; got != "=A1" {
		t.Fatalf("pasted formula input = %q, want =A1", got)
	}
}

func TestClipboardCutFailurePreservesSource(t *testing.T) {
	model := newInteractionModel(t, 1)
	model.Grid.Selected = workbook.Coord{}
	if err := workbook.SetCell(model.session, workbook.CellAddress{Sheet: model.activeSheet().Name}, "keep"); err != nil {
		t.Fatal(err)
	}
	clipboard := &testClipboard{writeErr: errors.New("clipboard unavailable")}
	model.clipboard = clipboard
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if got := model.activeSheet().Cells[workbook.Coord{}].Input; got != "keep" {
		t.Fatalf("cut failure changed source to %q", got)
	}
	if !strings.Contains(model.status, "clipboard unavailable") {
		t.Fatalf("status = %q, want clipboard error", model.status)
	}
}

func TestPaletteClipboardAndStructuralCommands(t *testing.T) {
	model := newInteractionModel(t, 1)
	clipboard := &testClipboard{}
	model.clipboard = clipboard
	want := map[string]bool{
		"copy": true, "cut": true, "paste": true, "copy-row": true, "cut-row": true,
		"delete-row": true, "delete-column": true, "duplicate-sheet": true,
	}
	for _, item := range newPaletteItems(model.keys) {
		paletteItem, ok := item.(paletteItem)
		if ok {
			delete(want, paletteItem.command.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("palette lacks commands: %#v", want)
	}
	model.runCommand("duplicate-sheet")
	if len(model.session.Workbook.Sheets) != 2 {
		t.Fatalf("sheets = %d, want duplicate sheet", len(model.session.Workbook.Sheets))
	}
}

func TestCopyCutRowUseActiveCellRow(t *testing.T) {
	model := newInteractionModel(t, 1)
	clipboard := &testClipboard{}
	model.clipboard = clipboard
	model.activeSheet().Cells[workbook.Coord{Row: 0, Col: 0}] = workbook.Cell{Input: "other"}
	model.activeSheet().Cells[workbook.Coord{Row: 2, Col: 2}] = workbook.Cell{Input: "row"}
	model.Grid.Selected = workbook.Coord{Row: 2, Col: 2}
	model.runCommand("copy-row")
	if clipboard.writeText != "\t\trow" {
		t.Fatalf("copied row TSV = %q, want used-column span", clipboard.writeText)
	}
	model.runCommand("cut-row")
	if got := model.activeSheet().Cells[workbook.Coord{Row: 0, Col: 0}].Input; got != "other" {
		t.Fatalf("cut row changed another row to %q", got)
	}
	if got := model.activeSheet().Cells[workbook.Coord{Row: 2, Col: 2}].Input; got != "" {
		t.Fatalf("cut row left active row input %q", got)
	}
}

func TestClipboardModalPrecedenceAndCtrlCQuit(t *testing.T) {
	model := newInteractionModel(t, 1)
	clipboard := &testClipboard{}
	model.clipboard = clipboard
	model.Palette.Open = true
	clipboard.text = "must not paste"
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	if len(model.activeSheet().Cells) != 0 {
		t.Fatal("clipboard shortcut leaked through palette modal")
	}
	model.Palette.Open = false
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if model.TakeCommand() != "" {
		t.Fatal("Ctrl+C was consumed as a spreadsheet command")
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
