package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"shtui/internal/ui"
)

type saveRecoveryAction uint8

const (
	saveRecoveryNoAction saveRecoveryAction = iota
	saveRecoveryForce
	saveRecoveryCancel
)

type saveRecoveryDialog struct {
	open  bool
	error string
}

func (dialog *saveRecoveryDialog) show(err error) {
	dialog.open = true
	dialog.error = err.Error()
}

func (dialog *saveRecoveryDialog) handleKey(message tea.KeyMsg) saveRecoveryAction {
	if dialog == nil || !dialog.open {
		return saveRecoveryNoAction
	}
	switch message.Type {
	case tea.KeyEsc:
		dialog.open = false
		return saveRecoveryCancel
	case tea.KeyEnter:
		dialog.open = false
		return saveRecoveryCancel
	case tea.KeyRunes:
		switch strings.ToLower(string(message.Runes)) {
		case "y":
			dialog.open = false
			return saveRecoveryForce
		case "n", "q":
			dialog.open = false
			return saveRecoveryCancel
		}
	}
	return saveRecoveryNoAction
}

func (dialog *saveRecoveryDialog) handleMouse(message tea.MouseMsg, width, height int) saveRecoveryAction {
	if dialog == nil || !dialog.open {
		return saveRecoveryNoAction
	}
	event := tea.MouseEvent(message)
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return saveRecoveryNoAction
	}
	popup := dialog.View(width, height)
	rows := strings.Split(popup, "\n")
	popupWidth := 0
	for _, row := range rows {
		popupWidth = max(popupWidth, ansi.StringWidth(row))
	}
	popupWidth = min(popupWidth, width)
	popupHeight := min(len(rows), height)
	localX, localY := event.X-(width-popupWidth)/2, event.Y-(height-popupHeight)/2
	if localX < 0 || localX >= popupWidth || localY < 0 || localY >= popupHeight {
		return saveRecoveryNoAction
	}
	line := ansi.Strip(rows[localY])
	if index := strings.Index(line, "Force save"); index >= 0 {
		start := ansi.StringWidth(line[:index])
		if localX >= start && localX < start+len("Force save") {
			dialog.open = false
			return saveRecoveryForce
		}
	}
	if index := strings.Index(line, "Cancel"); index >= 0 {
		start := ansi.StringWidth(line[:index])
		if localX >= start && localX < start+len("Cancel") {
			dialog.open = false
			return saveRecoveryCancel
		}
	}
	return saveRecoveryNoAction
}

func (dialog saveRecoveryDialog) View(width, height int) string {
	innerWidth := max(min(68, width-8), 1)
	accent := lipgloss.NewStyle().Foreground(ui.ANSIColor(6)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(ui.ANSIColor(8))
	danger := lipgloss.NewStyle().Foreground(ui.ANSIColor(1)).Bold(true)
	errorText := ansi.Wordwrap(dialog.error, innerWidth, " ")
	message := "The safe save check found workbook parts that may be lost or altered (for example charts, macros, validations, or relationships). Force save replaces the file with shtui's version; Excel may remove or repair unsupported content. A backup copy named .bak is created next to the target before replacement."
	rows := []string{
		accent.Render("Workbook preservation check failed"),
		"",
		ansi.Wordwrap(message, innerWidth, " "),
		"",
		danger.Render("Save error:"),
		errorText,
		"",
		"  Force save (Y)",
		"  Cancel (N/Esc)",
		"",
		muted.Render("Default is cancel · the original is not changed unless force save succeeds"),
	}
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ANSIColor(1)).
		Padding(1, 2).
		Width(innerWidth).
		Render(content)
}
