package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"omasheet/internal/ui"
)

type quitAction uint8

const (
	quitNoAction quitAction = iota
	quitConfirm
	quitCancel
)

type quitDialog struct {
	open     bool
	selected int
}

func (dialog *quitDialog) show() {
	dialog.open = true
	dialog.selected = 1
}

func (dialog *quitDialog) handleKey(message tea.KeyMsg) quitAction {
	if dialog == nil || !dialog.open {
		return quitNoAction
	}
	switch message.Type {
	case tea.KeyEsc:
		dialog.open = false
		return quitCancel
	case tea.KeyTab, tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight:
		dialog.selected = 1 - dialog.selected
	case tea.KeyEnter:
		if dialog.selected == 0 {
			dialog.open = false
			return quitConfirm
		}
		dialog.open = false
		return quitCancel
	case tea.KeyRunes:
		switch strings.ToLower(string(message.Runes)) {
		case "y":
			dialog.open = false
			return quitConfirm
		case "n":
			dialog.open = false
			return quitCancel
		}
	}
	return quitNoAction
}

func (dialog *quitDialog) handleMouse(message tea.MouseMsg, width, height int, dirty bool) quitAction {
	if dialog == nil || !dialog.open {
		return quitNoAction
	}
	event := tea.MouseEvent(message)
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return quitNoAction
	}
	popup := dialog.View(width, height, dirty)
	rows := strings.Split(popup, "\n")
	popupWidth := 0
	for _, row := range rows {
		popupWidth = max(popupWidth, ansi.StringWidth(row))
	}
	popupWidth = min(popupWidth, width)
	popupHeight := min(len(rows), height)
	localX, localY := event.X-(width-popupWidth)/2, event.Y-(height-popupHeight)/2
	if localX < 0 || localX >= popupWidth || localY < 0 || localY >= popupHeight {
		return quitNoAction
	}
	line := ansi.Strip(rows[localY])
	for _, option := range []struct {
		label    string
		selected int
		action   quitAction
	}{{"Yep!", 0, quitConfirm}, {"Nope", 1, quitCancel}} {
		index := strings.Index(line, option.label)
		if index < 0 {
			continue
		}
		start := ansi.StringWidth(line[:index])
		end := start + lipgloss.Width(option.label)
		if localX < start || localX >= end {
			continue
		}
		dialog.selected = option.selected
		dialog.open = false
		return option.action
	}
	return quitNoAction
}

func (dialog quitDialog) View(width, height int, dirty bool) string {
	accent := lipgloss.NewStyle().Foreground(ui.ANSIColor(6)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(ui.ANSIColor(8))
	selected := lipgloss.NewStyle().Foreground(ui.ANSIColor(7)).Background(ui.ANSIColor(5)).Bold(true)
	button := lipgloss.NewStyle().Foreground(ui.ANSIColor(7))
	message := "Are you sure you want to quit?"
	if dirty {
		message = "Unsaved changes will be lost."
	}
	confirmLabel, cancelLabel := button.Render(" Yep! "), button.Render(" Nope ")
	if dialog.selected == 0 {
		confirmLabel = selected.Render(" Yep! ")
	}
	if dialog.selected == 1 {
		cancelLabel = selected.Render(" Nope ")
	}

	if height < 12 || width < 44 {
		innerWidth := max(min(48, width-4), 1)
		actions := lipgloss.NewStyle().Width(innerWidth).Align(lipgloss.Center).Render(lipgloss.JoinHorizontal(lipgloss.Top, confirmLabel, "   ", cancelLabel))
		rows := []string{accent.Render("Quit Shtui?"), ansi.Wordwrap(message, innerWidth, " "), actions}
		if width < 22 {
			rows = []string{accent.Render("Quit?"), ansi.Wordwrap(message, innerWidth, " "), confirmLabel, cancelLabel}
		}
		if height >= 7 {
			rows = append(rows, muted.Render("Y/Enter Yep · N/Esc Nope"))
		}
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ANSIColor(5)).
			Padding(0, 1).
			Width(innerWidth).
			Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	}

	innerWidth := max(min(48, width-8), 1)
	actions := lipgloss.NewStyle().Width(innerWidth).Align(lipgloss.Center).Render(lipgloss.JoinHorizontal(lipgloss.Top, confirmLabel, "   ", cancelLabel))
	rows := []string{
		accent.Render("Quit Shtui?"),
		"",
		ansi.Wordwrap(message, innerWidth, " "),
		"",
		actions,
		"",
		muted.Render("Enter choose · Y confirm · N/Esc cancel"),
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ANSIColor(5)).
		Padding(1, 2).
		Width(innerWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}
