package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"shtui/internal/workbook"
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
	pressDialogTab(&model, 6)
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
	model.ConditionalDialog.Focus = model.ConditionalDialog.doneFocus()
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
	for _, action := range []string{"Range:", "If", "Value:", "F:", "T:", "Done", "Cancel"} {
		if !strings.Contains(view, action) {
			t.Fatalf("height-10 dialog hid required control %q: %q", action, view)
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
	for _, action := range []string{"Range:", "Min:", "Max:", "Done", "Cancel"} {
		if !strings.Contains(view, action) {
			t.Fatalf("height-10 color-scale dialog hid required control %q: %q", action, view)
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
			model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(test.selection)})
			model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			for _, expected := range []string{"Color scale", test.selection + "-color"} {
				if !strings.Contains(model.View(), expected) {
					t.Fatalf("scale tab missing %q: %s", expected, model.View())
				}
			}
			pressDialogTab(&model, model.ConditionalDialog.doneFocus()-model.ConditionalDialog.Focus)
			model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			rules := model.session.Workbook.Sheets[0].ConditionalRules
			if model.ConditionalDialog.Open || len(rules) != 1 || rules[0].ColorScale == nil || rules[0].ColorScale.MidRGB != test.wantMid {
				t.Fatalf("scale tab rule = %#v open=%t", rules, model.ConditionalDialog.Open)
			}
		})
	}
}
func TestConditionalDialogTwoColorScaleSkipsHiddenMidpoint(t *testing.T) {
	dialog := newConditionalDialog("A1")
	dialog.Mode = conditionalColorScale
	dialog.setThreeColor(false)

	if got := dialog.focusCount(); got != 8 {
		t.Fatalf("two-color scale focus count = %d, want 8 visible controls", got)
	}
	dialog.Focus = 4
	if action := dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter}); action != "color" || dialog.ColorField != "max" {
		t.Fatalf("two-color scale maximum focus opened %q for %q", action, dialog.ColorField)
	}
}

func TestConditionalDialogTwoColorScaleSummaryOmitsMidpoint(t *testing.T) {
	dialog := newConditionalDialog("A1:A2")
	dialog.Mode = conditionalColorScale
	dialog.setThreeColor(false)
	dialog.Focus = dialog.addFocus()
	dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})

	view := ansi.Strip(dialog.View(80, 24))
	if !strings.Contains(view, "1. Color scale #CD0000 → #00CD00") {
		t.Fatalf("two-color summary omits min/max colors: %q", view)
	}
	if strings.Contains(view, "choose color") {
		t.Fatalf("two-color summary shows a nonexistent midpoint: %q", view)
	}
}

func TestConditionalDialogDoesNotDuplicateAddedColorScaleDraft(t *testing.T) {
	dialog := newConditionalDialog("A1:A2")
	dialog.Mode = conditionalColorScale
	dialog.addRule()

	_, rules, err := dialog.BuildRules()
	if err != nil {
		t.Fatalf("BuildRules: %v", err)
	}
	if len(rules) != 1 || rules[0].ColorScale == nil {
		t.Fatalf("BuildRules returned %d rules after adding one scale draft: %#v", len(rules), rules)
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

func TestNestedColorPickerMouseUpdatesDraftOnly(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.width, model.height = 80, 24
	model.runCommand("conditional-format")
	pressDialogEnter(&model, 4)
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !model.ColorPicker.Open {
		t.Fatal("conditional fill control did not open color picker")
	}
	x, y := renderedTextPosition(t, &model, "Red")

	model.Update(tea.MouseMsg(tea.MouseEvent{
		X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))

	if model.ColorPicker.Open || !model.ConditionalDialog.Open {
		t.Fatalf("nested picker state: pickerOpen=%t dialogOpen=%t, want false/true", model.ColorPicker.Open, model.ConditionalDialog.Open)
	}
	if model.ConditionalDialog.FillRGB != FormatRGBHex(ResolveThemeColor(1, nil))[1:] {
		t.Fatalf("conditional draft fill = %q, want Red theme color", model.ConditionalDialog.FillRGB)
	}
	if got := model.activeSheet().ConditionalRules; len(got) != 0 {
		t.Fatalf("nested picker applied workbook rules before Done: %#v", got)
	}
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

func TestBorderColorPickerMouseActions(t *testing.T) {
	model := newInteractionModel(t, 1)
	model.width, model.height = 80, 24
	model.Grid.Selection = workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1, Col: 1}}
	wantTheme := RGB{R: 0x12, G: 0x34, B: 0x56}
	model.themeQuery = func(uint8, time.Duration) (RGB, bool) { return wantTheme, true }

	model.runCommand("border-color")
	if !model.ColorPicker.Open || model.ColorPicker.Target != colorTarget("border") {
		t.Fatalf("Border color did not open the border picker: %#v", model.ColorPicker)
	}
	x, y := renderedTextPosition(t, &model, "Red")
	model.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	assertSelectedBorderColor(t, &model, "123456")

	model.runCommand("border-color")
	x, y = renderedTextPosition(t, &model, "Add custom RGB")
	model.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if !model.ColorPicker.EditingInput {
		t.Fatal("clicking Add custom RGB did not focus its input")
	}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("#A1B2C3")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assertSelectedBorderColor(t, &model, "A1B2C3")

	model.runCommand("border-color")
	model.Update(tea.MouseMsg(tea.MouseEvent{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if model.ColorPicker.Open {
		t.Fatal("outside click did not dismiss Border color picker")
	}
	assertSelectedBorderColor(t, &model, "A1B2C3")

	model.runCommand("border-none")
	for row := 0; row <= 1; row++ {
		for col := 0; col <= 1; col++ {
			if borders := model.activeSheet().Cells[workbook.Coord{Row: row, Col: col}].Style.Borders; borders != (workbook.BorderSet{}) {
				t.Errorf("Remove borders left cell (%d,%d) with %#v", row, col, borders)
			}
		}
	}
}

func TestBorderColorAppliesToSelectedRange(t *testing.T) {
	model := newInteractionModel(t, 1)
	model.Grid.Selection = workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1, Col: 1}}
	model.themeQuery = func(uint8, time.Duration) (RGB, bool) {
		return RGB{R: 0xAA, G: 0xBB, B: 0xCC}, true
	}
	model.runCommand("border-color")
	model.ColorPicker.Selected = 1
	model.handleColorKey(tea.KeyMsg{Type: tea.KeyEnter})
	assertSelectedBorderColor(t, &model, "AABBCC")
}
func TestBorderColorCSVNoticeOnlyAfterApply(t *testing.T) {
	model := newInteractionModel(t, 1)
	model.width, model.height = 80, 24
	model.runCommand("border-color")
	model.Update(tea.MouseMsg(tea.MouseEvent{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if model.csvNotice != "" {
		t.Fatalf("canceling border picker showed a CSV notice: %q", model.csvNotice)
	}

	model.runCommand("border-color")
	x, y := renderedTextPosition(t, &model, "Red")
	model.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if model.csvNotice == "" {
		t.Fatal("successful border color application did not show a CSV notice")
	}
}

func assertSelectedBorderColor(t *testing.T, model *AppModel, want string) {
	t.Helper()
	for row := 0; row <= 1; row++ {
		for col := 0; col <= 1; col++ {
			borders := model.activeSheet().Cells[workbook.Coord{Row: row, Col: col}].Style.Borders
			for side, border := range map[string]workbook.BorderStyle{
				"top": borders.Top, "right": borders.Right, "bottom": borders.Bottom, "left": borders.Left,
			} {
				if border.Preset != "thin" || border.ColorRGB != want {
					t.Errorf("cell (%d,%d) %s border = %#v, want thin #%s", row, col, side, border, want)
				}
			}
		}
	}
}

func TestToolbarOverflowRetainsBorderCommands(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.CSV)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	model := NewModel(session)
	model.width = 80
	line := strings.Split(ansi.Strip(model.View()), "\n")[0]
	moreIndex := strings.Index(line, "More")
	if moreIndex < 0 {
		t.Fatalf("toolbar has no More action: %q", line)
	}
	model.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: ansi.StringWidth(line[:moreIndex]), Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))
	if !model.Palette.Open {
		t.Fatal("More did not open the command palette")
	}
	for _, id := range []string{"border-color", "border-none"} {
		found := false
		for _, item := range model.Palette.list.Items() {
			if item, ok := item.(paletteItem); ok && item.command.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("overflow palette does not retain %q", id)
		}
	}
	for _, removed := range []string{"border-all", "border-outside", "border-top", "border-bottom", "border-left", "border-right"} {
		for _, item := range model.Palette.list.Items() {
			if item, ok := item.(paletteItem); ok && item.command.ID == removed {
				t.Errorf("overflow palette still exposes obsolete command %q", removed)
			}
		}
	}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("border color")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model.lastCommand != "border-color" || !model.ColorPicker.Open {
		t.Fatalf("selecting Border color left command=%q pickerOpen=%t", model.lastCommand, model.ColorPicker.Open)
	}
}

func TestPaletteIncludesCoreWorkbookOperations(t *testing.T) {
	palette := NewCommandPalette(DefaultKeyMap())
	for _, expected := range []string{"sort", "filter", "border-color", "border-none", "merge", "unmerge", "conditional-format", "toggle-bold", "add-sheet", "rename-sheet", "open-file", "save-as"} {
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
	for _, removed := range []string{"border-all", "border-outside", "border-top", "border-bottom", "border-left", "border-right"} {
		for _, item := range palette.list.Items() {
			if item, ok := item.(paletteItem); ok && item.command.ID == removed {
				t.Errorf("command palette still exposes obsolete command %q", removed)
			}
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

func TestConditionalDialogMouseSwitchesTabsAndChoosesCondition(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")

	clickDialogText(t, &model, "Color scale")
	if model.ConditionalDialog.Mode != conditionalColorScale {
		t.Fatalf("mode after clicking Color scale = %d, want color scale", model.ConditionalDialog.Mode)
	}
	clickDialogText(t, &model, "Single color")
	if model.ConditionalDialog.Mode != conditionalSingleColor {
		t.Fatalf("mode after clicking Single color = %d, want single color", model.ConditionalDialog.Mode)
	}
	clickDialogText(t, &model, "Condition:")
	if !model.ConditionalDialog.ConditionMenuOpen {
		t.Fatal("clicking condition did not open its choices")
	}
	clickDialogText(t, &model, "Less than")
	if model.ConditionalDialog.Operator != 1 || model.ConditionalDialog.ConditionMenuOpen {
		t.Fatalf("condition selection: operator=%d menuOpen=%t, want Less than and closed", model.ConditionalDialog.Operator, model.ConditionalDialog.ConditionMenuOpen)
	}
}

func TestConditionalDialogMouseFocusesRangeAndValueInputs(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")

	clickDialogText(t, &model, "Range:")
	if model.ConditionalDialog.Focus != 0 {
		t.Fatalf("range click focus = %d, want 0", model.ConditionalDialog.Focus)
	}
	clickDialogText(t, &model, "Value:")
	if model.ConditionalDialog.Focus != 3 {
		t.Fatalf("value click focus = %d, want 3", model.ConditionalDialog.Focus)
	}
}

func TestConditionalDialogMouseOpensColorControls(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		mode       int
		target     string
		colorField string
	}{
		{name: "fill", mode: conditionalSingleColor, target: "Fill color", colorField: "fill"},
		{name: "font", mode: conditionalSingleColor, target: "Font color", colorField: "font"},
		{name: "minimum", mode: conditionalColorScale, target: "Minimum:", colorField: "min"},
		{name: "midpoint", mode: conditionalColorScale, target: "Midpoint:", colorField: "mid"},
		{name: "maximum", mode: conditionalColorScale, target: "Maximum:", colorField: "max"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
			model.runCommand("conditional-format")
			model.ConditionalDialog.Mode = testCase.mode

			clickDialogText(t, &model, testCase.target)

			if !model.ColorPicker.Open || model.ConditionalDialog.ColorField != testCase.colorField {
				t.Fatalf("color click: pickerOpen=%t field=%q, want true/%q", model.ColorPicker.Open, model.ConditionalDialog.ColorField, testCase.colorField)
			}
		})
	}
}

func TestConditionalDialogMouseTogglesScaleColors(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")
	model.ConditionalDialog.Mode = conditionalColorScale

	clickDialogText(t, &model, "2-color scale")
	if model.ConditionalDialog.ThreeColor {
		t.Fatal("clicking 2-color scale left midpoint enabled")
	}
	clickDialogText(t, &model, "3-color scale")
	if !model.ConditionalDialog.ThreeColor {
		t.Fatal("clicking 3-color scale did not enable midpoint")
	}
}

func TestConditionalDialogShowsAddedRuleSummaries(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")
	model.ConditionalDialog.ValueInput.SetValue("5")
	model.ConditionalDialog.FillRGB = "CD0000"

	clickDialogText(t, &model, "+ Add another rule")

	if len(model.ConditionalDialog.Rules) != 1 || model.ConditionalDialog.Rules[0].Value != "5" {
		t.Fatalf("draft rules after mouse Add = %#v, want the completed rule", model.ConditionalDialog.Rules)
	}
	if got := model.session.Workbook.Sheets[0].ConditionalRules; len(got) != 0 {
		t.Fatalf("Add mutated workbook before Done: %#v", got)
	}
	if view := model.View(); !strings.Contains(view, "1. Greater than 5") {
		t.Fatalf("view does not summarize added draft: %q", view)
	}
}

func TestConditionalDialogMouseDoneAppliesVisibleDraftsOnce(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")
	model.ConditionalDialog.Rules = []workbook.ConditionalRule{{
		Operator: workbook.GreaterThan, Value: "3", FillRGB: "CD0000",
	}}
	model.ConditionalDialog.ValueInput.SetValue("5")
	model.ConditionalDialog.FillRGB = "00CD00"

	clickDialogText(t, &model, "Done")

	rules := model.session.Workbook.Sheets[0].ConditionalRules
	if model.ConditionalDialog.Open || len(rules) != 2 || rules[0].Value != "3" || rules[1].Value != "5" {
		t.Fatalf("Done applied rules = %#v dialogOpen=%t, want both drafts once", rules, model.ConditionalDialog.Open)
	}
}

func TestConditionalDialogMouseCancelLeavesWorkbookUnchanged(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")
	model.ConditionalDialog.ValueInput.SetValue("5")
	model.ConditionalDialog.FillRGB = "CD0000"

	clickDialogText(t, &model, "Cancel")

	if model.ConditionalDialog.Open || len(model.session.Workbook.Sheets[0].ConditionalRules) != 0 {
		t.Fatalf("Cancel left dialog open or changed rules: open=%t rules=%#v", model.ConditionalDialog.Open, model.session.Workbook.Sheets[0].ConditionalRules)
	}
}

func TestConditionalDialogMouseOutsideDoesNotApplyDraft(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.width, model.height = 80, 24
	model.runCommand("conditional-format")
	model.ConditionalDialog.ValueInput.SetValue("5")
	model.ConditionalDialog.FillRGB = "CD0000"

	model.Update(tea.MouseMsg(tea.MouseEvent{
		X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))

	if !model.ConditionalDialog.Open || len(model.session.Workbook.Sheets[0].ConditionalRules) != 0 {
		t.Fatalf("outside click: dialogOpen=%t rules=%#v, want open and unchanged", model.ConditionalDialog.Open, model.session.Workbook.Sheets[0].ConditionalRules)
	}
}

func clickDialogText(t *testing.T, model *AppModel, target string) {
	t.Helper()
	view := model.View()
	for y, line := range strings.Split(view, "\n") {
		if index := strings.Index(line, target); index >= 0 {
			x := ansi.StringWidth(line[:index])
			model.Update(tea.MouseMsg(tea.MouseEvent{
				X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
			}))
			return
		}
	}
	t.Fatalf("rendered dialog has no %q: %q", target, view)
}

func TestCSVFeatureNoticeAppearsAfterSuccessfulMutation(t *testing.T) {
	mutations := []struct {
		name string
		run  func(*AppModel)
	}{
		{
			name: "sheet addition",
			run:  func(model *AppModel) { model.runCommand("add-sheet") },
		},
		{
			name: "column resize",
			run: func(model *AppModel) {
				model.layout()
				sheet := model.activeSheet()
				x := model.Grid.OriginX + model.Grid.RowHeaderWidth + model.Grid.columnWidth(0, sheet)
				y := model.Grid.OriginY
				model.handleMouse(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
				model.handleMouse(tea.MouseMsg(tea.MouseEvent{X: x + 2, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft}))
				model.handleMouse(tea.MouseMsg(tea.MouseEvent{X: x + 2, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft}))
			},
		},
		{
			name: "direct formatting",
			run: func(model *AppModel) {
				model.applyCellStyle(workbook.CellStyle{FillRGB: "112233"})
			},
		},
		{
			name: "border formatting",
			run: func(model *AppModel) {
				model.runCommand("border-color")
				model.ColorPicker.Selected = 1
				model.handleColorKey(tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
		{
			name: "merge",
			run: func(model *AppModel) {
				model.Grid.Selection = workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Col: 1}}
				model.runCommand("merge")
			},
		},
		{
			name: "conditional formatting",
			run: func(model *AppModel) {
				model.ConditionalDialog = newConditionalDialog("A1")
				model.ConditionalDialog.ValueInput.SetValue("5")
				model.ConditionalDialog.FillRGB = "CD0000"
				model.handleConditionalDialogAction("done")
			},
		},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			model := newCSVFeatureNoticeModel(t)
			mutation.run(model)
			if view := ansi.Strip(model.View()); !strings.Contains(view, "CSV can't keep workbook features") {
				t.Fatalf("successful %s did not show the CSV compatibility notice: %q", mutation.name, view)
			}
		})
	}
}

func TestCSVFeatureNoticeDoesNotAppearForFailedMutationOrXLSXSession(t *testing.T) {
	t.Run("failed conditional rule", func(t *testing.T) {
		model := newCSVFeatureNoticeModel(t)
		model.ConditionalDialog = newConditionalDialog("A0")
		model.ConditionalDialog.ValueInput.SetValue("5")
		model.ConditionalDialog.FillRGB = "CD0000"
		model.handleConditionalDialogAction("done")
		if view := ansi.Strip(model.View()); strings.Contains(view, "CSV can't keep workbook features") {
			t.Fatalf("failed mutation showed a CSV compatibility notice: %q", view)
		}
		if !model.ConditionalDialog.Open || model.ConditionalDialog.Error == "" {
			t.Fatalf("failed conditional rule did not remain open with an error: %#v", model.ConditionalDialog)
		}
	})
	t.Run("XLSX session", func(t *testing.T) {
		session, err := workbook.NewWorkbook(workbook.XLSX)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		model := NewModel(session)
		model.runCommand("add-sheet")
		if view := ansi.Strip(model.View()); strings.Contains(view, "CSV can't keep workbook features") {
			t.Fatalf("XLSX mutation showed a CSV compatibility notice: %q", view)
		}
	})
}

func TestCSVFeatureNoticeIsThemeAwareAndBelowBlockingOverlay(t *testing.T) {
	model := newCSVFeatureNoticeModel(t)
	model.runCommand("add-sheet")
	model.ConditionalDialog = newConditionalDialog("A1")

	view := model.View()
	if !strings.Contains(ansi.Strip(view), "CSV can't keep workbook features") {
		t.Fatalf("theme-aware notice is missing: %q", view)
	}
	if !strings.Contains(ansi.Strip(view), "Conditional formatting") {
		t.Fatalf("notice obscured the blocking conditional-format dialog: %q", view)
	}
}

func newCSVFeatureNoticeModel(t *testing.T) *AppModel {
	t.Helper()
	session, err := workbook.NewWorkbook(workbook.CSV)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	model := NewModel(session)
	model.width, model.height = 80, 24
	model.layout()
	return &model
}

func TestCSVFeatureNoticeExpiryDoesNotDismissNewerNotice(t *testing.T) {
	model := newCSVFeatureNoticeModel(t)
	model.runCommand("add-sheet")
	_, timer := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if timer == nil {
		t.Fatal("successful CSV feature did not schedule a notice expiry")
	}
	olderGeneration := model.csvNoticeGeneration

	model.showCSVFeatureNotice()
	newerGeneration := model.csvNoticeGeneration
	if newerGeneration <= olderGeneration {
		t.Fatalf("notice generation did not advance: older=%d newer=%d", olderGeneration, newerGeneration)
	}

	model.Update(csvNoticeExpiredMsg{generation: olderGeneration})
	if view := ansi.Strip(model.View()); !strings.Contains(view, "CSV can't keep workbook features") {
		t.Fatalf("older expiry dismissed a newer notice: %q", view)
	}
	model.Update(csvNoticeExpiredMsg{generation: newerGeneration})
	if view := ansi.Strip(model.View()); strings.Contains(view, "CSV can't keep workbook features") {
		t.Fatalf("current expiry did not dismiss its notice: %q", view)
	}
}

func TestConditionalDialogTabAndShiftTabVisitEveryControl(t *testing.T) {
	for _, testCase := range []struct {
		name string
		mode int
	}{
		{name: "single color", mode: conditionalSingleColor},
		{name: "color scale", mode: conditionalColorScale},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			dialog := newConditionalDialog("A1")
			dialog.Mode = testCase.mode
			baseFocusCount := dialog.focusCount()
			dialog.ValueInput.SetValue("5")
			dialog.FillRGB = "CD0000"
			dialog.Focus = dialog.addFocus()
			dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
			if len(dialog.Rules) != 1 {
				t.Fatalf("keyboard Add rule created %d drafts, want 1", len(dialog.Rules))
			}
			if got := dialog.focusCount(); got != baseFocusCount+1 {
				t.Fatalf("focus count with removable draft = %d, want %d", got, baseFocusCount+1)
			}

			count := dialog.focusCount()
			dialog.Focus = 0
			seen := make(map[int]bool, count)
			for range count {
				if seen[dialog.Focus] {
					t.Fatalf("Tab revisited focus %d before covering all controls", dialog.Focus)
				}
				seen[dialog.Focus] = true
				dialog.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
			}
			if dialog.Focus != 0 || len(seen) != count {
				t.Fatalf("Tab traversal ended at %d after visiting %d of %d controls", dialog.Focus, len(seen), count)
			}
			for range count {
				before := dialog.Focus
				dialog.HandleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
				want := (before - 1 + count) % count
				if dialog.Focus != want {
					t.Fatalf("Shift+Tab moved from %d to %d, want %d", before, dialog.Focus, want)
				}
			}

			dialog.Focus = dialog.focusCount() - 3
			dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
			if len(dialog.Rules) != 0 {
				t.Fatalf("keyboard Remove last rule left drafts %#v", dialog.Rules)
			}
		})
	}
}

func TestConditionalDialogKeyboardOpensColorPickerAndCompletesRule(t *testing.T) {
	model := newConditionalModel(t, workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: 1}})
	model.runCommand("conditional-format")
	pressDialogTab(&model, 4)
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !model.ColorPicker.Open || model.ConditionalDialog.ColorField != "fill" {
		t.Fatalf("Enter on Fill color did not open the color picker: picker=%t field=%q", model.ColorPicker.Open, model.ConditionalDialog.ColorField)
	}
	chooseThemeRed(&model)
	model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	pressDialogTab(&model, 4)
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	rules := model.session.Workbook.Sheets[0].ConditionalRules
	if model.ConditionalDialog.Open || len(rules) != 1 || rules[0].Value != "5" || rules[0].FillRGB != "CD0000" {
		t.Fatalf("keyboard rule completion: dialogOpen=%t rules=%#v", model.ConditionalDialog.Open, rules)
	}
}

func TestConditionalDialogShowsVisibleFocusAndGroupedSections(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	dialog := newConditionalDialog("A1")
	dialog.Focus = 4
	view := dialog.View(80, 24)
	plain := ansi.Strip(view)
	for _, heading := range []string{"Apply to range", "Rule type and criteria", "Formatting style", "Current draft rules"} {
		if !strings.Contains(plain, heading) {
			t.Errorf("dialog is missing grouped section %q: %q", heading, plain)
		}
	}
	focusedControl := lipgloss.NewStyle().Reverse(true).Bold(true).Render("Fill color: choose color   click to choose")
	if !strings.Contains(view, focusedControl) {
		t.Fatalf("focused Fill color control has no visible emphasis: %q", view)
	}
}
