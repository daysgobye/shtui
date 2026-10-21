package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var shtuiWordmarkRows = [5][5]string{
	{"█████████", "███   ███", "█████████", "███   ███", "█████████"},
	{"███      ", "███   ███", "   ███   ", "███   ███", "   ███   "},
	{"█████████", "█████████", "   ███   ", "███   ███", "   ███   "},
	{"      ███", "███   ███", "   ███   ", "███   ███", "   ███   "},
	{"█████████", "███   ███", "   ███   ", "█████████", "█████████"},
}

// SHTUIWordmarkText returns the colored uppercase name for compact terminal output.
func SHTUIWordmarkText() string {
	sh := lipgloss.NewStyle().Foreground(lipgloss.Color("#C678F2")).Bold(true)
	tui := lipgloss.NewStyle().Foreground(lipgloss.Color("#E5B85C")).Bold(true)
	return sh.Render("SH") + tui.Render("TUI")
}

// SHTUIWordmark returns the colored spreadsheet-grid wordmark for terminal output.
func SHTUIWordmark() string {
	sh := lipgloss.NewStyle().Foreground(lipgloss.Color("#C678F2")).Bold(true)
	tui := lipgloss.NewStyle().Foreground(lipgloss.Color("#E5B85C")).Bold(true)
	border := lipgloss.NewStyle().Foreground(ANSIColor(8))
	labels := strings.Repeat(" ", 10) + strings.Join([]string{"A", "B", "C", "D", "E"}, strings.Repeat(" ", 11)) + strings.Repeat(" ", 5)
	top := "   ┌" + strings.Repeat("───────────┬", 4) + "───────────┐"
	bottom := "   └" + strings.Repeat("───────────┴", 4) + "───────────┘"
	var output strings.Builder
	output.WriteString(border.Render(labels))
	output.WriteByte('\n')
	output.WriteString(border.Render(top))
	for row, cells := range shtuiWordmarkRows {
		output.WriteByte('\n')
		output.WriteByte(' ')
		output.WriteByte(byte('1' + row))
		output.WriteByte(' ')
		output.WriteString(border.Render("│"))
		for column, cell := range cells {
			output.WriteByte(' ')
			style := tui
			if column < 2 {
				style = sh
			}
			output.WriteString(style.Render(cell))
			output.WriteByte(' ')
			output.WriteString(border.Render("│"))
		}
	}
	output.WriteByte('\n')
	output.WriteString(border.Render(bottom))
	return output.String()
}
