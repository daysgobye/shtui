package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"omasheet/internal/workbook"
)

func TestGridCoordinateMappingWithScrollOffsets(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(10, 3, 40, 8)
	grid.ScrollRow = 5
	grid.ScrollCol = 2
	sheet := &workbook.Sheet{Rows: 50, Cols: 20}
	coord, ok := grid.CellAt(15, 4, sheet)
	if !ok || coord != (workbook.Coord{Row: 5, Col: 2}) {
		t.Fatalf("CellAt = %v, %v; want row 5 col 2", coord, ok)
	}
	coord, ok = grid.CellAt(25, 6, sheet)
	if !ok || coord != (workbook.Coord{Row: 7, Col: 3}) {
		t.Fatalf("scrolled CellAt = %v, %v; want row 7 col 3", coord, ok)
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
	grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: 5, Y: 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}), sheet)
	grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: 25, Y: 3, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft}), sheet)
	grid.HandleMouse(tea.MouseMsg(tea.MouseEvent{X: 25, Y: 3, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft}), sheet)
	want := workbook.CellRange{Start: workbook.Coord{Row: 0, Col: 0}, End: workbook.Coord{Row: 2, Col: 2}}
	if grid.Selection != want {
		t.Fatalf("drag selection = %#v, want %#v", grid.Selection, want)
	}
}
