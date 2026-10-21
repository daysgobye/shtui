package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type renameSheetDialogState struct {
	Open         bool
	OriginalName string
	Input        textinput.Model
	Error        string
}

type renameSheetDialogControl struct {
	XStart int
	XEnd   int
	Y      int
	Action string
}

type renameSheetDialogLayout struct {
	View     string
	Bounds   overlayRect
	Controls []renameSheetDialogControl
}

func newRenameSheetDialog(name string) renameSheetDialogState {
	input := textinput.New()
	input.Prompt = "Sheet name: "
	input.SetValue(name)
	input.Focus()
	input.CursorEnd()
	input.Width = 32
	input.PromptStyle = lipgloss.NewStyle().Foreground(ANSIColor(8))
	input.TextStyle = lipgloss.NewStyle().Foreground(ANSIColor(7))
	input.Cursor.Style = lipgloss.NewStyle().Foreground(ANSIColor(6))
	return renameSheetDialogState{Open: true, OriginalName: name, Input: input}
}

func (dialog *renameSheetDialogState) HandleKey(message tea.KeyMsg) string {
	if !dialog.Open {
		return ""
	}
	switch message.Type {
	case tea.KeyEsc:
		return "cancel"
	case tea.KeyEnter:
		return "rename"
	default:
		previous := dialog.Input.Value()
		dialog.Input, _ = dialog.Input.Update(message)
		if dialog.Input.Value() != previous {
			dialog.Error = ""
		}
		return ""
	}
}

func (dialog *renameSheetDialogState) View(width, height int) string {
	return dialog.layout(width, height).View
}

func (dialog *renameSheetDialogState) layout(width, height int) renameSheetDialogLayout {
	width = max(width, 1)
	height = max(height, 1)
	frameWidth := min(64, max(width-2, 1))
	innerWidth := max(frameWidth-4, 1)
	dialog.Input.Width = max(innerWidth-lipgloss.Width(dialog.Input.Prompt), 1)

	accent := lipgloss.NewStyle().Foreground(ANSIColor(6)).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ANSIColor(8))
	buttonPadding := 1
	gap := "   "
	if innerWidth < 20 {
		buttonPadding = 0
		gap = " "
	}
	renameButton := lipgloss.NewStyle().Foreground(ANSIColor(0)).Background(ANSIColor(6)).Bold(true).Padding(0, buttonPadding).Render("Rename")
	cancelButton := lipgloss.NewStyle().Foreground(ANSIColor(7)).Padding(0, buttonPadding).Render("Cancel")
	buttonRow := lipgloss.JoinHorizontal(lipgloss.Center, renameButton, gap, cancelButton)

	lines := []string{accent.Render("Rename sheet"), "", dim.Render("Sheet name"), dialog.Input.View()}
	if dialog.Error != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(ANSIColor(1)).Render(truncate(dialog.Error, innerWidth)))
	}
	if innerWidth >= 40 {
		lines = append(lines, "", buttonRow, dim.Render("Ctrl+U clear · Enter rename · Esc cancel"))
	} else {
		lines = append(lines, buttonRow, dim.Render("Ctrl+U clear · Enter"), dim.Render("Esc cancel"))
	}
	popup := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ANSIColor(6)).Padding(0, 1).Width(innerWidth).Render(strings.Join(lines, "\n"))
	layout := renameSheetDialogLayout{View: popup, Bounds: overlayBounds(popup, width, height)}

	for y, line := range strings.Split(ansi.Strip(popup), "\n") {
		renameIndex, cancelIndex := strings.Index(line, "Rename"), strings.Index(line, "Cancel")
		if renameIndex < 0 || cancelIndex < 0 {
			continue
		}
		renameStart := lipgloss.Width(line[:renameIndex]) - buttonPadding
		cancelStart := lipgloss.Width(line[:cancelIndex]) - buttonPadding
		layout.Controls = append(layout.Controls,
			renameSheetDialogControl{XStart: max(renameStart, 0), XEnd: renameStart + lipgloss.Width("Rename") + 2*buttonPadding, Y: y, Action: "rename"},
			renameSheetDialogControl{XStart: max(cancelStart, 0), XEnd: cancelStart + lipgloss.Width("Cancel") + 2*buttonPadding, Y: y, Action: "cancel"},
		)
		break
	}
	return layout
}

func (dialog *renameSheetDialogState) HandleMouse(message tea.MouseMsg, width, height int) string {
	event := tea.MouseEvent(message)
	if !dialog.Open || event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return ""
	}
	layout := dialog.layout(width, height)
	if event.X < layout.Bounds.X || event.X >= layout.Bounds.X+layout.Bounds.Width || event.Y < layout.Bounds.Y || event.Y >= layout.Bounds.Y+layout.Bounds.Height {
		return ""
	}
	x, y := event.X-layout.Bounds.X, event.Y-layout.Bounds.Y
	for _, control := range layout.Controls {
		if x >= control.XStart && x < control.XEnd && y == control.Y {
			return control.Action
		}
	}
	return ""
}
