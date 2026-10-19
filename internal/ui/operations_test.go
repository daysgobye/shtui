package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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

func TestMergeConfirmationAndUndo(t *testing.T) {
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
}

func TestConditionalDialogPrefillsRangeAndAppliesRule(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")
	view := model.View()
	for _, expected := range []string{"Conditional formatting", "Single color", "A1:A2"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("conditional dialog missing %q: %s", expected, view)
		}
	}
	pressDialogEnter(&model, 3)
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	chooseThemeRed(&model)
	pressDialogTab(&model, 3)
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	rules := model.session.Workbook.Sheets[0].ConditionalRules
	if model.ConditionalDialog.Open || len(rules) != 1 || rules[0].Range != (workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}}) {
		t.Fatalf("Done did not apply the selected range: open=%t focus=%d error=%q picker=%#v rules=%#v", model.ConditionalDialog.Open, model.ConditionalDialog.Focus, model.ConditionalDialog.Error, model.ColorPicker, rules)
	}
	if rules[0].Operator != workbook.GreaterThan || rules[0].Value != "5" || rules[0].FillRGB != "CD0000" {
		t.Fatalf("applied conditional rule = %#v", rules[0])
	}
	if got := workbook.EffectiveStyle(&model.session.Workbook.Sheets[0], workbook.Coord{Row: 1}).FillRGB; got != "CD0000" {
		t.Fatalf("matching cell fill = %q, want theme red", got)
	}
	if got := workbook.EffectiveStyle(&model.session.Workbook.Sheets[0], workbook.Coord{}).FillRGB; got != "" {
		t.Fatalf("nonmatching cell fill = %q, want unchanged", got)
	}
}

func TestConditionalDialogAddsMultipleRules(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")
	draftConditionalRule(&model, "5", false)
	if len(model.session.Workbook.Sheets[0].ConditionalRules) != 0 {
		t.Fatal("Add another rule mutated workbook before Done")
	}
	draftConditionalRule(&model, "3", true)
	pressDialogTab(&model, 5)
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rules := model.session.Workbook.Sheets[0].ConditionalRules
	if len(rules) != 2 || rules[0].Operator != workbook.GreaterThan || rules[1].Operator != workbook.LessThan || rules[1].Value != "3" {
		t.Fatalf("Done applied rules = %#v", rules)
	}
}

func TestConditionalDialogCancelLeavesWorkbookUnchanged(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")
	model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.ConditionalDialog.Open || len(model.session.Workbook.Sheets[0].ConditionalRules) != 0 {
		t.Fatal("Cancel left a dialog open or mutated the workbook")
	}
}

func TestConditionalDialogRejectsInvalidRangeWithoutMutation(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")
	model.ConditionalDialog.RangeInput.SetValue("A0:A5")
	model.ConditionalDialog.ValueInput.SetValue("5")
	model.ConditionalDialog.FillRGB = "CD0000"
	model.ConditionalDialog.Focus = 7
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !model.ConditionalDialog.Open || len(model.session.Workbook.Sheets[0].ConditionalRules) != 0 || model.ConditionalDialog.Error == "" {
		t.Fatalf("invalid range outcome: open=%t rules=%#v error=%q", model.ConditionalDialog.Open, model.session.Workbook.Sheets[0].ConditionalRules, model.ConditionalDialog.Error)
	}
}

func TestConditionalDialogFitsSmallViewport(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
	model.runCommand("conditional-format")
	view := model.View()
	for _, action := range []string{"Done", "Cancel"} {
		if !strings.Contains(view, action) {
			t.Fatalf("height-10 dialog hid the %q action: %q", action, view)
		}
	}
	if lines := strings.Split(view, "\n"); len(lines) > 10 {
		t.Fatalf("dialog rendered %d lines in height 10", len(lines))
	}
	for _, line := range strings.Split(view, "\n") {
		if width := ansi.StringWidth(line); width > 30 {
			t.Fatalf("dialog line width %d exceeds viewport: %q", width, line)
		}
	}
	model.ConditionalDialog.Mode = conditionalColorScale
	view = model.View()
	for _, action := range []string{"Done", "Cancel"} {
		if !strings.Contains(view, action) {
			t.Fatalf("height-10 color-scale dialog hid the %q action: %q", action, view)
		}
	}
	if lines := strings.Split(view, "\n"); len(lines) > 10 {
		t.Fatalf("color-scale dialog rendered %d lines in height 10", len(lines))
	}
	for _, line := range strings.Split(view, "\n") {
		if width := ansi.StringWidth(line); width > 30 {
			t.Fatalf("color-scale dialog line width %d exceeds viewport: %q", width, line)
		}
	}
}

func TestConditionalDialogSupportsTwoAndThreeColorScales(t *testing.T) {
	for _, test := range []struct {
		name      string
		selection string
		wantMid   string
	}{
		{name: "two-color", selection: "2", wantMid: ""},
		{name: "three-color", selection: "3", wantMid: "FFFF00"},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
			model.runCommand("conditional-format")
			model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			model.Update(tea.KeyMsg{Type: tea.KeyRight})
			model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(test.selection)})
			for _, expected := range []string{"Color scale", test.selection + "-color"} {
				if !strings.Contains(model.View(), expected) {
					t.Fatalf("scale tab missing %q: %s", expected, model.View())
				}
			}
			model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			pressDialogTab(&model, 4)
			model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			rules := model.session.Workbook.Sheets[0].ConditionalRules
			if model.ConditionalDialog.Open || len(rules) != 1 || rules[0].ColorScale == nil || rules[0].ColorScale.MidRGB != test.wantMid {
				t.Fatalf("scale tab rule = %#v open=%t", rules, model.ConditionalDialog.Open)
			}
		})
	}
}

func TestConditionalDialogAcceptsCustomRGBColor(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")
	pressDialogEnter(&model, 4)
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for range len(model.Theme.Options()) {
		model.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !model.ColorPicker.EditingInput {
		t.Fatal("custom color choice did not open RGB input")
	}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("#12AB34")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model.ConditionalDialog.FillRGB != "12AB34" || len(model.session.Workbook.Sheets[0].ConditionalRules) != 0 {
		t.Fatalf("custom RGB draft = %q; workbook rules=%#v", model.ConditionalDialog.FillRGB, model.session.Workbook.Sheets[0].ConditionalRules)
	}
	model.Update(tea.KeyMsg{Type: tea.KeyEsc})
}

func newConditionalModel(t *testing.T, selection workbook.CellRange) AppModel {
	t.Helper()
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	for row, value := range []string{"1", "6"} {
		if err := workbook.SetCell(session, workbook.CellAddress{Sheet: "Sheet1", Row: row, Col: 0}, value); err != nil {
			t.Fatal(err)
		}
	}
	model := NewModel(session)
	model.Grid.Selection = selection
	return model
}

func pressDialogEnter(model *AppModel, count int) {
	for range count {
		model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
}

func pressDialogTab(model *AppModel, count int) {
	for range count {
		model.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
}

func chooseThemeRed(model *AppModel) {
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func draftConditionalRule(model *AppModel, value string, lessThan bool) {
	if model.ConditionalDialog.Focus == 0 {
		pressDialogEnter(model, 2)
	}
	if lessThan {
		model.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	chooseThemeRed(model)
	pressDialogTab(model, 2)
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
