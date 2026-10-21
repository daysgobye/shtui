package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSHTUIWordmarkUsesSpreadsheetGrid(t *testing.T) {
	lines := strings.Split(ansi.Strip(SHTUIWordmark()), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSpace(line)
	}
	got := strings.Join(lines, "\n")
	want := strings.Join([]string{
		"A           B           C           D           E",
		"┌───────────┬───────────┬───────────┬───────────┬───────────┐",
		"1 │ █████████ │ ███   ███ │ █████████ │ ███   ███ │ █████████ │",
		"2 │ ███       │ ███   ███ │    ███    │ ███   ███ │    ███    │",
		"3 │ █████████ │ █████████ │    ███    │ ███   ███ │    ███    │",
		"4 │       ███ │ ███   ███ │    ███    │ ███   ███ │    ███    │",
		"5 │ █████████ │ ███   ███ │    ███    │ █████████ │ █████████ │",
		"└───────────┴───────────┴───────────┴───────────┴───────────┘",
	}, "\n")
	if got != want {
		t.Fatalf("large wordmark =\n%s\nwant\n%s", got, want)
	}
}

func TestHomeScreenUsesCompactWordmarkWhenGridDoesNotFit(t *testing.T) {
	screen := newHomeScreen()
	view := ansi.Strip(screen.View(63, 24))
	if !strings.Contains(view, "SHTUI") {
		t.Fatalf("compact home screen lacks SHTUI wordmark:\n%s", view)
	}
	if strings.Contains(view, "A           B") {
		t.Fatalf("compact home screen rendered oversized grid wordmark:\n%s", view)
	}
}
