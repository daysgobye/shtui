package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type toolbarItem struct {
	ID    string
	Label string
}

type toolbarItemPlacement struct {
	action      string
	label       string
	start       int
	end         int
	groupBefore bool
}

var toolbarItems = []toolbarItem{
	{ID: "file", Label: "File"},
	{ID: "edit", Label: "Edit"},
	{ID: "view", Label: "View"},
	{ID: "data", Label: "Data"},
	{ID: "format", Label: "Format"},
	{ID: "conditional-format", Label: "Conditional"},
	{ID: "insert-function", Label: "ƒx"},
	{ID: "undo", Label: "Undo"},
	{ID: "redo", Label: "Redo"},
	{ID: "insert-row", Label: "+ Row"},
	{ID: "insert-column", Label: "+ Col"},
	{ID: "fill-color", Label: "Fill"},
	{ID: "font-color", Label: "Font"},
	{ID: "border-menu", Label: "Borders"},
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
	for index, item := range layoutToolbar(model.width) {
		if index > 0 {
			if item.groupBefore {
				output.WriteString(lipgloss.NewStyle().Foreground(ANSIColor(8)).Render("  │  "))
			} else {
				output.WriteString("  ")
			}
		}
		output.WriteString(toolbarItemStyle().Render(item.label))
	}
	return output.String()
}

func toolbarItemStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6)).Padding(0, 1)
}

func toolbarGroupBoundary(index int) bool {
	switch index {
	case 3, 7, 9, 11, 14:
		return true
	default:
		return false
	}
}

func toolbarGapWidth(groupBefore bool) int {
	if groupBefore {
		return lipgloss.Width("  │  ")
	}
	return 2
}

func layoutToolbar(width int) []toolbarItemPlacement {
	if width <= 0 {
		return nil
	}
	placements := make([]toolbarItemPlacement, 0, len(toolbarItems)+1)
	position := 0
	moreWidth := lipgloss.Width(toolbarItemStyle().Render("More"))
	for index, item := range toolbarItems {
		groupBefore := index > 0 && toolbarGroupBoundary(index)
		start := position
		if len(placements) > 0 {
			start += toolbarGapWidth(groupBefore)
		}
		end := start + lipgloss.Width(toolbarItemStyle().Render(item.Label))
		requiredEnd := end
		if index < len(toolbarItems)-1 {
			requiredEnd += toolbarGapWidth(true) + moreWidth
		}
		if requiredEnd > width {
			moreStart := position
			if len(placements) > 0 {
				moreStart += toolbarGapWidth(true)
			}
			for len(placements) > 0 && moreStart+moreWidth > width {
				placements = placements[:len(placements)-1]
				position = 0
				if len(placements) > 0 {
					position = placements[len(placements)-1].end
					moreStart = position + toolbarGapWidth(true)
				} else {
					moreStart = 0
				}
			}
			if moreStart+moreWidth <= width {
				placements = append(placements, toolbarItemPlacement{
					action: "more", label: "More", start: moreStart, end: moreStart + moreWidth, groupBefore: true,
				})
			}
			break
		}
		placements = append(placements, toolbarItemPlacement{
			action: item.ID, label: item.Label, start: start, end: end, groupBefore: groupBefore,
		})
		position = end
	}
	return placements
}

func toolbarActionAtWidth(x, width int) string {
	for _, item := range layoutToolbar(width) {
		if x >= item.start && x < item.end {
			return item.action
		}
	}
	return ""
}
