package ui

import (
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
