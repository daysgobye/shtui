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

func TestFailedOpenWithoutSessionShowsErrorOnHomeScreen(t *testing.T) {
	controller := NewController("")
	controller.Update(loadCompleteMsg{path: "/missing/book.csv", err: os.ErrNotExist})
	view := controller.View()
	if !strings.Contains(view, "book.csv") || !strings.Contains(view, "does not exist") {
		t.Fatalf("home screen hid the open error: %q", view)
	}
}

func TestPaletteOpenCommandStartsPicker(t *testing.T) {
	controller := NewController("")
	controller.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("open file")})
	_, command := controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil || !strings.Contains(controller.View(), "Open workbook") {
		t.Fatalf("command-palette Open file did not start the picker: %q", controller.View())
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
func TestOpenPickerFiltersCSVAndXLSX(t *testing.T) {
	directory := t.TempDir()
	for name, data := range map[string][]byte{
		"a.csv":  []byte("value\n"),
		"b.xlsx": []byte("not opened in this test"),
		"c.txt":  []byte("unsupported"),
	} {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	controller := NewController("")
	openPicker(t, controller, directory)
	view := controller.View()
	if !strings.Contains(view, "a.csv") || !strings.Contains(view, "b.xlsx") {
		t.Fatalf("picker did not list supported files: %q", view)
	}

	controller.Update(tea.KeyMsg{Type: tea.KeyDown})
	controller.Update(tea.KeyMsg{Type: tea.KeyDown})
	controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if controller.loading || !strings.Contains(strings.ToLower(controller.View()), "supported") {
		t.Fatalf("unsupported file reached loading path without warning: %q", controller.View())
	}

	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if _, command := controller.Update(tea.KeyMsg{Type: tea.KeyEnter}); command == nil {
		t.Fatal("CSV selection did not start asynchronous loading")
	}

	xlsxController := NewController("")
	openPicker(t, xlsxController, directory)
	xlsxController.Update(tea.KeyMsg{Type: tea.KeyDown})
	if _, command := xlsxController.Update(tea.KeyMsg{Type: tea.KeyEnter}); command == nil {
		t.Fatal("XLSX selection did not start asynchronous loading")
	}
}

func TestOpenPickerEscapePreservesWorkbook(t *testing.T) {
	session := &workbook.Session{
		Path:   "existing.csv",
		Format: workbook.CSV,
		Workbook: &workbook.Workbook{
			Format: workbook.CSV, Dirty: true,
			Sheets: []workbook.Sheet{{Name: "Existing", Cells: map[workbook.Coord]workbook.Cell{{Row: 0, Col: 0}: {Input: "keep"}}}},
		},
	}
	controller := NewController("")
	controller.session = session
	controller.path = session.Path
	controller.view = ui.NewModel(session)
	openPicker(t, controller, t.TempDir())

	controller.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if controller.session != session || controller.path != session.Path || !session.Workbook.Dirty {
		t.Fatal("canceling the picker changed the current workbook or path")
	}
	if !strings.Contains(controller.View(), "keep") {
		t.Fatalf("canceling the picker did not restore the workbook view: %q", controller.View())
	}
}

func TestOpenPickerUsesExistingAsyncLoadPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.csv")
	if err := os.WriteFile(path, []byte("value\nfrom-picker\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	controller := NewController("")
	openPicker(t, controller, filepath.Dir(path))

	_, command := controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("selecting a supported file did not return the async load command")
	}
	message := command()
	controller.Update(message)
	if controller.session == nil || controller.path != path {
		t.Fatalf("async load did not publish selected workbook: session=%#v path=%q", controller.session, controller.path)
	}
	if got := controller.session.Workbook.Sheets[0].Cells[workbook.Coord{Row: 1}].Input; got != "from-picker" {
		t.Fatalf("loaded A2 = %q, want from-picker", got)
	}
}

func TestOpenPickerFitsSmallTerminal(t *testing.T) {
	controller := NewController("")
	openPicker(t, controller, t.TempDir())
	controller.Update(tea.WindowSizeMsg{Width: 30, Height: 8})

	view := controller.View()
	if rows := strings.Count(view, "\n") + 1; rows > 8 {
		t.Fatalf("picker popup rendered %d rows in an 8-row terminal: %q", rows, view)
	}
	if !strings.Contains(view, "Open workbook") || !strings.Contains(view, "Esc") {
		t.Fatalf("small-terminal view lost picker controls: %q", view)
	}
}

func TestSaveAsPickerKeepsCSVLossWarning(t *testing.T) {
	session := &workbook.Session{
		Format: workbook.CSV,
		Workbook: &workbook.Workbook{
			Format: workbook.CSV,
			Sheets: []workbook.Sheet{{Name: "Sheet1", Cells: map[workbook.Coord]workbook.Cell{{Row: 0, Col: 0}: {Input: "x", Style: workbook.CellStyle{FillRGB: "FF0000"}}}}},
		},
	}
	controller := NewController("")
	controller.session = session
	controller.view = ui.NewModel(session)
	controller.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("save as")})
	_, initPicker := controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if initPicker == nil {
		t.Fatal("Save As did not open the file picker")
	}
	controller.Update(initPicker())
	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("export.csv")})
	controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !controller.confirmCSVLoss || filepath.Base(controller.pendingSavePath) != "export.csv" {
		t.Fatalf("Save As bypassed the CSV loss warning: warning=%t path=%q", controller.confirmCSVLoss, controller.pendingSavePath)
	}
	if !strings.Contains(controller.View(), "formatting") {
		t.Fatalf("CSV loss warning is not visible: %q", controller.View())
	}
}

func TestLoadCompleteHydratesColorScales(t *testing.T) {
	path := filepath.Join(t.TempDir(), "color-scale.xlsx")
	source, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetCell(source, workbook.CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "10"); err != nil {
		t.Fatal(err)
	}
	selection := workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{}}
	scale := workbook.ColorScale{MinRGB: "FF0000", MidRGB: "FFFF00", MaxRGB: "00FF00"}
	if err := workbook.AddColorScaleRule(source, "Sheet1", selection, scale); err != nil {
		t.Fatalf("AddColorScaleRule: %v", err)
	}
	if err := workbook.SaveFile(source, path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := workbook.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	t.Cleanup(func() { _ = loaded.Close() })
	controller := NewController("")
	controller.Update(loadCompleteMsg{path: path, session: loaded})
	rules := controller.session.Workbook.Sheets[0].ConditionalRules
	if len(rules) != 1 || rules[0].ColorScale == nil || *rules[0].ColorScale != scale {
		t.Fatalf("load completion did not hydrate color scale: %#v", rules)
	}
}

func openPicker(t *testing.T, controller *Controller, directory string) {
	t.Helper()
	t.Chdir(directory)
	controller.picker.CurrentDirectory = directory
	_, command := controller.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	if command == nil {
		t.Fatal("Ctrl+O did not start the asynchronous file picker")
	}
	if message := command(); message != nil {
		controller.Update(message)
	}
}
