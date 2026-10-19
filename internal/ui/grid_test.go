package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"omasheet/internal/workbook"
)

func TestGridCoordinateMappingWithScrollOffsets(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(10, 3, 40, 8)
	grid.ScrollRow = 5
	grid.ScrollCol = 2
	sheet := &workbook.Sheet{Rows: 50, Cols: 20}
	coord, ok := grid.CellAt(15, 5, sheet)
	if !ok || coord != (workbook.Coord{Row: 5, Col: 2}) {
		t.Fatalf("CellAt = %v, %v; want row 5 col 2", coord, ok)
	}
	coord, ok = grid.CellAt(25, 7, sheet)
	if !ok || coord != (workbook.Coord{Row: 6, Col: 3}) {
		t.Fatalf("scrolled CellAt = %v, %v; want row 6 col 3", coord, ok)
	}
}

func TestMouseWheelScrollPreservesSelection(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 40, 10)
	grid.Selected = workbook.Coord{Row: 4, Col: 3}
	grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: 20, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}), &workbook.Sheet{Rows: 100, Cols: 20})
	if grid.ScrollRow == 0 {
		t.Fatal("wheel down did not scroll the grid")
	}
	if grid.Selected != (workbook.Coord{Row: 4, Col: 3}) {
		t.Fatalf("wheel changed selected cell to %v", grid.Selected)
	}
}

func TestColumnBoundaryDragResizesOnlyTargetColumn(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 60, 12)
	grid.ColumnWidths = map[int]int{0: 10, 1: 12}
	sheet := &workbook.Sheet{Rows: 5, Cols: 3}
	grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: grid.RowHeaderWidth + 10, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}), sheet)
	grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: grid.RowHeaderWidth + 13, Y: 0, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft}), sheet)
	grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: grid.RowHeaderWidth + 13, Y: 0, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft}), sheet)
	if grid.ColumnWidths[0] != 13 || grid.ColumnWidths[1] != 12 {
		t.Fatalf("column widths after drag = %#v, want col 0 = 13 and col 1 = 12", grid.ColumnWidths)
	}
}

func TestTerminalResizePreservesSelectedCell(t *testing.T) {
	model := NewModel(&workbook.Session{Workbook: &workbook.Workbook{Sheets: []workbook.Sheet{{Name: "Sheet1", Rows: 20, Cols: 10, Cells: map[workbook.Coord]workbook.Cell{}}}}})
	model.Grid.Selected = workbook.Coord{Row: 8, Col: 4}
	model.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	if model.Grid.Selected != (workbook.Coord{Row: 8, Col: 4}) {
		t.Fatalf("window resize changed selected cell to %v", model.Grid.Selected)
	}
	if model.Grid.ScrollRow == 0 || model.Grid.ScrollCol == 0 {
		t.Fatal("resize did not scroll the selected cell into the visible viewport")
	}
}

func TestHidingToolbarAddsOneGridRow(t *testing.T) {
	model := NewModel(&workbook.Session{Workbook: &workbook.Workbook{Sheets: []workbook.Sheet{{Name: "Sheet1"}}}})
	model.layout()
	withToolbar := model.Grid.Height
	model.ToggleToolbar()
	model.layout()
	if model.Grid.Height != withToolbar+1 {
		t.Fatalf("grid height changed from %d to %d; hiding toolbar should add one row", withToolbar, model.Grid.Height)
	}
}

func TestMouseDragSelectsRectangularRange(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 60, 12)
	sheet := &workbook.Sheet{Rows: 8, Cols: 8}
	grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: 5, Y: 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}), sheet)
	grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: 25, Y: 6, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft}), sheet)
	grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: 25, Y: 6, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft}), sheet)
	want := workbook.CellRange{Start: workbook.Coord{Row: 0, Col: 0}, End: workbook.Coord{Row: 2, Col: 2}}
	if grid.Selection != want {
		t.Fatalf("drag selection = %#v, want %#v", grid.Selection, want)
	}
}

func TestGridRendersDefaultCellGridlines(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 40, 8)
	view := ansi.Strip(grid.View(&workbook.Sheet{Rows: 4, Cols: 3}, 8))
	if !strings.Contains(view, "│") || !strings.Contains(view, "─") {
		t.Fatalf("grid view lacks visible cell boundaries: %q", view)
	}
}

func TestExplicitBorderPresetsRenderAllEdges(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 30, 8)
	style := workbook.CellStyle{Borders: workbook.BorderSet{
		Top:    workbook.BorderStyle{Preset: "thick"},
		Right:  workbook.BorderStyle{Preset: "thick"},
		Bottom: workbook.BorderStyle{Preset: "thick"},
		Left:   workbook.BorderStyle{Preset: "thick"},
	}}
	sheet := &workbook.Sheet{Rows: 1, Cols: 1, Cells: map[workbook.Coord]workbook.Cell{{}: {Input: "edge", Style: style}}}
	view := ansi.Strip(grid.View(sheet, 8))
	for _, glyph := range []string{"┃", "━"} {
		if !strings.Contains(view, glyph) {
			t.Errorf("explicit thick border view lacks %q: %q", glyph, view)
		}
	}
}

func TestMergedRangeRendersAsJoinedCells(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 50, 8)
	sheet := &workbook.Sheet{
		Rows:   3,
		Cols:   3,
		Cells:  map[workbook.Coord]workbook.Cell{{}: {Input: "joined"}, {Row: 0, Col: 1}: {Input: "hidden"}, {Row: 1, Col: 0}: {Input: "hidden"}, {Row: 0, Col: 2}: {Input: "outside"}},
		Merges: []workbook.CellRange{{Start: workbook.Coord{}, End: workbook.Coord{Row: 1, Col: 1}}},
	}
	view := ansi.Strip(grid.View(sheet, 8))
	if strings.Count(view, "joined") != 1 || strings.Contains(view, "hidden") || !strings.Contains(view, "outside") {
		t.Fatalf("merged view = %q; want top-left once, hidden interior values, and visible outside cell", view)
	}
}

func TestGridCellAtMapsContentAndSeparatorLines(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 40, 8)
	sheet := &workbook.Sheet{Rows: 10, Cols: 5}
	for _, y := range []int{2, 3} {
		coord, ok := grid.CellAt(6, y, sheet)
		if !ok || coord != (workbook.Coord{}) {
			t.Errorf("CellAt(6,%d) = %v, %t; want A1 on content and separator lines", y, coord, ok)
		}
	}
	coord, ok := grid.CellAt(6, 4, sheet)
	if !ok || coord != (workbook.Coord{Row: 1}) {
		t.Fatalf("CellAt on second row = %v, %t; want A2", coord, ok)
	}
}

func TestColumnResizeAcceptsBoundaryHitZone(t *testing.T) {
	sheet := &workbook.Sheet{Rows: 1, Cols: 3}
	for _, offset := range []int{-1, 0, 1} {
		grid := NewGridViewport()
		grid.SetBounds(0, 0, 40, 8)
		grid.ColumnWidths[0] = 10
		x := grid.RowHeaderWidth + 10 + offset
		grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: x, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}), sheet)
		if grid.resizingColumn != 0 {
			t.Errorf("press at boundary offset %d started resize for column %d; want column 0", offset, grid.resizingColumn)
		}
	}
}

func TestColumnResizePersistsOnlyDraggedColumn(t *testing.T) {
	session, err := workbook.NewWorkbook(workbook.XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	model := NewModel(session)
	model.layout()
	boundary := model.Grid.RowHeaderWidth + defaultColumnWidth
	y := model.Grid.OriginY
	for _, event := range []tea.MouseEvent{
		{X: boundary, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft},
		{X: boundary + 4, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft},
		{X: boundary + 4, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft},
	} {
		model.Update(tea.MouseMsg(event))
	}
	widths := session.Workbook.Sheets[0].ColumnWidths
	if widths[0] != 14 {
		t.Fatalf("persisted width A = %v, want 14", widths[0])
	}
	if _, changed := widths[1]; changed {
		t.Fatalf("dragging column A changed column B width: %#v", widths)
	}
}
