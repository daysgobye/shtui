package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"omasheet/internal/workbook"
)

func TestComposeOverlayCentersPopupAndPreservesBase(t *testing.T) {
	base := "AAAAAAAAAAAA\nBBBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDDDD\nEEEEEEEEEEEE"
	view := ComposeOverlay(base, "XX\nYY", 12, 5)
	lines := strings.Split(view, "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d rows, want 5: %q", len(lines), view)
	}
	if lines[0] != "AAAAAAAAAAAA" || lines[3] != "DDDDDDDDDDDD" || lines[4] != "EEEEEEEEEEEE" {
		t.Fatalf("rows outside popup changed: %q", view)
	}
	if lines[1] != "BBBBBXXBBBBB" || lines[2] != "CCCCCYYCCCCC" {
		t.Fatalf("popup not centered over base: %q", view)
	}
}

func TestComposeOverlayClampsToSmallViewport(t *testing.T) {
	view := ComposeOverlay("12345678\nabcdefgh\nABCDEFGH", "popup wider\nsecond\nthird\nfourth", 8, 3)
	lines := strings.Split(view, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d rows, want 3: %q", len(lines), view)
	}
	for row, line := range lines {
		if width := lipgloss.Width(line); width > 8 {
			t.Errorf("row %d has width %d, exceeds viewport: %q", row, width, line)
		}
	}
}

func TestPaletteOverlayPreservesHeadersWhenToolbarHidden(t *testing.T) {
	model := NewModel(&workbook.Session{Workbook: &workbook.Workbook{Sheets: []workbook.Sheet{{Name: "Sheet1"}}}})
	model.toolbarVisible = false
	model.Palette.Show()

	view := model.View()
	gridHeader := model.Grid.View(model.activeSheet(), 1)
	if !strings.Contains(view, gridHeader) {
		t.Fatalf("palette replaced the live grid header\nview: %q\nheader: %q", view, gridHeader)
	}
}
