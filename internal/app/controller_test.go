package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"omasheet/internal/ui"
	"omasheet/internal/workbook"
)

func TestPathLaunchOpensSelectedFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.csv")
	if err := os.WriteFile(path, []byte("Name,Count\npear,3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	controller := NewController(path)
	message := controller.Init()()
	controller.Update(message)
	if controller.session == nil || controller.session.Format != workbook.CSV {
		t.Fatalf("loaded session = %#v; want CSV", controller.session)
	}
	if !strings.Contains(controller.View(), "pear") {
		t.Fatal("loaded CSV is not visible in the TUI")
	}
}

func TestPathLaunchOpensXLSXFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.xlsx")
	created, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	if err := workbook.SaveFile(created, path); err != nil {
		t.Fatal(err)
	}
	if err := created.Close(); err != nil {
		t.Fatal(err)
	}
	controller := NewController(path)
	controller.Update(controller.Init()())
	if controller.session == nil || controller.session.Format != workbook.XLSX {
		t.Fatalf("loaded session = %#v; want XLSX", controller.session)
	}
	t.Cleanup(func() { _ = controller.session.Close() })
}

func TestNoPathScreenCreatesCSVWorkbook(t *testing.T) {
	controller := NewController("")
	view := controller.View()
	if !strings.Contains(view, "Open file") || !strings.Contains(view, "New CSV") {
		t.Fatalf("no-path screen lacks open/create actions: %q", view)
	}
	controller.newWorkbook(workbook.CSV)
	if controller.session == nil || controller.session.Format != workbook.CSV || len(controller.session.Workbook.Sheets) != 1 {
		t.Fatalf("new CSV session = %#v", controller.session)
	}
}

func TestNoPathShortcutCreatesWorkbookWithoutEnteringEditMode(t *testing.T) {
	controller := NewController("")
	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if controller.session == nil || controller.session.Format != workbook.CSV {
		t.Fatal("n shortcut did not create a CSV workbook")
	}
	if strings.Contains(controller.view.View(), "Edit:") {
		t.Fatalf("creating a workbook left cell editor active: %q", controller.view.View())
	}
}

func TestLoadErrorRetainsPriorSession(t *testing.T) {
	prior := &workbook.Session{Format: workbook.CSV, Workbook: &workbook.Workbook{Format: workbook.CSV, Sheets: []workbook.Sheet{{Name: "Existing", Cells: map[workbook.Coord]workbook.Cell{}}}}}
	controller := NewController("")
	controller.session = prior
	controller.view = ui.NewModel(prior)
	controller.Update(loadCompleteMsg{path: "/missing/book.csv", err: os.ErrNotExist})
	if controller.session != prior || !strings.Contains(controller.View(), "Existing") {
		t.Fatal("load error replaced the prior workbook session")
	}
}

func TestPaletteOpenCommandStartsPathPrompt(t *testing.T) {
	controller := NewController("")
	controller.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("open file")})
	controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !controller.pathPrompt || !strings.Contains(controller.View(), "Open file path") {
		t.Fatalf("command-palette Open file did not start path input: %q", controller.View())
	}
}

func TestCSVLossWarningsCoverStylesAndMultipleSheets(t *testing.T) {
	controller := NewController("")
	controller.session = &workbook.Session{
		Format: workbook.CSV,
		Workbook: &workbook.Workbook{Format: workbook.CSV, Sheets: []workbook.Sheet{
			{Name: "First", Cells: map[workbook.Coord]workbook.Cell{{Row: 0, Col: 0}: {Input: "x", Style: workbook.CellStyle{FillRGB: "FF0000"}}}},
			{Name: "Second", Cells: map[workbook.Coord]workbook.Cell{}},
		}},
	}
	controller.requestSave()
	view := controller.View()
	if !strings.Contains(view, "formatting") || !strings.Contains(view, "other sheets") {
		t.Fatalf("CSV loss warning missing features: %q", view)
	}
}

func TestFailedSaveDoesNotMarkWorkbookClean(t *testing.T) {
	session := &workbook.Session{
		Format:   workbook.CSV,
		Workbook: &workbook.Workbook{Format: workbook.CSV, Dirty: true, Sheets: []workbook.Sheet{{Name: "Sheet1", Cells: map[workbook.Coord]workbook.Cell{}}}},
	}
	controller := NewController("")
	controller.session = session
	controller.view = ui.NewModel(session)
	message := controller.saveCmd(filepath.Join(t.TempDir(), "missing", "out.csv"))()
	controller.Update(message)
	if !session.Workbook.Dirty {
		t.Fatal("failed save marked workbook clean")
	}
	if !strings.Contains(controller.View(), "failed") {
		t.Fatal("failed save did not display an error")
	}
}

func TestCtrlSavesSelectedWorkbook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.csv")
	session := &workbook.Session{
		Path:     path,
		Format:   workbook.CSV,
		Workbook: &workbook.Workbook{Format: workbook.CSV, Dirty: true, Sheets: []workbook.Sheet{{Name: "Sheet1", Rows: 1, Cols: 1, Cells: map[workbook.Coord]workbook.Cell{{Row: 0, Col: 0}: {Input: "saved"}}}}},
	}
	controller := NewController("")
	controller.session = session
	controller.view = ui.NewModel(session)
	_, command := controller.Update(teaKeyCtrlS())
	if command == nil {
		t.Fatal("Ctrl+S did not start a save")
	}
	controller.Update(command())
	if session.Workbook.Dirty {
		t.Fatal("successful save did not clear dirty state")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "saved\n" {
		t.Fatalf("saved CSV = %q, %v", got, err)
	}
	reopened, err := workbook.OpenFile(path)
	if err != nil {
		t.Fatalf("reopen saved CSV: %v", err)
	}
	defer reopened.Close()
	if got := reopened.Workbook.Sheets[0].Cells[workbook.Coord{}].Input; got != "saved" {
		t.Fatalf("reopened A1 = %q, want saved", got)
	}
}

func teaKeyCtrlS() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlS}
}
