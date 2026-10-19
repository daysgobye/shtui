package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// ComposeOverlay draws a clipped, centered popup over the existing terminal view.
// Rows outside the popup footprint are returned unchanged.
func ComposeOverlay(base, popup string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	baseRows := strings.Split(base, "\n")
	popupRows := strings.Split(popup, "\n")
	popupWidth := 0
	for _, row := range popupRows {
		popupWidth = max(popupWidth, ansi.StringWidth(row))
	}
	popupWidth = min(popupWidth, width)
	popupHeight := min(len(popupRows), height)
	if popupWidth == 0 || popupHeight == 0 {
		return base
	}

	x := (width - popupWidth) / 2
	y := (height - popupHeight) / 2
	for row := range popupHeight {
		baseIndex := y + row
		for len(baseRows) <= baseIndex {
			baseRows = append(baseRows, "")
		}
		left := ansi.Cut(baseRows[baseIndex], 0, x)
		leftWidth := ansi.StringWidth(left)
		if leftWidth < x {
			left += strings.Repeat(" ", x-leftWidth)
		}
		overlay := ansi.Cut(popupRows[row], 0, popupWidth)
		overlayWidth := ansi.StringWidth(overlay)
		if overlayWidth < popupWidth {
			overlay += strings.Repeat(" ", popupWidth-overlayWidth)
		}
		right := ansi.Cut(baseRows[baseIndex], x+popupWidth, width)
		baseRows[baseIndex] = left + overlay + right
	}
	return strings.Join(baseRows, "\n")
}
