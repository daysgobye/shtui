package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type paletteCommand struct {
	ID   string
	Name string
}

type CommandPalette struct {
	Open     bool
	Query    string
	Selected int
}

var paletteCommands = []paletteCommand{
	{ID: "toggle-toolbar", Name: "Toggle toolbar"},
	{ID: "hide-toolbar", Name: "Hide toolbar"},
	{ID: "show-toolbar", Name: "Show toolbar"},
	{ID: "undo", Name: "Undo"},
	{ID: "redo", Name: "Redo"},
	{ID: "find", Name: "Find"},
	{ID: "fill-color", Name: "Fill color"},
	{ID: "font-color", Name: "Font color"},
	{ID: "insert-row", Name: "Insert row"},
	{ID: "insert-column", Name: "Insert column"},
}

func (palette *CommandPalette) Show() {
	palette.Open = true
	palette.Query = ""
	palette.Selected = 0
}

func (palette *CommandPalette) Filtered() []paletteCommand {
	if palette == nil {
		return nil
	}
	needle := strings.ToLower(strings.TrimSpace(palette.Query))
	matches := make([]paletteCommand, 0, len(paletteCommands))
	for _, command := range paletteCommands {
		if needle == "" || strings.Contains(strings.ToLower(command.Name), needle) {
			matches = append(matches, command)
		}
	}
	return matches
}

func (palette *CommandPalette) HandleKey(message tea.KeyMsg) (string, bool) {
	if palette == nil || !palette.Open {
		return "", false
	}
	matches := palette.Filtered()
	switch message.Type {
	case tea.KeyEsc:
		palette.Open = false
	case tea.KeyEnter:
		if len(matches) == 0 {
			return "", false
		}
		palette.Selected = min(max(palette.Selected, 0), len(matches)-1)
		palette.Open = false
		return matches[palette.Selected].ID, true
	case tea.KeyUp:
		if len(matches) > 0 {
			palette.Selected = max(0, palette.Selected-1)
		}
	case tea.KeyDown:
		if len(matches) > 0 {
			palette.Selected = min(len(matches)-1, palette.Selected+1)
		}
	case tea.KeyBackspace:
		runes := []rune(palette.Query)
		if len(runes) > 0 {
			palette.Query = string(runes[:len(runes)-1])
		}
		palette.Selected = 0
	case tea.KeyRunes:
		palette.Query += string(message.Runes)
		palette.Selected = 0
	}
	return "", false
}

func (palette *CommandPalette) View(width int) string {
	if palette == nil || !palette.Open {
		return ""
	}
	matches := palette.Filtered()
	var output strings.Builder
	output.WriteString("⌕  " + palette.Query + "_\n")
	for index, command := range matches {
		prefix := "  "
		if index == palette.Selected {
			prefix = "> "
		}
		output.WriteString(prefix + command.Name + "\n")
	}
	if len(matches) == 0 {
		output.WriteString("  No matching commands\n")
	}
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ANSIColor(8)).Width(max(width-4, 1))
	return style.Render(strings.TrimSuffix(output.String(), "\n"))
}
