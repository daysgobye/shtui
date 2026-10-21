package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"shtui/internal/workbook"
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
		HeaderHeight:   2,
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
	row := grid.ScrollRow + (y-grid.HeaderHeight)/2
	if len(grid.VisibleRowIndexes) > 0 {
		if row < 0 || row >= len(grid.VisibleRowIndexes) {
			return workbook.Coord{}, false
		}
		row = grid.VisibleRowIndexes[row]
	}
	return normalizeToMergeAnchor(sheet, workbook.Coord{Row: row, Col: col}), true
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
	for col := grid.ScrollCol; position <= x+1; col++ {
		position += grid.columnWidth(col, sheet)
		if x-position >= -1 && x-position <= 1 {
			return col, true
		}
		if position > x+1 {
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

func (grid *GridViewport) renderedHeight(height int) int {
	if grid == nil || height <= 0 {
		return 0
	}
	rowCount := max((height-grid.HeaderHeight)/2, 0)
	if len(grid.VisibleRowIndexes) > 0 {
		rowCount = min(rowCount, max(len(grid.VisibleRowIndexes)-grid.ScrollRow, 0))
	}
	return 2 + rowCount*2
}
func (grid *GridViewport) View(sheet *workbook.Sheet, height int) string {
	if grid == nil || sheet == nil || height <= 0 {
		return ""
	}
	styleResolver := workbook.NewEffectiveStyleResolver(sheet)
	var output strings.Builder
	columnHeader := lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6))
	rowHeader := lipgloss.NewStyle().Foreground(ANSIColor(8))
	columns := grid.visibleColumns(sheet)
	mergedLines := make(map[workbook.Coord][]string)
	output.WriteString(strings.Repeat(" ", grid.RowHeaderWidth-1))
	for _, col := range columns {
		output.WriteString("│")
		output.WriteString(columnHeader.Width(grid.columnWidth(col, sheet) - 1).Render(truncate(columnName(col), grid.columnWidth(col, sheet)-1)))
	}
	output.WriteByte('\n')
	output.WriteString(grid.horizontalBoundary(sheet, styleResolver, columns, -1, grid.ScrollRow, mergedLines))

	rowCount := max((height-grid.HeaderHeight)/2, 0)
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
		for index := 0; index < len(columns); {
			col := columns[index]
			merge, merged := mergeAt(sheet, workbook.Coord{Row: row, Col: col})
			spanWidth := grid.columnWidth(col, sheet)
			mergeStart := merged && col == merge.Start.Col
			if mergeStart {
				for next := index + 1; next < len(columns) && columns[next] <= merge.End.Col; next++ {
					spanWidth += grid.columnWidth(columns[next], sheet)
				}
			}
			cellCoord := workbook.Coord{Row: row, Col: col}
			if mergeStart && row == merge.Start.Row {
				mergedLines[cellCoord] = grid.wrapMergedText(sheet, merge, spanWidth)
			}
			output.WriteString(grid.verticalBoundary(sheet, styleResolver, row, columns, index))
			contentWidth := max(spanWidth-1, 1)
			switch {
			case mergeStart:
				value := mergedTextLine(mergedLines[merge.Start], 2*(row-merge.Start.Row))
				output.WriteString(grid.cellTextStyle(sheet, styleResolver, merge.Start).Width(contentWidth).Render(truncate(value, contentWidth)))
			case !merged:
				value := displayCellValue(sheet.Cells[cellCoord])
				output.WriteString(grid.cellTextStyle(sheet, styleResolver, cellCoord).Width(contentWidth).Render(truncate(value, contentWidth)))
			default:
				output.WriteString(strings.Repeat(" ", contentWidth))
			}
			if mergeStart {
				for index < len(columns) && columns[index] <= merge.End.Col {
					index++
				}
			} else {
				index++
			}
		}
		output.WriteByte('\n')
		output.WriteString(grid.horizontalBoundary(sheet, styleResolver, columns, row, row+1, mergedLines))
	}
	return output.String()
}

func (grid *GridViewport) wrapMergedText(sheet *workbook.Sheet, merge workbook.CellRange, spanWidth int) []string {
	contentWidth := max(spanWidth-1, 1)
	value := displayCellValue(sheet.Cells[merge.Start])
	var lines []string
	for _, line := range strings.Split(ansi.Wordwrap(value, contentWidth, " "), "\n") {
		if ansi.StringWidth(line) > contentWidth {
			lines = append(lines, strings.Split(ansi.Hardwrap(line, contentWidth, false), "\n")...)
		} else {
			lines = append(lines, line)
		}
	}
	maxLines := max(2*(merge.End.Row-merge.Start.Row+1)-1, 1)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = ansi.Truncate(lines[maxLines-1], max(contentWidth-1, 0), "") + "…"
	}
	return lines
}

func displayCellValue(cell workbook.Cell) string {
	if strings.HasPrefix(cell.Input, "=") && cell.CalculationState == workbook.Current {
		return cell.Result
	}
	return cell.Input
}

func mergedTextLine(lines []string, index int) string {
	if index < 0 || index >= len(lines) {
		return ""
	}
	return lines[index]
}

func (grid *GridViewport) cellTextStyle(sheet *workbook.Sheet, styleResolver *workbook.EffectiveStyleResolver, coord workbook.Coord) lipgloss.Style {
	cell := sheet.Cells[coord]
	style := lipgloss.NewStyle().Bold(cell.Style.Bold)
	effective := styleResolver.StyleAt(coord)
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
	if coordInRange(coord, grid.Selection) {
		style = style.Reverse(true)
	}
	return style
}

func (grid *GridViewport) horizontalBoundary(sheet *workbook.Sheet, styleResolver *workbook.EffectiveStyleResolver, columns []int, upperRow, lowerRow int, mergedLines map[workbook.Coord][]string) string {
	var line strings.Builder
	line.WriteString(strings.Repeat(" ", grid.RowHeaderWidth-1))
	for index := 0; index < len(columns); {
		col := columns[index]
		width := grid.columnWidth(col, sheet) - 1
		mergedAcrossRows := grid.mergedAcrossRows(sheet, upperRow, lowerRow, col)
		if mergedAcrossRows {
			merge, _ := mergeAt(sheet, workbook.Coord{Row: upperRow, Col: col})
			spanWidth := width + 1
			next := index + 1
			for next < len(columns) && columns[next] <= merge.End.Col {
				spanWidth += grid.columnWidth(columns[next], sheet)
				next++
			}
			contentWidth := max(spanWidth-1, 1)
			line.WriteByte(' ')
			if col == merge.Start.Col {
				value := mergedTextLine(mergedLines[merge.Start], 2*(upperRow-merge.Start.Row)+1)
				line.WriteString(grid.cellTextStyle(sheet, styleResolver, merge.Start).Width(contentWidth).Render(truncate(value, contentWidth)))
			} else {
				line.WriteString(strings.Repeat(" ", contentWidth))
			}
			index = next
			continue
		}
		border := grid.horizontalBorder(sheet, col, upperRow, lowerRow)
		fill := ""
		junctionFill := ""
		if upperRow >= 0 && upperRow < sheet.Rows && lowerRow >= 0 && lowerRow < sheet.Rows {
			fill = grid.sharedBoundaryFill(sheet, styleResolver,
				workbook.Coord{Row: upperRow, Col: col},
				workbook.Coord{Row: lowerRow, Col: col})
			if index > 0 {
				junctionFill = grid.junctionBoundaryFill(sheet, styleResolver,
					workbook.Coord{Row: upperRow, Col: columns[index-1]},
					workbook.Coord{Row: upperRow, Col: col},
					workbook.Coord{Row: lowerRow, Col: columns[index-1]},
					workbook.Coord{Row: lowerRow, Col: col})
			}
		}
		if index > 0 && grid.mergedAcrossColumns(sheet, lowerRow, columns[index-1], col) {
			line.WriteByte(' ')
		} else {
			line.WriteString(grid.styleBorderGlyph(border, junctionFill, "┼"))
		}
		line.WriteString(grid.horizontalStroke(border, width, fill))
		index++
	}
	return line.String()
}

func (grid *GridViewport) sharedBoundaryFill(sheet *workbook.Sheet, styleResolver *workbook.EffectiveStyleResolver, first, second workbook.Coord) string {
	firstFill := grid.boundaryFill(sheet, styleResolver, first)
	if firstFill == "" {
		return ""
	}
	secondFill := grid.boundaryFill(sheet, styleResolver, second)
	if strings.EqualFold(firstFill, secondFill) {
		return firstFill
	}
	return ""
}

func (grid *GridViewport) junctionBoundaryFill(sheet *workbook.Sheet, styleResolver *workbook.EffectiveStyleResolver, topLeft, topRight, bottomLeft, bottomRight workbook.Coord) string {
	fill := grid.boundaryFill(sheet, styleResolver, topLeft)
	if fill == "" {
		return ""
	}
	if strings.EqualFold(fill, grid.boundaryFill(sheet, styleResolver, topRight)) &&
		strings.EqualFold(fill, grid.boundaryFill(sheet, styleResolver, bottomLeft)) &&
		strings.EqualFold(fill, grid.boundaryFill(sheet, styleResolver, bottomRight)) {
		return fill
	}
	return ""
}

func (grid *GridViewport) horizontalBorder(sheet *workbook.Sheet, col, upperRow, lowerRow int) workbook.BorderStyle {
	border := workbook.BorderStyle{}
	if upperRow >= 0 && upperRow < sheet.Rows {
		border = sheet.Cells[workbook.Coord{Row: upperRow, Col: col}].Style.Borders.Bottom
	}
	if lowerRow >= 0 && lowerRow < sheet.Rows && border.Preset == "" {
		border = sheet.Cells[workbook.Coord{Row: lowerRow, Col: col}].Style.Borders.Top
	}
	return border
}

func (grid *GridViewport) verticalBoundary(sheet *workbook.Sheet, styleResolver *workbook.EffectiveStyleResolver, row int, columns []int, index int) string {
	col := columns[index]
	if index > 0 && grid.mergedAcrossColumns(sheet, row, columns[index-1], col) {
		return " "
	}
	border := sheet.Cells[workbook.Coord{Row: row, Col: col}].Style.Borders.Left
	fill := ""
	if index > 0 {
		previousCol := columns[index-1]
		previous := sheet.Cells[workbook.Coord{Row: row, Col: previousCol}].Style.Borders.Right
		if previous.Preset != "" && previous.Preset != "none" {
			border = previous
		}
		fill = grid.sharedBoundaryFill(sheet, styleResolver,
			workbook.Coord{Row: row, Col: previousCol},
			workbook.Coord{Row: row, Col: col})
	}
	return grid.verticalStroke(border, fill)
}

func (grid *GridViewport) boundaryFill(sheet *workbook.Sheet, styleResolver *workbook.EffectiveStyleResolver, coord workbook.Coord) string {
	if merge, ok := mergeAt(sheet, coord); ok {
		coord = merge.Start
	}
	return styleResolver.StyleAt(coord).FillRGB
}

func (grid *GridViewport) verticalStroke(border workbook.BorderStyle, fill string) string {
	glyph := "│"
	switch strings.ToLower(border.Preset) {
	case "thick", "medium":
		glyph = "┃"
	case "double":
		glyph = "║"
	case "dashed", "mediumdashed":
		glyph = "┆"
	case "dotted":
		glyph = "┊"
	}
	return grid.styleBorderGlyph(border, fill, glyph)
}

func (grid *GridViewport) horizontalStroke(border workbook.BorderStyle, width int, fill string) string {
	glyph := "─"
	switch strings.ToLower(border.Preset) {
	case "thick", "medium":
		glyph = "━"
	case "double":
		glyph = "═"
	case "dashed", "mediumdashed":
		glyph = "┄"
	case "dotted":
		glyph = "┈"
	}
	return grid.styleBorderGlyph(border, fill, strings.Repeat(glyph, max(width, 0)))
}

func (grid *GridViewport) styleBorderGlyph(border workbook.BorderStyle, fill, glyph string) string {
	style := lipgloss.NewStyle()
	styled := false
	if border.Preset != "" && border.Preset != "none" {
		if border.ColorRGB != "" {
			if color, err := ParseRGBHex("#" + border.ColorRGB); err == nil {
				style = style.Foreground(TerminalColor(color, supportsTrueColor()))
			} else {
				style = style.Foreground(ANSIColor(7))
			}
		} else {
			style = style.Foreground(ANSIColor(7))
		}
		styled = true
	}
	if fill != "" {
		if color, err := ParseRGBHex("#" + fill); err == nil {
			style = style.Background(TerminalColor(color, supportsTrueColor()))
			styled = true
		}
	}
	if !styled {
		return glyph
	}
	return style.Render(glyph)
}

func mergeAt(sheet *workbook.Sheet, coord workbook.Coord) (workbook.CellRange, bool) {
	if sheet == nil {
		return workbook.CellRange{}, false
	}
	for _, merge := range sheet.Merges {
		if coordInRange(coord, merge) {
			return merge, true
		}
	}
	return workbook.CellRange{}, false
}

func normalizeToMergeAnchor(sheet *workbook.Sheet, coord workbook.Coord) workbook.Coord {
	if merge, ok := mergeAt(sheet, coord); ok {
		return merge.Start
	}
	return coord
}

func (grid *GridViewport) mergedAcrossColumns(sheet *workbook.Sheet, row, leftCol, rightCol int) bool {
	if row < 0 {
		return false
	}
	merge, ok := mergeAt(sheet, workbook.Coord{Row: row, Col: rightCol})
	return ok && row >= merge.Start.Row && row <= merge.End.Row && leftCol >= merge.Start.Col && rightCol <= merge.End.Col
}

func (grid *GridViewport) mergedAcrossRows(sheet *workbook.Sheet, upperRow, lowerRow, col int) bool {
	if upperRow < 0 || lowerRow < 0 {
		return false
	}
	merge, ok := mergeAt(sheet, workbook.Coord{Row: upperRow, Col: col})
	return ok && lowerRow <= merge.End.Row && col >= merge.Start.Col && col <= merge.End.Col
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
