package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	model := NewModel(nil)
	viewX := -1
	for x := range 100 {
		if toolbarActionAt(x) == "view" {
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
