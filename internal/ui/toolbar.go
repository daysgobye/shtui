package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type toolbarItem struct {
	ID    string
	Label string
}

var toolbarItems = []toolbarItem{
	{ID: "file", Label: "File"},
	{ID: "edit", Label: "Edit"},
	{ID: "view", Label: "View"},
	{ID: "data", Label: "Data"},
	{ID: "format", Label: "Format"},
	{ID: "undo", Label: "Undo"},
	{ID: "redo", Label: "Redo"},
	{ID: "insert-row", Label: "+ Row"},
	{ID: "insert-column", Label: "+ Col"},
	{ID: "fill-color", Label: "Fill"},
	{ID: "font-color", Label: "Font"},
	{ID: "sort", Label: "Sort"},
	{ID: "filter", Label: "Filter"},
}

func (model *AppModel) ToggleToolbar() {
	if model != nil {
		model.toolbarVisible = !model.toolbarVisible
	}
}

func (model *AppModel) toolbarView() string {
	if model == nil || !model.toolbarVisible {
		return ""
	}
	var output strings.Builder
	style := lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6))
	for index, item := range toolbarItems {
		if index > 0 {
			output.WriteByte(' ')
		}
		output.WriteString(style.Render(item.Label))
	}
	return output.String()
}

func toolbarActionAt(x int) string {
	position := 0
	for index, item := range toolbarItems {
		width := lipgloss.Width(item.Label)
		if x >= position && x < position+width {
			return item.ID
		}
		position += width
		if index < len(toolbarItems)-1 {
			position++
		}
	}
	return ""
}
