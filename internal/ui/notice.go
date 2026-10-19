package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"omasheet/internal/workbook"
)

type csvNoticeExpiredMsg struct {
	generation uint64
}

func (model *AppModel) showCSVFeatureNotice() tea.Cmd {
	if model == nil || model.session == nil || model.session.Format != workbook.CSV || !model.hasCSVIncompatibleFeatures() {
		return nil
	}
	model.csvNoticeGeneration++
	generation := model.csvNoticeGeneration
	model.csvNotice = "CSV can't keep workbook features; save as XLSX to preserve them"
	command := tea.Tick(2500*time.Millisecond, func(time.Time) tea.Msg {
		return csvNoticeExpiredMsg{generation: generation}
	})
	model.pendingCSVNoticeCmd = command
	return command
}

func (model *AppModel) hasCSVIncompatibleFeatures() bool {
	if model == nil || model.session == nil || model.session.Workbook == nil {
		return false
	}
	book := model.session.Workbook
	if len(book.Sheets) > 1 {
		return true
	}
	for _, sheet := range book.Sheets {
		if len(sheet.ColumnWidths) > 0 || len(sheet.Merges) > 0 || len(sheet.ConditionalRules) > 0 {
			return true
		}
		for _, cell := range sheet.Cells {
			if cell.Style != (workbook.CellStyle{}) {
				return true
			}
		}
	}
	return false
}

func (model *AppModel) takeCSVNoticeCmd() tea.Cmd {
	command := model.pendingCSVNoticeCmd
	model.pendingCSVNoticeCmd = nil
	return command
}

func renderCSVFeatureNotice(text string, width, height int) string {
	if width <= 0 || height <= 0 || text == "" {
		return ""
	}
	innerWidth := max(width-6, 1)
	text = ansi.Wordwrap(text, innerWidth, " ")
	return lipgloss.NewStyle().
		Foreground(ANSIColor(7)).
		Background(ANSIColor(0)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ANSIColor(6)).
		Padding(0, 1).
		Render(text)
}

func composeCornerNotice(base, popup string, width, height int) string {
	if width <= 0 || height <= 0 || popup == "" {
		return base
	}
	popupRows := strings.Split(popup, "\n")
	popupWidth := 0
	for _, row := range popupRows {
		popupWidth = max(popupWidth, ansi.StringWidth(row))
	}
	popupWidth = min(popupWidth, width)
	popupHeight := min(len(popupRows), height)
	originX := width - popupWidth
	originY := height - popupHeight
	baseRows := strings.Split(base, "\n")
	for len(baseRows) < height {
		baseRows = append(baseRows, "")
	}
	for row := range popupHeight {
		baseIndex := originY + row
		left := ansi.Cut(baseRows[baseIndex], 0, originX)
		leftWidth := ansi.StringWidth(left)
		if leftWidth < originX {
			left += strings.Repeat(" ", originX-leftWidth)
		}
		overlay := ansi.Cut(popupRows[row], 0, popupWidth)
		overlayWidth := ansi.StringWidth(overlay)
		if overlayWidth < popupWidth {
			overlay += strings.Repeat(" ", popupWidth-overlayWidth)
		}
		right := ansi.Cut(baseRows[baseIndex], originX+popupWidth, width)
		baseRows[baseIndex] = left + overlay + right
	}
	return strings.Join(baseRows, "\n")
}
