package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"omasheet/internal/workbook"
)

func TestThemePaletteAndCustomColorFallback(t *testing.T) {
	palette := NewThemePalette()
	if err := palette.AddCustom("#AABBCC"); err != nil {
		t.Fatalf("AddCustom: %v", err)
	}
	options := palette.Options()
	if len(options) != 18 {
		t.Fatalf("color option count = %d, want 18", len(options))
	}
	if !options[0].Default || options[0].Name != "Terminal default" {
		t.Fatalf("first palette option = %#v, want terminal default", options[0])
	}
	for index := range 16 {
		option := options[index+1]
		if option.Custom || option.Default || option.ANSI != uint8(index) {
			t.Fatalf("option %d = %#v; theme colors must precede custom colors", index, option)
		}
	}
	if !options[17].Custom || options[17].Color != (RGB{R: 0xAA, G: 0xBB, B: 0xCC}) {
		t.Fatalf("custom color option = %#v", options[17])
	}
	parsed, err := ParseRGBHex("#1a2Bc3")
	if err != nil || parsed != (RGB{R: 0x1A, G: 0x2B, B: 0xC3}) {
		t.Fatalf("ParseRGBHex = %#v, %v", parsed, err)
	}
	if _, err := ParseRGBHex("#12345"); err == nil {
		t.Fatal("ParseRGBHex accepted malformed RGB")
	}
	if got := NearestANSI(RGB{}); got != 0 {
		t.Fatalf("black maps to ANSI %d, want 0", got)
	}
	custom := RGB{R: 0xAA, G: 0xBB, B: 0xCC}
	if got, want := TerminalColor(custom, false), ANSIColor(NearestANSI(custom)); got != want {
		t.Fatalf("16-color fallback = %v, want nearest ANSI %v", got, want)
	}
}

func TestThemeColorResolutionQueriesOnlyWithBoundedTimeout(t *testing.T) {
	want := RGB{R: 0x12, G: 0x34, B: 0x56}
	called := false
	got := ResolveThemeColor(3, func(index uint8, timeout time.Duration) (RGB, bool) {
		called = true
		if index != 3 || timeout <= 0 || timeout > 100*time.Millisecond {
			t.Errorf("theme query args = (%d, %s)", index, timeout)
		}
		return want, true
	})
	if !called || got != want {
		t.Fatalf("resolved theme color = %#v, called=%v; want %#v", got, called, want)
	}
	if got := ResolveThemeColor(1, nil); got != standardANSIColors[1] {
		t.Fatalf("fallback ANSI color = %#v, want %#v", got, standardANSIColors[1])
	}
}

func TestAssignedThemeColorIsQueriedOnlyOnSelection(t *testing.T) {
	session := &workbook.Session{
		Format: workbook.CSV,
		Workbook: &workbook.Workbook{
			Format: workbook.CSV,
			Sheets: []workbook.Sheet{{Name: "Sheet1", Cells: make(map[workbook.Coord]workbook.Cell)}},
		},
	}
	model := NewModel(session)
	queried := false
	themeColor := RGB{R: 0x22, G: 0x44, B: 0x66}
	model.themeQuery = func(index uint8, _ time.Duration) (RGB, bool) {
		queried = true
		if index != 0 {
			t.Errorf("queried ANSI slot %d, want 0", index)
		}
		return themeColor, true
	}
	if queried {
		t.Fatal("theme queried during model creation")
	}
	model.openColorPicker(fillColorTarget)
	if queried {
		t.Fatal("theme queried while opening palette")
	}
	model.ColorPicker.Selected = 1
	model.handleColorKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !queried || session.Workbook.Sheets[0].Cells[workbook.Coord{}].Style.FillRGB != "224466" {
		t.Fatal("selected theme color was not resolved and stored on the cell")
	}
}

func TestTrueColorAndDefaultTerminalBackground(t *testing.T) {
	custom := RGB{R: 0xAA, G: 0xBB, B: 0xCC}
	if got, want := TerminalColor(custom, true), lipgloss.Color("#AABBCC"); got != want {
		t.Fatalf("truecolor output = %v, want %v", got, want)
	}
	model := NewModel(nil)
	view := model.View()
	if strings.Contains(view, "\u001b[48;") {
		t.Fatal("initial screen paints a fixed terminal background")
	}
}
