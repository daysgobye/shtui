package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type overlayRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func overlayBounds(popup string, width, height int) overlayRect {
	if width <= 0 || height <= 0 {
		return overlayRect{}
	}
	popupRows := strings.Split(popup, "\n")
	popupWidth := 0
	for _, row := range popupRows {
		popupWidth = max(popupWidth, ansi.StringWidth(row))
	}
	popupWidth = min(popupWidth, width)
	popupHeight := min(len(popupRows), height)
	if popupWidth == 0 || popupHeight == 0 {
		return overlayRect{}
	}
	return overlayRect{
		X: (width - popupWidth) / 2, Y: (height - popupHeight) / 2,
		Width: popupWidth, Height: popupHeight,
	}
}

// ComposeOverlay draws a clipped, centered popup over the existing terminal view.
// Rows outside the popup footprint are returned unchanged.
func ComposeOverlay(base, popup string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	bounds := overlayBounds(popup, width, height)
	if bounds.Width == 0 || bounds.Height == 0 {
		return base
	}
	baseRows := strings.Split(base, "\n")
	popupRows := strings.Split(popup, "\n")
	for row := range bounds.Height {
		baseIndex := bounds.Y + row
		for len(baseRows) <= baseIndex {
			baseRows = append(baseRows, "")
		}
		left := ansi.Cut(baseRows[baseIndex], 0, bounds.X)
		leftWidth := ansi.StringWidth(left)
		if leftWidth < bounds.X {
			left += strings.Repeat(" ", bounds.X-leftWidth)
		}
		overlay := ansi.Cut(popupRows[row], 0, bounds.Width)
		overlayWidth := ansi.StringWidth(overlay)
		if overlayWidth < bounds.Width {
			overlay += strings.Repeat(" ", bounds.Width-overlayWidth)
		}
		right := ansi.Cut(baseRows[baseIndex], bounds.X+bounds.Width, width)
		baseRows[baseIndex] = left + overlay + right
	}
	return strings.Join(baseRows, "\n")
}
