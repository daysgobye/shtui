package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"shtui/internal/workbook"
)

func TestGridFillDoesNotColorExteriorBorderGlyphs(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 40, 4)
	border := workbook.BorderStyle{Preset: "thick", ColorRGB: "FF0000"}
	sheet := &workbook.Sheet{
		Rows: 1,
		Cols: 1,
		Cells: map[workbook.Coord]workbook.Cell{{}: {Input: "x", Style: workbook.CellStyle{
			FillRGB: "123456",
			Borders: workbook.BorderSet{Top: border, Bottom: border, Left: border, Right: border},
		}}},
	}
	rows := strings.Split(grid.View(sheet, 4), "\n")
	if len(rows) != 4 {
		t.Fatalf("rendered rows = %d, want 4: %q", len(rows), rows)
	}
	borderColor, err := ParseRGBHex("#" + border.ColorRGB)
	if err != nil {
		t.Fatal(err)
	}
	wantVertical := lipgloss.NewStyle().Foreground(TerminalColor(borderColor, supportsTrueColor())).Render("┃")
	wantHorizontal := lipgloss.NewStyle().Foreground(TerminalColor(borderColor, supportsTrueColor())).Render("━")
	assertRenderedBoundary(t, rows[2], grid.RowHeaderWidth-1, "┃", wantVertical)
	assertRenderedBoundary(t, rows[1], grid.RowHeaderWidth, "━", wantHorizontal)
	assertRenderedBoundary(t, rows[3], grid.RowHeaderWidth, "━", wantHorizontal)
}

func TestGridDifferentNeighborFillsKeepSharedBoundariesNeutral(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 40, 8)
	sheet := &workbook.Sheet{
		Rows: 2,
		Cols: 2,
		Cells: map[workbook.Coord]workbook.Cell{
			{Row: 0, Col: 0}: {Style: workbook.CellStyle{FillRGB: "FF0000"}},
			{Row: 0, Col: 1}: {Style: workbook.CellStyle{FillRGB: "0000FF"}},
			{Row: 1, Col: 0}: {Style: workbook.CellStyle{FillRGB: "00FF00"}},
			{Row: 1, Col: 1}: {Style: workbook.CellStyle{FillRGB: "FFFF00"}},
		},
	}
	rows := strings.Split(grid.View(sheet, 8), "\n")
	assertRenderedBoundary(t, rows[2], grid.RowHeaderWidth-1+defaultColumnWidth, "│", "│")
	assertRenderedBoundary(t, rows[3], grid.RowHeaderWidth, "─", "─")
	assertRenderedBoundary(t, rows[3], grid.RowHeaderWidth+defaultColumnWidth, "─", "─")
}

func TestGridSharedBoundaryFillRequiresMatchingAdjacentFills(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	cases := []struct {
		name  string
		first string
		next  string
		want  string
	}{
		{name: "same", first: "FF0000", next: "FF0000", want: "FF0000"},
		{name: "different", first: "FF0000", next: "0000FF"},
		{name: "empty neighbor", first: "FF0000"},
	}
	for _, testCase := range cases {
		t.Run("vertical/"+testCase.name, func(t *testing.T) {
			grid := NewGridViewport()
			grid.SetBounds(0, 0, 40, 4)
			sheet := &workbook.Sheet{
				Rows: 1,
				Cols: 2,
				Cells: map[workbook.Coord]workbook.Cell{
					{Row: 0, Col: 0}: {Style: workbook.CellStyle{FillRGB: testCase.first}},
					{Row: 0, Col: 1}: {Style: workbook.CellStyle{FillRGB: testCase.next}},
				},
			}
			rows := strings.Split(grid.View(sheet, 4), "\n")
			assertFillBackground(t, rows[2], grid.RowHeaderWidth-1+defaultColumnWidth, "│", testCase.want)
		})
		t.Run("horizontal/"+testCase.name, func(t *testing.T) {
			grid := NewGridViewport()
			grid.SetBounds(0, 0, 40, 6)
			sheet := &workbook.Sheet{
				Rows: 2,
				Cols: 1,
				Cells: map[workbook.Coord]workbook.Cell{
					{Row: 0, Col: 0}: {Style: workbook.CellStyle{FillRGB: testCase.first}},
					{Row: 1, Col: 0}: {Style: workbook.CellStyle{FillRGB: testCase.next}},
				},
			}
			rows := strings.Split(grid.View(sheet, 6), "\n")
			assertFillBackground(t, rows[3], grid.RowHeaderWidth, "─", testCase.want)
		})
	}
}

func TestGridJunctionFillRequiresFourMatchingCells(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	for _, testCase := range []struct {
		name     string
		mismatch workbook.Coord
		fill     string
		want     string
	}{
		{name: "all equal", fill: "FF0000", want: "FF0000"},
		{name: "unlike neighbor", fill: "FF0000", mismatch: workbook.Coord{Row: 1, Col: 1}},
		{name: "empty neighbor", fill: "FF0000", mismatch: workbook.Coord{Row: 1, Col: 1}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			grid := NewGridViewport()
			grid.SetBounds(0, 0, 40, 6)
			cells := make(map[workbook.Coord]workbook.Cell, 4)
			for _, coord := range []workbook.Coord{{Row: 0, Col: 0}, {Row: 0, Col: 1}, {Row: 1, Col: 0}, {Row: 1, Col: 1}} {
				fill := testCase.fill
				if testCase.name == "unlike neighbor" && coord == testCase.mismatch {
					fill = "0000FF"
				}
				if testCase.name == "empty neighbor" && coord == testCase.mismatch {
					fill = ""
				}
				cells[coord] = workbook.Cell{Style: workbook.CellStyle{FillRGB: fill}}
			}
			sheet := &workbook.Sheet{Rows: 2, Cols: 2, Cells: cells}
			rows := strings.Split(grid.View(sheet, 6), "\n")
			assertFillBackground(t, rows[3], grid.RowHeaderWidth-1+defaultColumnWidth, "┼", testCase.want)
		})
	}
}

func TestGridMergedBoundaryFillUsesAnchor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	for _, testCase := range []struct {
		name         string
		adjacentFill string
		want         string
	}{
		{name: "matching neighbor", adjacentFill: "FF0000", want: "FF0000"},
		{name: "unlike neighbor", adjacentFill: "0000FF"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			grid := NewGridViewport()
			grid.SetBounds(0, 0, 50, 4)
			sheet := &workbook.Sheet{
				Rows: 1,
				Cols: 3,
				Cells: map[workbook.Coord]workbook.Cell{
					{Row: 0, Col: 0}: {Style: workbook.CellStyle{FillRGB: "FF0000"}},
					{Row: 0, Col: 2}: {Style: workbook.CellStyle{FillRGB: testCase.adjacentFill}},
				},
				Merges: []workbook.CellRange{{Start: workbook.Coord{Row: 0, Col: 0}, End: workbook.Coord{Row: 0, Col: 1}}},
			}
			rows := strings.Split(grid.View(sheet, 4), "\n")
			assertFillBackground(t, rows[2], grid.RowHeaderWidth-1+2*defaultColumnWidth, "│", testCase.want)
		})
	}
}

func assertFillBackground(t *testing.T, line string, x int, glyph, fill string) {
	t.Helper()
	want := glyph
	if fill != "" {
		color, err := ParseRGBHex("#" + fill)
		if err != nil {
			t.Fatal(err)
		}
		want = lipgloss.NewStyle().Background(TerminalColor(color, supportsTrueColor())).Render(glyph)
	}
	assertRenderedBoundary(t, line, x, glyph, want)
}

func TestFilledMergedCellKeepsOuterBorders(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 50, 6)
	fill := "345678"
	border := workbook.BorderStyle{Preset: "thick", ColorRGB: "FF0000"}
	sheet := &workbook.Sheet{
		Rows: 3,
		Cols: 3,
		Cells: map[workbook.Coord]workbook.Cell{
			{}: {Input: "joined", Style: workbook.CellStyle{
				FillRGB: fill,
				Borders: workbook.BorderSet{Top: border, Left: border},
			}},
			{Row: 0, Col: 1}: {Style: workbook.CellStyle{Borders: workbook.BorderSet{Right: border}}},
			{Row: 1, Col: 0}: {Style: workbook.CellStyle{Borders: workbook.BorderSet{Bottom: border, Left: border}}},
			{Row: 1, Col: 1}: {Style: workbook.CellStyle{Borders: workbook.BorderSet{Right: border, Bottom: border}}},
		},
		Merges: []workbook.CellRange{{Start: workbook.Coord{}, End: workbook.Coord{Row: 1, Col: 1}}},
	}
	rows := strings.Split(grid.View(sheet, 6), "\n")
	if !strings.Contains(ansi.Strip(rows[1]), "━") || !strings.Contains(ansi.Strip(rows[5]), "━") {
		t.Fatalf("merged cell lost outer horizontal borders: top=%q bottom=%q", ansi.Strip(rows[1]), ansi.Strip(rows[5]))
	}
	if !strings.Contains(ansi.Strip(rows[2]), "┃") || !strings.Contains(ansi.Strip(rows[4]), "┃") {
		t.Fatalf("merged cell lost outer vertical borders: first=%q second=%q", ansi.Strip(rows[2]), ansi.Strip(rows[4]))
	}
	assertFillBackground(t, rows[3], grid.RowHeaderWidth+1, " ", fill)
	borderColor, err := ParseRGBHex("#" + border.ColorRGB)
	if err != nil {
		t.Fatal(err)
	}
	assertRenderedBoundary(t, rows[5], grid.RowHeaderWidth, "━",
		lipgloss.NewStyle().Foreground(TerminalColor(borderColor, supportsTrueColor())).Render("━"))
}

func assertRenderedBoundary(t *testing.T, line string, x int, glyph, want string) {
	t.Helper()
	gotGlyph, gotStyle := renderedCellAt(line, x)
	wantGlyph, wantStyle := renderedCellAt(want, 0)
	if gotGlyph != glyph || wantGlyph != glyph {
		t.Fatalf("boundary at x=%d = %q; want %q", x, gotGlyph, glyph)
	}
	if colorParameters(gotStyle) != colorParameters(wantStyle) {
		t.Fatalf("boundary at x=%d colors = %q; want %q", x, colorParameters(gotStyle), colorParameters(wantStyle))
	}
}

func renderedCellAt(line string, target int) (string, string) {
	var style strings.Builder
	for position := 0; position < len(line); {
		if strings.HasPrefix(line[position:], "\x1b[") {
			end := strings.IndexByte(line[position:], 'm')
			if end < 0 {
				break
			}
			sequence := line[position : position+end+1]
			params := strings.TrimSuffix(strings.TrimPrefix(sequence, "\x1b["), "m")
			if params == "0" {
				style.Reset()
			} else {
				style.WriteString(sequence)
			}
			position += end + 1
			continue
		}
		r, size := utf8.DecodeRuneInString(line[position:])
		if target == 0 {
			return string(r), style.String()
		}
		target--
		position += size
	}
	return "", ""
}

func colorParameters(style string) string {
	var colors []string
	for position := 0; position < len(style); {
		if !strings.HasPrefix(style[position:], "\x1b[") {
			position++
			continue
		}
		sequenceEnd := strings.IndexByte(style[position:], 'm')
		if sequenceEnd < 0 {
			break
		}
		params := strings.TrimSuffix(strings.TrimPrefix(style[position:position+sequenceEnd+1], "\x1b["), "m")
		values := strings.Split(params, ";")
		for index := 0; index < len(values); index++ {
			if values[index] != "38" && values[index] != "48" {
				continue
			}
			count := 1
			if index+1 < len(values) {
				switch values[index+1] {
				case "5":
					count = 3
				case "2":
					count = 5
				}
			}
			colorEnd := min(index+count, len(values))
			colors = append(colors, strings.Join(values[index:colorEnd], ";"))
			index = colorEnd - 1
		}
		position += sequenceEnd + 1
	}
	return strings.Join(colors, "|")
}

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

func TestCellAtNormalizesMergedCoordinatesToAnchor(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 60, 10)
	sheet := &workbook.Sheet{
		Rows:   3,
		Cols:   4,
		Merges: []workbook.CellRange{{Start: workbook.Coord{}, End: workbook.Coord{Row: 1, Col: 2}}},
	}
	for _, point := range []struct {
		name string
		x    int
		y    int
	}{
		{name: "anchor", x: 6, y: 2},
		{name: "interior", x: 16, y: 4},
		{name: "final cell", x: 26, y: 4},
	} {
		t.Run(point.name, func(t *testing.T) {
			coord, ok := grid.CellAt(point.x, point.y, sheet)
			if !ok || coord != (workbook.Coord{}) {
				t.Fatalf("CellAt(%d,%d) = %v, %t; want merge anchor A1", point.x, point.y, coord, ok)
			}
		})
	}
}

func TestMergedTextWrapsWithinExistingArea(t *testing.T) {
	grid := NewGridViewport()
	grid.SetBounds(0, 0, 60, 10)
	sheet := &workbook.Sheet{
		Rows: 4,
		Cols: 3,
		Cells: map[workbook.Coord]workbook.Cell{
			{}:               {Input: "alpha 東京 bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa quebec romeo sierra tango uniform victor"},
			{Row: 0, Col: 2}: {Input: "OUTSIDE"},
		},
		Merges: []workbook.CellRange{{Start: workbook.Coord{}, End: workbook.Coord{Row: 2, Col: 1}}},
	}
	lines := strings.Split(ansi.Strip(grid.View(sheet, 10)), "\n")
	if len(lines) < 7 || !strings.Contains(lines[2], "alpha") {
		t.Fatalf("merged text is not top-aligned in first content line: %q", lines)
	}
	if !strings.Contains(lines[3], "charlie") || !strings.Contains(lines[4], "foxtrot") {
		t.Fatalf("merged text did not wrap across the merge interior: separator=%q next row=%q", lines[3], lines[4])
	}
	if !strings.Contains(lines[6], "…") {
		t.Fatalf("overflowing merged text lacks an ellipsis on its last interior line: %q", lines[6])
	}
	outsideByte := strings.Index(lines[2], "OUTSIDE")
	if outsideByte < 0 {
		t.Fatalf("neighbor cell content missing from merged row: %q", lines[2])
	}
	gotColumn := ansi.StringWidth(lines[2][:outsideByte])
	wantColumn := grid.RowHeaderWidth + 2*defaultColumnWidth
	if gotColumn != wantColumn {
		t.Fatalf("neighbor cell begins at display column %d, want %d; line=%q", gotColumn, wantColumn, lines[2])
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
