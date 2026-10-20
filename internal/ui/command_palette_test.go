package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"omasheet/internal/workbook"
)

func TestCommandPaletteOpensWithToolbarHiddenAndSearchInvokesAction(t *testing.T) {
	model := NewModel(&workbook.Session{Workbook: &workbook.Workbook{Sheets: []workbook.Sheet{{Name: "Sheet1"}}}})
	if !model.toolbarVisible {
		t.Fatal("toolbar should be visible by default")
	}
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if !model.Palette.Open {
		t.Fatal("Ctrl+P did not open the command palette")
	}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hide toolbar")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model.toolbarVisible {
		t.Fatal("searching and invoking Hide toolbar left the toolbar visible")
	}
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if !model.Palette.Open {
		t.Fatal("Ctrl+P did not open the palette while the toolbar was hidden")
	}
}

func TestToolbarCanBeToggledFromViewAction(t *testing.T) {
	model := NewModel(&workbook.Session{Workbook: &workbook.Workbook{Sheets: []workbook.Sheet{{Name: "Sheet1", Cells: map[workbook.Coord]workbook.Cell{}}}}})
	viewX := -1
	for x := range 100 {
		if toolbarActionAtWidth(x, model.width) == "view" {
			viewX = x
			break
		}
	}
	if viewX < 0 {
		t.Fatal("toolbar has no View action")
	}
	model.handleMouse(tea.MouseMsg(tea.MouseEvent{X: viewX, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if model.toolbarVisible {
		t.Fatal("View toolbar action did not hide the toolbar")
	}
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("show toolbar")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !model.toolbarVisible {
		t.Fatal("command palette did not restore the toolbar")
	}
}
func TestPaletteSearchMatchesActionAndKeybinding(t *testing.T) {
	palette := NewCommandPalette(DefaultKeyMap())
	for _, test := range []struct {
		query    string
		wantName string
		wantKey  string
	}{
		{query: "undo", wantName: "Undo", wantKey: "Ctrl+Z"},
		{query: "Ctrl+S", wantName: "Save", wantKey: "Ctrl+S"},
	} {
		palette.Show()
		palette.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(test.query)})
		view := palette.View(80)
		if !strings.Contains(view, test.wantName) || !strings.Contains(view, test.wantKey) {
			visible := make([]string, 0)
			for _, item := range palette.list.VisibleItems() {
				visible = append(visible, item.FilterValue())
			}
			t.Errorf("search %q rendered %q, want %q with shortcut %q; filter state=%s enabled=%t value=%q visible=%q",
				test.query, view, test.wantName, test.wantKey, palette.list.FilterState(), palette.list.FilteringEnabled(), palette.list.FilterValue(), visible)
		}
	}
}
func TestCommandPaletteShowsActionsBeforeSearch(t *testing.T) {
	palette := NewCommandPalette(DefaultKeyMap())
	palette.Show()
	view := palette.View(80)
	if !strings.Contains(view, "Toggle toolbar") {
		t.Fatalf("opening palette did not show available commands before filtering: %q", view)
	}
}

func TestToolbarPrimaryActionsFitAndAreClickableAt80Columns(t *testing.T) {
	for _, action := range []struct {
		label string
		want  string
	}{
		{label: "Conditional", want: "conditional"},
		{label: "ƒx", want: "function"},
	} {
		model := newInteractionModel(t, 1)
		model.width, model.height = 80, 24
		line := strings.Split(ansi.Strip(model.View()), "\n")[0]
		if width := lipgloss.Width(line); width > 80 {
			t.Fatalf("toolbar width = %d, want at most 80: %q", width, line)
		}
		index := strings.Index(line, action.label)
		if index < 0 {
			t.Fatalf("80-column toolbar lacks %q: %q", action.label, line)
		}
		x := ansi.StringWidth(line[:index])
		model.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
		switch action.want {
		case "conditional":
			if !model.ConditionalDialog.Open {
				t.Fatal("clicking Conditional did not open its dialog")
			}
		case "function":
			if !model.FunctionPicker.Open {
				t.Fatal("clicking ƒx did not open the function picker")
			}
		}
	}
}

func TestBorderColorToolbarButtonOpensPicker(t *testing.T) {
	model := newInteractionModel(t, 1)
	model.width, model.height = 200, 24
	borderX := -1
	for x := range model.width {
		if toolbarActionAtWidth(x, model.width) == "border-color" {
			borderX = x
			break
		}
	}
	if borderX < 0 {
		t.Fatal("toolbar has no Border color action")
	}

	model.Update(tea.MouseMsg(tea.MouseEvent{
		X: borderX, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))
	if !model.ColorPicker.Open || model.ColorPicker.Target != borderColorTarget {
		t.Fatalf("clicking Border opened picker=%t target=%q, want border color picker", model.ColorPicker.Open, model.ColorPicker.Target)
	}
}

func TestToolbarOverflowRetainsCommandsAtNarrowWidths(t *testing.T) {
	model := newInteractionModel(t, 1)
	model.width, model.height = 30, 24
	line := strings.Split(ansi.Strip(model.View()), "\n")[0]
	if width := lipgloss.Width(line); width > 30 {
		t.Fatalf("narrow toolbar width = %d, want at most 30: %q", width, line)
	}
	index := strings.Index(line, "More")
	if index < 0 {
		t.Fatalf("narrow toolbar has no More entry: %q", line)
	}
	x := ansi.StringWidth(line[:index])
	model.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if !model.Palette.Open {
		t.Fatal("clicking More did not open the command palette")
	}
	for _, command := range []string{"fill-color", "insert-function"} {
		found := false
		for _, item := range model.Palette.list.Items() {
			if item, ok := item.(paletteItem); ok && item.command.ID == command {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("overflow palette does not retain command %q", command)
		}
	}

	tiny := newInteractionModel(t, 1)
	tiny.width, tiny.height = 10, 10
	tinyLine := strings.Split(ansi.Strip(tiny.View()), "\n")[0]
	if !strings.Contains(tinyLine, "More") {
		t.Fatalf("10-column toolbar lacks a usable overflow action: %q", tinyLine)
	}
}

func TestToggleToolbarDoesNotHideColumnHeaders(t *testing.T) {
	model := newInteractionModel(t, 1)
	model.width, model.height = 80, 24
	if !containsColumnHeaderRow(model.View()) {
		t.Fatal("column headers are missing before hiding the toolbar")
	}
	model.ToggleToolbar()
	view := ansi.Strip(model.View())
	if strings.Contains(strings.Split(view, "\n")[0], "File") {
		t.Fatal("toolbar remains visible after toggling it off")
	}
	if !containsColumnHeaderRow(view) {
		t.Fatalf("hiding the toolbar also hid column headers: %q", view)
	}
}

func containsColumnHeaderRow(view string) bool {
	for _, line := range strings.Split(ansi.Strip(view), "\n") {
		if strings.Contains(line, "│A") && strings.Contains(line, "│B") {
			return true
		}
	}
	return false
}
