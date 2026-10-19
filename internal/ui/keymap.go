package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// KeyMap is the single source for input matching and displayed shortcuts.
type KeyMap struct {
	OpenPalette   key.Binding
	OpenFile      key.Binding
	Save          key.Binding
	Undo          key.Binding
	Redo          key.Binding
	Find          key.Binding
	ToggleToolbar key.Binding
	PreviousSheet key.Binding
	NextSheet     key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		OpenPalette:   key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("Ctrl+P", "commands")),
		OpenFile:      key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("Ctrl+O", "open")),
		Save:          key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("Ctrl+S", "save")),
		Undo:          key.NewBinding(key.WithKeys("ctrl+z"), key.WithHelp("Ctrl+Z", "undo")),
		Redo:          key.NewBinding(key.WithKeys("ctrl+y"), key.WithHelp("Ctrl+Y", "redo")),
		Find:          key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("Ctrl+F", "find")),
		ToggleToolbar: key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("Ctrl+T", "toggle toolbar")),
		PreviousSheet: key.NewBinding(key.WithKeys("ctrl+pageup"), key.WithHelp("Ctrl+PgUp", "previous sheet")),
		NextSheet:     key.NewBinding(key.WithKeys("ctrl+pagedown"), key.WithHelp("Ctrl+PgDn", "next sheet")),
	}
}

func (keys KeyMap) Matches(message tea.KeyMsg, binding key.Binding) bool {
	pressed := strings.ToLower(message.String())
	for _, candidate := range binding.Keys() {
		if pressed == strings.ToLower(candidate) {
			return true
		}
	}
	return false
}

func (keys KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{keys.OpenPalette, keys.Save, keys.Undo, keys.Redo, keys.Find}
}

func (keys KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keys.OpenPalette, keys.OpenFile, keys.Save, keys.Undo, keys.Redo},
		{keys.Find, keys.ToggleToolbar, keys.PreviousSheet, keys.NextSheet},
	}
}

func (keys KeyMap) Shortcut(commandID string) string {
	var binding key.Binding
	switch commandID {
	case "open-file":
		binding = keys.OpenFile
	case "save":
		binding = keys.Save
	case "undo":
		binding = keys.Undo
	case "redo":
		binding = keys.Redo
	case "find":
		binding = keys.Find
	case "toggle-toolbar":
		binding = keys.ToggleToolbar
	case "previous-sheet":
		binding = keys.PreviousSheet
	case "next-sheet":
		binding = keys.NextSheet
	default:
		return ""
	}
	help := binding.Help()
	return help.Key
}
