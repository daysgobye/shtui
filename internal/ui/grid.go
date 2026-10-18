package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"omasheet/internal/workbook"
)

const (
	defaultColumnWidth = 10
	minimumColumnWidth = 4
	maximumColumnWidth = 40
)

type GridViewport struct {
	OriginX           int
	OriginY           int
	Width             int
	Height            int
	RowHeaderWidth    int
	HeaderHeight      int
	ScrollRow         int
	ScrollCol         int
	Selected          workbook.Coord
	Selection         workbook.CellRange
	ColumnWidths      map[int]int
	VisibleRowIndexes []int

	resizingColumn  int
	resizeStartX    int
	resizeStartW    int
	selectionAnchor workbook.Coord
	selecting       bool
}

func NewGridViewport() GridViewport {
	return GridViewport{
		RowHeaderWidth: 5,
		HeaderHeight:   1,
		ColumnWidths:   make(map[int]int),
		Selection:      workbook.CellRange{},
		resizingColumn: -1,
	}
}

func (grid *GridViewport) SetBounds(originX, originY, width, height int) {
	grid.OriginX = originX
	grid.OriginY = originY
	grid.Width = max(width, 0)
	grid.Height = max(height, 0)
}

func (grid *GridViewport) CellAt(screenX, screenY int, sheet *workbook.Sheet) (workbook.Coord, bool) {
	if grid == nil || sheet == nil {
		return workbook.Coord{}, false
	}
	x := screenX - grid.OriginX
	y := screenY - grid.OriginY
	if x < grid.RowHeaderWidth || y < grid.HeaderHeight || x >= grid.Width || y >= grid.Height {
		return workbook.Coord{}, false
	}
	colOffset := x - grid.RowHeaderWidth
	col := grid.ScrollCol
	for colOffset >= grid.columnWidth(col, sheet) {
		colOffset -= grid.columnWidth(col, sheet)
		col++
	}
	row := grid.ScrollRow + y - grid.HeaderHeight
	if len(grid.VisibleRowIndexes) > 0 {
		if row < 0 || row >= len(grid.VisibleRowIndexes) {
			return workbook.Coord{}, false
		}
		row = grid.VisibleRowIndexes[row]
	}
	return workbook.Coord{Row: row, Col: col}, true
}

// HandleMouse updates selection, scroll, or a column drag. Resize is reported once on release.
func (grid *GridViewport) HandleMouse(message tea.MouseMsg, sheet *workbook.Sheet) (int, int, bool) {
	if grid == nil || sheet == nil {
		return 0, 0, false
	}
	event := tea.MouseEvent(message)
	switch event.Button {
	case tea.MouseButtonWheelDown:
		grid.ScrollRow += 3
		return 0, 0, false
	case tea.MouseButtonWheelUp:
		grid.ScrollRow = max(0, grid.ScrollRow-3)
		return 0, 0, false
	}
	localX := event.X - grid.OriginX
	localY := event.Y - grid.OriginY
	if grid.resizingColumn >= 0 {
		if event.Action == tea.MouseActionMotion {
			grid.ColumnWidths[grid.resizingColumn] = clamp(grid.resizeStartW+event.X-grid.resizeStartX, minimumColumnWidth, maximumColumnWidth)
		}
		if event.Action == tea.MouseActionRelease {
			col, width := grid.resizingColumn, grid.ColumnWidths[grid.resizingColumn]
			grid.resizingColumn = -1
			return col, width, width != grid.resizeStartW
		}
		return 0, 0, false
	}
	if grid.selecting && (event.Action == tea.MouseActionMotion || event.Action == tea.MouseActionRelease) {
		if coord, ok := grid.CellAt(event.X, event.Y, sheet); ok {
			grid.Selected = coord
			grid.Selection = workbook.CellRange{
				Start: workbook.Coord{Row: min(grid.selectionAnchor.Row, coord.Row), Col: min(grid.selectionAnchor.Col, coord.Col)},
				End:   workbook.Coord{Row: max(grid.selectionAnchor.Row, coord.Row), Col: max(grid.selectionAnchor.Col, coord.Col)},
			}
		}
		if event.Action == tea.MouseActionRelease {
			grid.selecting = false
		}
		return 0, 0, false
	}
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return 0, 0, false
	}
	if localY == 0 && localX >= grid.RowHeaderWidth {
		if col, ok := grid.columnBoundaryAt(localX, sheet); ok {
			grid.resizingColumn = col
			grid.resizeStartX = event.X
			grid.resizeStartW = grid.columnWidth(col, sheet)
			return 0, 0, false
		}
	}
	if coord, ok := grid.CellAt(event.X, event.Y, sheet); ok {
		grid.Selected = coord
		grid.selectionAnchor = coord
		grid.selecting = true
		grid.Selection = workbook.CellRange{Start: coord, End: coord}
	}
	return 0, 0, false
}

func (grid *GridViewport) columnBoundaryAt(x int, sheet *workbook.Sheet) (int, bool) {
	position := grid.RowHeaderWidth
	for col := grid.ScrollCol; position <= x; col++ {
		position += grid.columnWidth(col, sheet)
		if position == x {
			return col, true
		}
		if position > x {
			return 0, false
		}
	}
	return 0, false
}

func (grid *GridViewport) columnWidth(col int, sheet *workbook.Sheet) int {
	if width, exists := grid.ColumnWidths[col]; exists {
		return clamp(width, minimumColumnWidth, maximumColumnWidth)
	}
	if width, exists := sheet.ColumnWidths[col]; exists && width > 0 {
		return clamp(int(width), minimumColumnWidth, maximumColumnWidth)
	}
	return defaultColumnWidth
}

func (grid *GridViewport) View(sheet *workbook.Sheet, height int) string {
	if grid == nil || sheet == nil || height <= 0 {
		return ""
	}
	var output strings.Builder
	columnHeader := lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6))
	rowHeader := lipgloss.NewStyle().Foreground(ANSIColor(8))
	output.WriteString(strings.Repeat(" ", grid.RowHeaderWidth-1))
	rowCount := max(height-grid.HeaderHeight, 0)
	columns := grid.visibleColumns(sheet)
	for _, col := range columns {
		width := grid.columnWidth(col, sheet)
		label := columnName(col)
		output.WriteString(" ")
		output.WriteString(columnHeader.Width(width - 1).Render(truncate(label, width-1)))
	}
	for line := range rowCount {
		rowPosition := grid.ScrollRow + line
		row := rowPosition
		if len(grid.VisibleRowIndexes) > 0 {
			if rowPosition >= len(grid.VisibleRowIndexes) {
				break
			}
			row = grid.VisibleRowIndexes[rowPosition]
		}
		output.WriteByte('\n')
		output.WriteString(rowHeader.Width(grid.RowHeaderWidth - 1).Render(padRight(itoa(row+1), grid.RowHeaderWidth-1)))
		for _, col := range columns {
			width := grid.columnWidth(col, sheet)
			coord := workbook.Coord{Row: row, Col: col}
			cell := sheet.Cells[coord]
			value := cell.Input
			if strings.HasPrefix(value, "=") && cell.CalculationState == workbook.Current {
				value = cell.Result
			}
			effective := workbook.EffectiveStyle(sheet, coord)
			style := lipgloss.NewStyle().Bold(effective.Bold)
			if effective.FontRGB != "" {
				if color, err := ParseRGBHex("#" + effective.FontRGB); err == nil {
					style = style.Foreground(TerminalColor(color, supportsTrueColor()))
				}
			}
			if effective.FillRGB != "" {
				if color, err := ParseRGBHex("#" + effective.FillRGB); err == nil {
					style = style.Background(TerminalColor(color, supportsTrueColor()))
				}
			}
			if effective.Borders.Bottom.Preset != "" && effective.Borders.Bottom.Preset != "none" {
				style = style.Underline(true)
			}
			if coordInRange(coord, grid.Selection) {
				style = style.Reverse(true)
			}
			leftGlyph, rightGlyph := " ", ""
			if effective.Borders.Left.Preset != "" && effective.Borders.Left.Preset != "none" {
				leftGlyph = "│"
			}
			if effective.Borders.Right.Preset != "" && effective.Borders.Right.Preset != "none" {
				rightGlyph = "│"
			}
			innerWidth := max(width-lipgloss.Width(leftGlyph)-lipgloss.Width(rightGlyph), 1)
			output.WriteString(leftGlyph)
			output.WriteString(style.Width(innerWidth).Render(truncate(value, innerWidth)))
			output.WriteString(rightGlyph)
		}
	}
	return output.String()
}

func coordInRange(coord workbook.Coord, selection workbook.CellRange) bool {
	return coord.Row >= selection.Start.Row && coord.Row <= selection.End.Row &&
		coord.Col >= selection.Start.Col && coord.Col <= selection.End.Col
}

func (grid *GridViewport) visibleColumns(sheet *workbook.Sheet) []int {
	if grid == nil || sheet == nil {
		return nil
	}
	available := max(grid.Width-grid.RowHeaderWidth, 0)
	columns := make([]int, 0)
	for col, used := grid.ScrollCol, 0; used < available; col++ {
		width := grid.columnWidth(col, sheet)
		if used+width > available {
			break
		}
		columns = append(columns, col)
		used += width
	}
	return columns
}

func columnName(column int) string {
	var label [8]byte
	position := len(label)
	for column++; column > 0; column = (column - 1) / 26 {
		position--
		label[position] = byte('A' + (column-1)%26)
	}
	return string(label[position:])
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	var output strings.Builder
	used := 0
	for _, character := range value {
		characterWidth := lipgloss.Width(string(character))
		if used+characterWidth > width-1 {
			break
		}
		output.WriteRune(character)
		used += characterWidth
	}
	output.WriteRune('…')
	return output.String()
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
