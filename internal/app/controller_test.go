package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/xuri/excelize/v2"
	"shtui/internal/ui"
	"shtui/internal/workbook"
)

func TestQuitConfirmationKeyboardAndDoubleCtrlC(t *testing.T) {
	controller := NewController("")
	_, command := controller.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if command != nil || !controller.quitDialog.open {
		t.Fatalf("first Ctrl+C: dialogOpen=%t quitCommand=%t", controller.quitDialog.open, command != nil)
	}
	if controller.quitDialog.selected != 1 {
		t.Fatalf("initial quit selection = %d, want Nope at index 1", controller.quitDialog.selected)
	}
	controller.Update(tea.KeyMsg{Type: tea.KeyTab})
	_, command = controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("Tab then Enter on Yep did not confirm quit")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatal("confirm action did not return tea.Quit")
	}

	yep := NewController("")
	yep.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_, command = yep.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if command == nil {
		t.Fatal("Y did not confirm quit")
	}

	cancelled := NewController("")
	cancelled.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_, command = cancelled.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if command != nil || cancelled.quitDialog.open {
		t.Fatalf("N did not cancel quit: dialogOpen=%t command=%t", cancelled.quitDialog.open, command != nil)
	}

	escape := NewController("")
	before := escape.View()
	escape.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_, command = escape.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if command != nil || escape.quitDialog.open || escape.View() != before {
		t.Fatal("Esc did not cancel quit and restore the unchanged view")
	}

	double := NewController("")
	double.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_, command = double.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if command == nil {
		t.Fatal("second Ctrl+C did not bypass confirmation")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatal("second Ctrl+C did not return tea.Quit")
	}
}
func TestQuitDialogUsesUppercaseBrandText(t *testing.T) {
	plain := ansi.Strip((quitDialog{}).View(80, 24, false))
	if !strings.Contains(plain, "Quit SHTUI?") {
		t.Fatalf("quit dialog heading = %q, want uppercase SHTUI", plain)
	}
	if strings.Contains(plain, "A           B           C           D           E") {
		t.Fatalf("quit dialog rendered the full grid wordmark: %q", plain)
	}
}

func TestQuitConfirmationMouseActions(t *testing.T) {
	yes := NewController("")
	yes.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	x, y := controllerActionPoint(t, yes.View(), "Yep!")
	_, command := yes.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if command == nil {
		t.Fatal("clicking Yep did not confirm quit")
	}

	no := NewController("")
	no.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	x, y = controllerActionPoint(t, no.View(), "Nope")
	_, command = no.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if command != nil || no.quitDialog.open {
		t.Fatalf("clicking Nope: dialogOpen=%t command=%t", no.quitDialog.open, command != nil)
	}

	outside := NewController("")
	outside.session = &workbook.Session{
		Format: workbook.CSV,
		Workbook: &workbook.Workbook{Format: workbook.CSV, Sheets: []workbook.Sheet{
			{Name: "Sheet1", Rows: 4, Cols: 4, Cells: map[workbook.Coord]workbook.Cell{}},
		}},
	}
	outside.view = ui.NewModel(outside.session)
	outside.view.Grid.Selected = workbook.Coord{Row: 2, Col: 2}
	outside.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_, command = outside.Update(tea.MouseMsg(tea.MouseEvent{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if command != nil || !outside.quitDialog.open || outside.view.Grid.Selected != (workbook.Coord{Row: 2, Col: 2}) {
		t.Fatal("click outside quit actions quit, closed the modal, or changed the workbook selection")
	}
}

func TestQuitConfirmationDirtyWarning(t *testing.T) {
	for _, test := range []struct {
		name  string
		dirty bool
		want  string
	}{
		{name: "dirty", dirty: true, want: "Unsaved changes will be lost"},
		{name: "clean", want: "Are you sure you want to quit?"},
	} {
		t.Run(test.name, func(t *testing.T) {
			controller := NewController("")
			controller.session = &workbook.Session{
				Format: workbook.CSV,
				Workbook: &workbook.Workbook{Format: workbook.CSV, Dirty: test.dirty, Sheets: []workbook.Sheet{
					{Name: "Sheet1", Cells: map[workbook.Coord]workbook.Cell{}},
				}},
			}
			controller.view = ui.NewModel(controller.session)
			controller.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			if view := ansi.Strip(controller.View()); !strings.Contains(view, test.want) {
				t.Fatalf("quit dialog = %q, want %q", view, test.want)
			}
		})
	}
}

func TestQuitConfirmationFitsCompactTerminal(t *testing.T) {
	controller := NewController("")
	controller.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	controller.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	view := ansi.Strip(controller.View())
	rows := strings.Split(view, "\n")
	if len(rows) > 8 {
		t.Fatalf("quit dialog rendered %d rows in 8-row terminal: %q", len(rows), view)
	}
	for index, row := range rows {
		if width := ansi.StringWidth(row); width > 30 {
			t.Errorf("quit dialog row %d width=%d in 30-column terminal: %q", index, width, row)
		}
	}
	for _, action := range []string{"Yep!", "Nope"} {
		if !strings.Contains(view, action) {
			t.Errorf("compact quit dialog hides %q: %q", action, view)
		}
	}
}
func TestWelcomeMouseCreatesCSVWorkbook(t *testing.T) {
	controller := NewController("")
	x, y := controllerActionPoint(t, controller.View(), "New CSV")
	controller.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if controller.session == nil || controller.session.Format != workbook.CSV {
		t.Fatalf("clicking New CSV did not create a CSV workbook: %#v", controller.session)
	}
	t.Cleanup(func() { _ = controller.session.Close() })
}

func TestWelcomeMouseStartsOpenPicker(t *testing.T) {
	controller := NewController("")
	x, y := controllerActionPoint(t, controller.View(), "Open workbook")
	_, command := controller.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
	if !controller.pickerOpen || command == nil {
		t.Fatalf("clicking Open workbook: pickerOpen=%t command=%t", controller.pickerOpen, command != nil)
	}
}

func controllerActionPoint(t *testing.T, view, label string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(view), "\n") {
		if x := strings.Index(line, label); x >= 0 {
			return x, y
		}
	}
	t.Fatalf("rendered controller view has no %q action: %q", label, ansi.Strip(view))
	return 0, 0
}

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
	if got, want := controller.exitSummary(), "Opened workbook: "+path; got != want {
		t.Fatalf("opened workbook summary = %q, want %q", got, want)
	}
	if !strings.Contains(controller.View(), "pear") {
		t.Fatal("loaded CSV is not visible in the TUI")
	}
}
func TestMouseResizedExistingXLSXColumnPersistsAfterSaveAndReopen(t *testing.T) {
	fixture := filepath.Join("..", "workbook", "testdata", "preservation", "chart.xlsx")
	fixtureBytes, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read XLSX preservation fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "chart.xlsx")
	if err := os.WriteFile(path, fixtureBytes, 0o600); err != nil {
		t.Fatalf("copy XLSX preservation fixture: %v", err)
	}

	controller := NewController(path)
	controller.Update(controller.Init()())
	if controller.session == nil || controller.session.Format != workbook.XLSX {
		t.Fatalf("opened session = %#v, want XLSX", controller.session)
	}
	t.Cleanup(func() {
		if controller.session != nil {
			_ = controller.session.Close()
		}
	})
	initialWidth := controller.session.Workbook.Sheets[0].ColumnWidths[0]
	controller.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	boundaryX := controller.view.Grid.RowHeaderWidth + int(initialWidth)
	mouse := func(x int, action tea.MouseAction) {
		t.Helper()
		controller.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: controller.view.Grid.OriginY, Action: action, Button: tea.MouseButtonLeft}))
	}
	mouse(boundaryX, tea.MouseActionPress)
	mouse(boundaryX+5, tea.MouseActionMotion)
	mouse(boundaryX+5, tea.MouseActionRelease)

	wantWidth := initialWidth + 5
	if got := controller.session.Workbook.Sheets[0].ColumnWidths[0]; got != wantWidth {
		t.Fatalf("resized model column A width = %v, want %v", got, wantWidth)
	}
	if err := workbook.SaveFile(controller.session, path); err != nil {
		t.Fatalf("save resized preservation fixture: %v", err)
	}
	if err := controller.session.Close(); err != nil {
		t.Fatalf("close resized workbook: %v", err)
	}
	controller.session = nil

	reopened := NewController(path)
	reopened.Update(reopened.Init()())
	if reopened.session == nil || reopened.session.Format != workbook.XLSX {
		t.Fatalf("reopened session = %#v, want XLSX", reopened.session)
	}
	t.Cleanup(func() { _ = reopened.session.Close() })
	if got := reopened.session.Workbook.Sheets[0].ColumnWidths[0]; got != wantWidth {
		t.Fatalf("reopened model column A width = %v, want %v", got, wantWidth)
	}
	xlsx, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("open saved XLSX with Excelize: %v", err)
	}
	defer xlsx.Close()
	if got, err := xlsx.GetColWidth("Sheet1", "A"); err != nil {
		t.Fatalf("GetColWidth: %v", err)
	} else if got != wantWidth {
		t.Fatalf("persisted Excelize column A width = %v, want %v", got, wantWidth)
	}
	reopened.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	gridView := reopened.view.Grid.View(&reopened.session.Workbook.Sheets[0], 8)
	header := strings.Split(ansi.Strip(strings.Split(gridView, "\n")[0]), "│")
	renderedColumnWidth := ansi.StringWidth(header[1]) + 1
	if renderedColumnWidth != int(wantWidth) {
		t.Fatalf("reopened rendered column A width = %d, want %v: %q", renderedColumnWidth, wantWidth, header[1])
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
	if !strings.Contains(view, "Open workbook") || !strings.Contains(view, "New CSV") {
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

func TestCSVSaveChoiceOpensAsOverlay(t *testing.T) {
	controller := newCSVSaveController(t, filepath.Join(t.TempDir(), "book.csv"))
	openCSVSaveChoice(t, controller)
	view := strings.Split(ansi.Strip(controller.View()), "\n")
	titleRow := -1
	for row, line := range view {
		if strings.Contains(line, "Save workbook") {
			titleRow = row
			break
		}
	}
	if titleRow < 5 || titleRow > 18 {
		t.Fatalf("save dialog title row = %d, want a centered overlay in the 24-row view: %q", titleRow, view)
	}
	if !strings.Contains(strings.Join(view, "\n"), "active s") {
		t.Fatalf("workbook content beneath the centered dialog is not visible: %q", view)
	}
	if !strings.Contains(strings.Join(view, "\n"), "Save as XLSX") || !strings.Contains(strings.Join(view, "\n"), "Cancel") {
		t.Fatalf("save choices are missing from overlay: %q", view)
	}
}

func TestCSVSaveChoiceKeyboardActions(t *testing.T) {
	t.Run("CSV", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "book.csv")
		controller := newCSVSaveController(t, path)
		openCSVSaveChoice(t, controller)
		_, command := controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
		if command == nil || saveDialogVisible(controller) {
			t.Fatalf("CSV shortcut: command=%t dialogVisible=%t", command != nil, saveDialogVisible(controller))
		}
		controller.Update(command())
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("CSV choice did not save to the current path: %v", err)
		}
	})
	t.Run("XLSX", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "book.csv")
		controller := newCSVSaveController(t, path)
		openCSVSaveChoice(t, controller)
		_, command := controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
		if command == nil || !controller.pickerOpen || controller.pickerMode != "save-xlsx" || controller.saveName.Value() != "book.xlsx" {
			t.Fatalf("XLSX shortcut: command=%t picker=%t mode=%q name=%q", command != nil, controller.pickerOpen, controller.pickerMode, controller.saveName.Value())
		}
	})
	t.Run("Escape", func(t *testing.T) {
		controller := newCSVSaveController(t, filepath.Join(t.TempDir(), "book.csv"))
		session := controller.session
		pathBefore := session.Path
		openCSVSaveChoice(t, controller)
		controller.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if saveDialogVisible(controller) || controller.session != session || session.Format != workbook.CSV || session.Path != pathBefore {
			t.Fatalf("Escape changed save state: dialogVisible=%t format=%s path=%q", saveDialogVisible(controller), session.Format, session.Path)
		}
	})
}

func TestCSVSaveChoiceMouseActions(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		label string
	}{
		{name: "CSV", label: "Save as CSV"},
		{name: "XLSX", label: "Save as XLSX"},
		{name: "Cancel", label: "Cancel"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			controller := newCSVSaveController(t, filepath.Join(t.TempDir(), "book.csv"))
			openCSVSaveChoice(t, controller)
			x, y := controllerActionPoint(t, controller.View(), testCase.label)
			_, command := controller.Update(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
			if saveDialogVisible(controller) {
				t.Fatalf("clicking %q left the dialog visible", testCase.label)
			}
			switch testCase.name {
			case "CSV":
				if command == nil || controller.session.Format != workbook.CSV {
					t.Fatalf("CSV click: command=%t format=%s", command != nil, controller.session.Format)
				}
			case "XLSX":
				if command == nil || !controller.pickerOpen || controller.pickerMode != "save-xlsx" {
					t.Fatalf("XLSX click: command=%t picker=%t mode=%q", command != nil, controller.pickerOpen, controller.pickerMode)
				}
			case "Cancel":
				if command != nil || controller.session.Format != workbook.CSV {
					t.Fatalf("Cancel click: command=%t format=%s", command != nil, controller.session.Format)
				}
			}
		})
	}
}

func TestCSVSaveChoiceCSVExportsActiveSheetAndKeepsSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.csv")
	controller := newCSVSaveController(t, path)
	session := controller.session
	activeStyle := workbook.CellStyle{FillRGB: "123456", Bold: true}
	if err := workbook.ApplyStyle(session, "Data", workbook.CellRange{}, activeStyle); err != nil {
		t.Fatalf("ApplyStyle: %v", err)
	}
	openCSVSaveChoice(t, controller)
	_, command := controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if command == nil {
		t.Fatal("CSV choice did not return a save command")
	}
	controller.Update(command())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read CSV export: %v", err)
	}
	if !strings.Contains(string(data), "active sheet") || strings.Contains(string(data), "first sheet") {
		t.Fatalf("CSV export did not contain only the active sheet: %q", data)
	}
	if controller.session != session || session.Format != workbook.CSV || len(session.Workbook.Sheets) != 2 {
		t.Fatalf("CSV export changed the live workbook: format=%s sheets=%d", session.Format, len(session.Workbook.Sheets))
	}
	if got := session.Workbook.Sheets[1].Cells[workbook.Coord{}].Style; got != activeStyle {
		t.Fatalf("CSV export discarded live formatting: %#v", got)
	}
}

func TestCSVSaveChoiceXLSXPickerUsesXLSXName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.csv")
	controller := newCSVSaveController(t, path)
	openCSVSaveChoice(t, controller)
	_, pickerCommand := controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if pickerCommand == nil {
		t.Fatal("XLSX choice did not open the save picker")
	}
	controller.saveName.SetValue("converted.xlsx")
	_, saveCommand := controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if saveCommand == nil {
		t.Fatal("choosing an XLSX path did not start conversion")
	}
	controller.Update(saveCommand())
	convertedPath := filepath.Join(filepath.Dir(path), "converted.xlsx")
	if controller.session.Format != workbook.XLSX || controller.session.Path != convertedPath || controller.pickerOpen || saveDialogVisible(controller) {
		t.Fatalf("XLSX save result: format=%s path=%q picker=%t dialogVisible=%t", controller.session.Format, controller.session.Path, controller.pickerOpen, saveDialogVisible(controller))
	}
	if _, err := os.Stat(convertedPath); err != nil {
		t.Fatalf("converted XLSX missing: %v", err)
	}
}

func newCSVSaveController(t *testing.T, path string) *Controller {
	t.Helper()
	session, err := workbook.NewWorkbook(workbook.CSV)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if path != "" {
		session.Path = path
		session.Workbook.Path = path
	}
	if err := workbook.SetCell(session, workbook.CellAddress{Sheet: "Sheet1"}, "first sheet"); err != nil {
		t.Fatalf("SetCell first sheet: %v", err)
	}
	if err := workbook.ApplyStyle(session, "Sheet1", workbook.CellRange{}, workbook.CellStyle{FillRGB: "FF0000"}); err != nil {
		t.Fatalf("ApplyStyle: %v", err)
	}
	if err := workbook.AddSheet(session, "Data"); err != nil {
		t.Fatalf("AddSheet: %v", err)
	}
	if err := workbook.SetCell(session, workbook.CellAddress{Sheet: "Data"}, "active sheet"); err != nil {
		t.Fatalf("SetCell active sheet: %v", err)
	}
	session.Workbook.ActiveSheet = 1
	controller := NewController("")
	controller.session = session
	controller.path = path
	controller.view = ui.NewModel(session)
	controller.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return controller
}

func openCSVSaveChoice(t *testing.T, controller *Controller) {
	t.Helper()
	controller.Update(teaKeyCtrlS())
	if !saveDialogVisible(controller) {
		t.Fatalf("Save did not open CSV choices: %q", controller.View())
	}
}

func saveDialogVisible(controller *Controller) bool {
	return strings.Contains(ansi.Strip(controller.View()), "Save workbook")
}

func TestSaveAsPickerKeepsCSVLossWarning(t *testing.T) {
	controller := newCSVSaveController(t, "")
	controller.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("save as")})
	_, initPicker := controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if initPicker == nil {
		t.Fatal("Save As did not open the file picker")
	}
	controller.Update(initPicker())
	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("export.csv")})
	controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !saveDialogVisible(controller) || filepath.Base(controller.pendingSavePath) != "export.csv" {
		t.Fatalf("Save As did not open CSV choices: dialogVisible=%t path=%q", saveDialogVisible(controller), controller.pendingSavePath)
	}
	if !strings.Contains(controller.View(), "Save as XLSX") || !strings.Contains(controller.View(), "formatting") {
		t.Fatalf("CSV save choices are not visible: %q", controller.View())
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

func TestExitSummaryNewAndAddedSheet(t *testing.T) {
	if got, want := NewController("").exitSummary(), "Thanks for using Shtui."; got != want {
		t.Fatalf("neutral exit summary = %q, want %q", got, want)
	}
	newBook := NewController("")
	newBook.newWorkbook(workbook.CSV)
	t.Cleanup(func() {
		if newBook.session != nil {
			_ = newBook.session.Close()
		}
	})
	if got, want := newBook.exitSummary(), "New CSV workbook (not saved)"; got != want {
		t.Fatalf("new workbook summary = %q, want %q", got, want)
	}

	path := filepath.Join(t.TempDir(), "saved.xlsx")
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	if err := workbook.SaveFile(session, path); err != nil {
		t.Fatal(err)
	}
	added := NewController("")
	added.session, added.path, added.view = session, path, added.newView(session)
	t.Cleanup(func() { _ = session.Close() })
	added.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	added.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("add sheet")})
	added.Update(tea.KeyMsg{Type: tea.KeyEnter})
	want := "Added Sheet2 to " + path + " (unsaved)"
	if got := added.exitSummary(); got != want {
		t.Fatalf("added sheet summary = %q, want %q", got, want)
	}
}

func TestExitSummarySaveAndLaterEdits(t *testing.T) {
	controller := NewController("")
	controller.newWorkbook(workbook.CSV)
	t.Cleanup(func() { _ = controller.session.Close() })
	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("before save")})
	controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := controller.exitSummary(); !strings.Contains(got, "(not saved)") {
		t.Fatalf("edited new workbook summary = %q, want not-saved state", got)
	}

	path := filepath.Join(t.TempDir(), "saved.csv")
	controller.Update(controller.saveCmd(path)())
	wantSaved := "Saved workbook: " + path
	if got := controller.exitSummary(); got != wantSaved {
		t.Fatalf("successful save summary = %q, want %q", got, wantSaved)
	}
	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("after save")})
	controller.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wantEdited := "Edited workbook: " + path + " (unsaved)"
	if got := controller.exitSummary(); got != wantEdited {
		t.Fatalf("post-save edit summary = %q, want %q", got, wantEdited)
	}
}

func TestExitSummaryKeyboardUndoRedo(t *testing.T) {
	controller := NewController("")
	controller.newWorkbook(workbook.CSV)
	t.Cleanup(func() { _ = controller.session.Close() })
	controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("value")})
	controller.Update(tea.KeyMsg{Type: tea.KeyEnter})

	controller.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if got, want := controller.exitSummary(), "Undid workbook change (not saved)"; got != want {
		t.Fatalf("undo summary = %q, want %q", got, want)
	}
	controller.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if got, want := controller.exitSummary(), "Undid workbook change (not saved)"; got != want {
		t.Fatalf("failed second undo changed summary to %q, want %q", got, want)
	}
	controller.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	if got, want := controller.exitSummary(), "Redid workbook change (not saved)"; got != want {
		t.Fatalf("redo summary = %q, want %q", got, want)
	}
	controller.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	if got, want := controller.exitSummary(), "Redid workbook change (not saved)"; got != want {
		t.Fatalf("failed second redo changed summary to %q, want %q", got, want)
	}
}

func TestExitSummaryIgnoresFailedOrCanceledSave(t *testing.T) {
	controller := NewController("")
	controller.newWorkbook(workbook.CSV)
	t.Cleanup(func() { _ = controller.session.Close() })
	want := controller.exitSummary()
	controller.Update(saveCompleteMsg{path: filepath.Join(t.TempDir(), "failed.csv"), err: os.ErrPermission})
	if got := controller.exitSummary(); got != want {
		t.Fatalf("failed save changed summary from %q to %q", want, got)
	}

	controller.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if !controller.pickerOpen {
		t.Fatal("save from an untitled workbook did not open a picker")
	}
	controller.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := controller.exitSummary(); got != want {
		t.Fatalf("canceled save changed summary from %q to %q", want, got)
	}
}

func TestRunPrintsSummaryOnlyAfterConfirmedQuit(t *testing.T) {
	for _, test := range []struct {
		name      string
		confirmed bool
		runErr    error
		wantText  bool
	}{
		{name: "context cancellation"},
		{name: "run error", confirmed: true, runErr: os.ErrClosed},
		{name: "confirmed quit", confirmed: true, wantText: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := formatExitSummary(test.confirmed, test.runErr, "Saved workbook: /tmp/book.xlsx")
			if got := strings.Contains(ansi.Strip(output), "Saved workbook: /tmp/book.xlsx"); got != test.wantText {
				t.Fatalf("summary output = %q, contains summary=%t want %t", ansi.Strip(output), got, test.wantText)
			}
			if test.wantText {
				plain := ansi.Strip(output)
				for _, part := range []string{
					"A           B           C           D           E",
					"1 │ █████████ │ ███   ███ │ █████████ │ ███   ███ │ █████████ │",
					"5 │ █████████ │ ███   ███ │    ███    │ █████████ │ █████████ │",
				} {
					if !strings.Contains(plain, part) {
						t.Fatalf("confirmed summary lacks wordmark part %q: %q", part, plain)
					}
				}
			}
		})
	}
}
func TestPreservationSaveFailureOffersBackedUpForceSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	if err := workbook.SaveFile(session, path); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	if err := workbook.SetCell(session, workbook.CellAddress{Sheet: "Sheet1"}, "edited"); err != nil {
		t.Fatalf("edit workbook: %v", err)
	}
	session.Workbook.Dirty = true
	controller := NewController("")
	controller.session = session
	controller.path = path
	controller.view = ui.NewModel(session)
	controller.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	t.Cleanup(func() { _ = session.Close() })

	controller.Update(saveCompleteMsg{
		path: path,
		err:  errors.New(`refusing save of "book.xlsx" to preserve workbook content: relationship part "xl/_rels/workbook.xml.rels" changed`),
	})
	view := ansi.Strip(controller.saveRecoveryDialog.View(controller.width, controller.height))
	for _, want := range []string{"Force save (Y)", "backup", "charts", "Cancel"} {
		if !strings.Contains(strings.ToLower(view), strings.ToLower(want)) {
			t.Fatalf("save recovery prompt lacks %q: %q", want, view)
		}
	}
	_, command := controller.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if command == nil {
		t.Fatal("Y did not start force save")
	}
	message, ok := command().(saveCompleteMsg)
	if !ok || message.err != nil {
		t.Fatalf("force save result = %#v, want successful saveCompleteMsg", message)
	}
	if message.backupPath == "" {
		t.Fatal("force save did not report the backup path")
	}
	if _, err := os.Stat(message.backupPath); err != nil {
		t.Fatalf("force-save backup does not exist: %v", err)
	}
	if session.Workbook.Dirty {
		t.Fatal("successful force save left workbook dirty")
	}
}
