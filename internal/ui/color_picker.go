package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (model *AppModel) handleColorPickerMouse(message tea.MouseMsg) {
	event := tea.MouseEvent(message)
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return
	}

	popup := model.colorPickerView()
	bounds := overlayBounds(popup, model.width, model.height)
	if event.X < bounds.X || event.X >= bounds.X+bounds.Width || event.Y < bounds.Y || event.Y >= bounds.Y+bounds.Height {
		model.ColorPicker.Open = false
		model.ColorPicker.EditingInput = false
		return
	}
	if model.ColorPicker.EditingInput {
		return
	}

	lines := strings.Split(popup, "\n")
	for index, option := range model.Theme.Options() {
		if row := textLineIndex(lines, option.Name); row >= 0 && event.Y == bounds.Y+row {
			model.ColorPicker.Selected = index
			model.handleColorKey(tea.KeyMsg{Type: tea.KeyEnter})
			return
		}
	}
	if row := textLineIndex(lines, "Add custom RGB"); row >= 0 && event.Y == bounds.Y+row {
		model.ColorPicker.Selected = len(model.Theme.Options())
		model.handleColorKey(tea.KeyMsg{Type: tea.KeyEnter})
	}
}

func textLineIndex(lines []string, text string) int {
	for index, line := range lines {
		if strings.Contains(line, text) {
			return index
		}
	}
	return -1
}
