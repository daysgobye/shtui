package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"omasheet/internal/ui"
)

type csvSaveAction string

const (
	csvSaveAsCSV  csvSaveAction = "csv"
	csvSaveAsXLSX csvSaveAction = "xlsx"
	csvSaveCancel csvSaveAction = "cancel"
)

type csvSaveDialog struct {
	open     bool
	selected int
	warnings []string
}

var csvSaveChoices = []struct {
	label  string
	detail string
	action csvSaveAction
}{
	{label: "Save as CSV", detail: "loses formatting and other sheets", action: csvSaveAsCSV},
	{label: "Save as XLSX", detail: "keeps workbook features", action: csvSaveAsXLSX},
	{label: "Cancel", action: csvSaveCancel},
}

func (dialog *csvSaveDialog) openWithWarnings(warnings []string) {
	dialog.open = true
	dialog.selected = 1
	dialog.warnings = append(dialog.warnings[:0], warnings...)
}

func (dialog *csvSaveDialog) handleKey(message tea.KeyMsg) csvSaveAction {
	if !dialog.open {
		return ""
	}
	switch message.Type {
	case tea.KeyEsc:
		return csvSaveCancel
	case tea.KeyUp:
		dialog.selected = max(0, dialog.selected-1)
	case tea.KeyDown:
		dialog.selected = min(len(csvSaveChoices)-1, dialog.selected+1)
	case tea.KeyEnter:
		return csvSaveChoices[dialog.selected].action
	case tea.KeyRunes:
		switch strings.ToLower(string(message.Runes)) {
		case "c":
			return csvSaveAsCSV
		case "x":
			return csvSaveAsXLSX
		case "q":
			return csvSaveCancel
		}
	}
	return ""
}

func (dialog *csvSaveDialog) handleMouse(message tea.MouseMsg, width, height int) csvSaveAction {
	if !dialog.open {
		return ""
	}
	event := tea.MouseEvent(message)
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return ""
	}
	popup := dialog.View(width, height)
	rows := strings.Split(popup, "\n")
	popupWidth := 0
	for _, row := range rows {
		popupWidth = max(popupWidth, ansi.StringWidth(row))
	}
	popupWidth = min(popupWidth, width)
	popupHeight := min(len(rows), height)
	originX := (width - popupWidth) / 2
	originY := (height - popupHeight) / 2
	localX, localY := event.X-originX, event.Y-originY
	if localX < 0 || localX >= popupWidth || localY < 0 || localY >= popupHeight {
		return csvSaveCancel
	}
	for index, choice := range csvSaveChoices {
		for rowIndex, row := range rows {
			if strings.Contains(ansi.Strip(row), choice.label) && rowIndex == localY {
				dialog.selected = index
				return choice.action
			}
		}
	}
	return ""
}

func (dialog csvSaveDialog) View(width, height int) string {
	innerWidth := max(min(58, width-8), 1)
	accent := lipgloss.NewStyle().Foreground(ui.ANSIColor(6)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(ui.ANSIColor(8))
	selected := lipgloss.NewStyle().Foreground(ui.ANSIColor(7)).Background(ui.ANSIColor(6)).Bold(true)
	warning := strings.Join(dialog.warnings, "; ")
	warning = ansi.Wordwrap(warning, innerWidth, " ")
	rows := []string{
		accent.Render("Save workbook"),
		"",
		"CSV cannot preserve:",
		warning,
		"",
		"Choose how to save:",
	}
	for index, choice := range csvSaveChoices {
		label := "  " + choice.label
		if choice.detail != "" {
			label += " — " + choice.detail
		}
		if index == dialog.selected {
			label = selected.Render("› " + choice.label)
		} else {
			label = muted.Render(label)
		}
		rows = append(rows, label)
	}
	rows = append(rows, "", muted.Render("↑/↓ choose · Enter select · C CSV · X XLSX · Esc cancel"))
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ANSIColor(6)).
		Padding(1, 2).
		Width(innerWidth).
		Render(content)
}
