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
	BeginEdit     key.Binding
	CommitDown    key.Binding
	CommitRight   key.Binding
	CancelEdit    key.Binding
	MoveUp        key.Binding
	MoveDown      key.Binding
	MoveLeft      key.Binding
	MoveRight     key.Binding
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
		PreviousSheet: key.NewBinding(key.WithKeys("ctrl+pgup", "ctrl+pageup"), key.WithHelp("Ctrl+PgUp", "previous sheet")),
		NextSheet:     key.NewBinding(key.WithKeys("ctrl+pgdown", "ctrl+pagedown"), key.WithHelp("Ctrl+PgDn", "next sheet")),
		BeginEdit:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "edit cell")),
		CommitDown:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "commit and move down")),
		CommitRight:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "commit and move right")),
		CancelEdit:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "cancel edit")),
		MoveUp:        key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "move up")),
		MoveDown:      key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "move down")),
		MoveLeft:      key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "move left")),
		MoveRight:     key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "move right")),
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
		{keys.BeginEdit, keys.CommitDown, keys.CommitRight, keys.CancelEdit},
		{keys.MoveUp, keys.MoveDown, keys.MoveLeft, keys.MoveRight},
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
	case "edit-cell":
		binding = keys.BeginEdit
	case "commit-down":
		binding = keys.CommitDown
	case "commit-right":
		binding = keys.CommitRight
	case "cancel-edit":
		binding = keys.CancelEdit
	case "move-up":
		binding = keys.MoveUp
	case "move-down":
		binding = keys.MoveDown
	case "move-left":
		binding = keys.MoveLeft
	case "move-right":
		binding = keys.MoveRight
	default:
		return ""
	}
	help := binding.Help()
	return help.Key
}
