package workbook

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type MergeResult string

const (
	Merged               MergeResult = "merged"
	ConfirmationRequired MergeResult = "confirmation-required"
)

// ApplyStyle applies one direct style to every cell in an inclusive range.
func ApplyStyle(session *Session, sheetName string, selection CellRange, style CellStyle) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("apply style: session is nil")
	}
	if err := validateCellRange(selection); err != nil {
		return fmt.Errorf("apply style: %w", err)
	}
	idx, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return fmt.Errorf("apply style: sheet %q does not exist", sheetName)
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	if _, err := styleToExcelize(style); err != nil {
		return err
	}
	before := make(map[Coord]cellState)
	after := make(map[Coord]cellState)
	sheet := &session.Workbook.Sheets[idx]
	for row := selection.Start.Row; row <= selection.End.Row; row++ {
		for col := selection.Start.Col; col <= selection.End.Col; col++ {
			coord := Coord{Row: row, Col: col}
			old := cellStateAt(sheet, coord)
			updated := old.cell
			updated.Style = style
			before[coord] = old
			after[coord] = cellState{cell: updated, exists: updated.Input != "" || hasCellStyle(updated.Style)}
		}
	}
	if err := applyStyledCellStates(session, sheetName, after); err != nil {
		_ = applyStyledCellStates(session, sheetName, before)
		return err
	}
	start, end, err := rangeCellNames(selection)
	if err != nil {
		return err
	}
	styleID, err := newExcelizeStyle(session.file, style)
	if err != nil {
		return err
	}
	if err := session.file.SetCellStyle(sheetName, start, end, styleID); err != nil {
		return fmt.Errorf("apply style to %s!%s:%s: %w", sheetName, start, end, err)
	}
	recordHistory(session, historyEntry{
		undo: func() error { return applyStyledCellStates(session, sheetName, before) },
		redo: func() error { return applyStyledCellStates(session, sheetName, after) },
	})
	return nil
}

// ApplyBorderColor applies thin borders with one color to all edges of a range.
func ApplyBorderColor(session *Session, sheetName string, selection CellRange, colorRGB string) error {
	thin := BorderStyle{Preset: "thin", ColorRGB: colorRGB}
	return applyBorderSet(session, sheetName, selection, BorderSet{
		Top: thin, Right: thin, Bottom: thin, Left: thin,
	})
}

// RemoveBorders clears all border edges in a range without replacing other cell formatting.
func RemoveBorders(session *Session, sheetName string, selection CellRange) error {
	return applyBorderSet(session, sheetName, selection, BorderSet{})
}
// ApplyBorderPreset keeps existing callers working until the UI preset commands are removed.
func ApplyBorderPreset(session *Session, sheetName string, selection CellRange, preset string) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("apply border preset: session is nil")
	}
	if err := validateCellRange(selection); err != nil {
		return fmt.Errorf("apply border preset: %w", err)
	}
	preset = strings.ToLower(strings.TrimSpace(preset))
	switch preset {
	case "all", "outside", "top", "bottom", "left", "right", "none":
	default:
		return fmt.Errorf("apply border preset: unsupported preset %q", preset)
	}
	idx, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return fmt.Errorf("apply border preset: sheet %q does not exist", sheetName)
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	before := make(map[Coord]cellState)
	after := make(map[Coord]cellState)
	sheet := &session.Workbook.Sheets[idx]
	thin := BorderStyle{Preset: "thin"}
	for row := selection.Start.Row; row <= selection.End.Row; row++ {
		for col := selection.Start.Col; col <= selection.End.Col; col++ {
			coord := Coord{Row: row, Col: col}
			old := cellStateAt(sheet, coord)
			updated := old.cell
			borders := updated.Style.Borders
			switch preset {
			case "all":
				borders = BorderSet{Top: thin, Right: thin, Bottom: thin, Left: thin}
			case "outside":
				borders = BorderSet{}
				if row == selection.Start.Row {
					borders.Top = thin
				}
				if row == selection.End.Row {
					borders.Bottom = thin
				}
				if col == selection.Start.Col {
					borders.Left = thin
				}
				if col == selection.End.Col {
					borders.Right = thin
				}
			case "top":
				borders.Top = thin
			case "bottom":
				borders.Bottom = thin
			case "left":
				borders.Left = thin
			case "right":
				borders.Right = thin
			case "none":
				borders = BorderSet{}
			}
			updated.Style.Borders = borders
			before[coord] = old
			after[coord] = cellState{cell: updated, exists: updated.Input != "" || hasCellStyle(updated.Style)}
		}
	}
	if err := applyStyledCellStates(session, sheetName, after); err != nil {
		_ = applyStyledCellStates(session, sheetName, before)
		return err
	}
	recordHistory(session, historyEntry{
		undo: func() error { return applyStyledCellStates(session, sheetName, before) },
		redo: func() error { return applyStyledCellStates(session, sheetName, after) },
	})
	return nil
}


func applyBorderSet(session *Session, sheetName string, selection CellRange, borders BorderSet) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("apply borders: session is nil")
	}
	if err := validateCellRange(selection); err != nil {
		return fmt.Errorf("apply borders: %w", err)
	}
	idx, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return fmt.Errorf("apply borders: sheet %q does not exist", sheetName)
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	before := make(map[Coord]cellState)
	after := make(map[Coord]cellState)
	sheet := &session.Workbook.Sheets[idx]
	for row := selection.Start.Row; row <= selection.End.Row; row++ {
		for col := selection.Start.Col; col <= selection.End.Col; col++ {
			coord := Coord{Row: row, Col: col}
			old := cellStateAt(sheet, coord)
			updated := old.cell
			updated.Style.Borders = borders
			before[coord] = old
			after[coord] = cellState{cell: updated, exists: updated.Input != "" || hasCellStyle(updated.Style)}
		}
	}
	if err := applyStyledCellStates(session, sheetName, after); err != nil {
		_ = applyStyledCellStates(session, sheetName, before)
		return err
	}
	recordHistory(session, historyEntry{
		undo: func() error { return applyStyledCellStates(session, sheetName, before) },
		redo: func() error { return applyStyledCellStates(session, sheetName, after) },
	})
	return nil
}

func applyStyledCellStates(session *Session, sheetName string, states map[Coord]cellState) error {
	if err := applyCellStates(session, sheetName, states); err != nil {
		return err
	}
	styleIDs := make(map[CellStyle]int)
	for coord, state := range states {
		styleID, exists := styleIDs[state.cell.Style]
		if !exists {
			var err error
			if state.cell.Style == (CellStyle{}) {
				styleID = 0
			} else {
				styleID, err = newExcelizeStyle(session.file, state.cell.Style)
				if err != nil {
					return err
				}
			}
			styleIDs[state.cell.Style] = styleID
		}
		address, err := excelize.CoordinatesToCellName(coord.Col+1, coord.Row+1)
		if err != nil {
			return fmt.Errorf("address style cell: %w", err)
		}
		if err := session.file.SetCellStyle(sheetName, address, address, styleID); err != nil {
			return fmt.Errorf("restore style %s!%s: %w", sheetName, address, err)
		}
	}
	return nil
}

// SetColumnWidth applies a positive width to a zero-based column index.
func SetColumnWidth(session *Session, sheetName string, col int, width float64) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("set column width: session is nil")
	}
	if col < 0 || width <= 0 || width > 255 || math.IsNaN(width) || math.IsInf(width, 0) {
		return fmt.Errorf("set column width: column must be nonnegative and width must be in (0,255]")
	}
	idx, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return fmt.Errorf("set column width: sheet %q does not exist", sheetName)
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	sheet := &session.Workbook.Sheets[idx]
	previous, hadWidth := sheet.ColumnWidths[col]
	columnName, err := excelize.ColumnNumberToName(col + 1)
	if err != nil {
		return fmt.Errorf("address column %d: %w", col, err)
	}
	previousExcelWidth, err := session.file.GetColWidth(sheetName, columnName)
	if err != nil {
		return fmt.Errorf("read prior width for %s!%s: %w", sheetName, columnName, err)
	}
	if sheet.ColumnWidths == nil {
		sheet.ColumnWidths = make(map[int]float64)
	}
	if err := setColumnWidthRaw(session, sheetName, col, width); err != nil {
		return err
	}
	recordHistory(session, historyEntry{
		undo: func() error {
			if hadWidth {
				return setColumnWidthRaw(session, sheetName, col, previous)
			}
			return clearColumnWidthRaw(session, sheetName, col, previousExcelWidth)
		},
		redo: func() error { return setColumnWidthRaw(session, sheetName, col, width) },
	})
	return nil
}

func setColumnWidthRaw(session *Session, sheetName string, col int, width float64) error {
	name, err := excelize.ColumnNumberToName(col + 1)
	if err != nil {
		return fmt.Errorf("address column %d: %w", col, err)
	}
	if err := session.file.SetColWidth(sheetName, name, name, width); err != nil {
		return fmt.Errorf("set width for %s!%s: %w", sheetName, name, err)
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	if session.Workbook.Sheets[idx].ColumnWidths == nil {
		session.Workbook.Sheets[idx].ColumnWidths = make(map[int]float64)
	}
	session.Workbook.Sheets[idx].ColumnWidths[col] = width
	session.Workbook.Dirty = true
	return nil
}

func clearColumnWidthRaw(session *Session, sheetName string, col int, width float64) error {
	name, err := excelize.ColumnNumberToName(col + 1)
	if err != nil {
		return fmt.Errorf("address column %d: %w", col, err)
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	if err := session.file.SetColWidth(sheetName, name, name, width); err != nil {
		return fmt.Errorf("restore prior width for %s!%s: %w", sheetName, name, err)
	}
	delete(session.Workbook.Sheets[idx].ColumnWidths, col)
	session.Workbook.Dirty = true
	return nil
}

// MergeRange merges a range, requiring confirmation before discarding non-top-left values.
func MergeRange(session *Session, sheetName string, selection CellRange, confirmDiscard bool) (MergeResult, error) {
	if session == nil || session.Workbook == nil {
		return "", fmt.Errorf("merge range: session is nil")
	}
	if err := validateCellRange(selection); err != nil {
		return "", fmt.Errorf("merge range: %w", err)
	}
	idx, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return "", fmt.Errorf("merge range: sheet %q does not exist", sheetName)
	}
	if selection.Start == selection.End {
		return Merged, nil
	}
	sheet := &session.Workbook.Sheets[idx]
	for _, merged := range sheet.Merges {
		if rangesOverlap(selection, merged) {
			return "", fmt.Errorf("merge range overlaps existing merged cells")
		}
	}
	for row := selection.Start.Row; row <= selection.End.Row; row++ {
		for col := selection.Start.Col; col <= selection.End.Col; col++ {
			if row == selection.Start.Row && col == selection.Start.Col {
				continue
			}
			if sheet.Cells[Coord{Row: row, Col: col}].Input != "" && !confirmDiscard {
				return ConfirmationRequired, nil
			}
		}
	}
	if err := ensureCalculationFile(session); err != nil {
		return "", err
	}
	before := snapshotRangeCells(sheet, selection)
	beforeMerges := append([]CellRange(nil), sheet.Merges...)
	if err := mergeRangeRaw(session, sheetName, selection); err != nil {
		return "", err
	}
	afterSheetIndex, _ := sheetIndex(session.Workbook, sheetName)
	after := snapshotRangeCells(&session.Workbook.Sheets[afterSheetIndex], selection)
	afterMerges := append([]CellRange(nil), session.Workbook.Sheets[afterSheetIndex].Merges...)
	recordHistory(session, historyEntry{
		undo: func() error {
			if err := unmergeRangeRaw(session, sheetName, selection); err != nil {
				return err
			}
			idx, _ := sheetIndex(session.Workbook, sheetName)
			session.Workbook.Sheets[idx].Merges = append([]CellRange(nil), beforeMerges...)
			return applyStyledCellStates(session, sheetName, before)
		},
		redo: func() error {
			if err := mergeRangeRaw(session, sheetName, selection); err != nil {
				return err
			}
			idx, _ := sheetIndex(session.Workbook, sheetName)
			session.Workbook.Sheets[idx].Merges = append([]CellRange(nil), afterMerges...)
			return applyStyledCellStates(session, sheetName, after)
		},
	})
	return Merged, nil
}

func mergeRangeRaw(session *Session, sheetName string, selection CellRange) error {
	start, end, err := rangeCellNames(selection)
	if err != nil {
		return err
	}
	if err := session.file.MergeCell(sheetName, start, end); err != nil {
		return fmt.Errorf("merge %s!%s:%s: %w", sheetName, start, end, err)
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	sheet := &session.Workbook.Sheets[idx]
	for row := selection.Start.Row; row <= selection.End.Row; row++ {
		for col := selection.Start.Col; col <= selection.End.Col; col++ {
			coord := Coord{Row: row, Col: col}
			if coord == selection.Start {
				continue
			}
			cell, exists := sheet.Cells[coord]
			if !exists {
				continue
			}
			cell.Input = ""
			cell.Result = ""
			cell.CalculationState = Literal
			if hasCellStyle(cell.Style) {
				sheet.Cells[coord] = cell
			} else {
				delete(sheet.Cells, coord)
			}
		}
	}
	sheet.Merges = append(sheet.Merges, selection)
	session.Workbook.Dirty = true
	return nil
}

func unmergeRangeRaw(session *Session, sheetName string, selection CellRange) error {
	start, end, err := rangeCellNames(selection)
	if err != nil {
		return err
	}
	if err := session.file.UnmergeCell(sheetName, start, end); err != nil {
		return fmt.Errorf("unmerge %s!%s:%s: %w", sheetName, start, end, err)
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	sheet := &session.Workbook.Sheets[idx]
	for index, merged := range sheet.Merges {
		if merged == selection {
			sheet.Merges = append(sheet.Merges[:index], sheet.Merges[index+1:]...)
			break
		}
	}
	session.Workbook.Dirty = true
	return nil
}

// UnmergeRange removes merge metadata for an exact merged range.
func UnmergeRange(session *Session, sheetName string, selection CellRange) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("unmerge range: session is nil")
	}
	if err := validateCellRange(selection); err != nil {
		return fmt.Errorf("unmerge range: %w", err)
	}
	idx, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return fmt.Errorf("unmerge range: sheet %q does not exist", sheetName)
	}
	sheet := &session.Workbook.Sheets[idx]
	if !containsMerge(sheet.Merges, selection) {
		return fmt.Errorf("unmerge range: %s is not a merged range", rangeDescription(selection))
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	before := append([]CellRange(nil), sheet.Merges...)
	if err := unmergeRangeRaw(session, sheetName, selection); err != nil {
		return err
	}
	after := append([]CellRange(nil), session.Workbook.Sheets[idx].Merges...)
	recordHistory(session, historyEntry{
		undo: func() error {
			if err := mergeRangeRaw(session, sheetName, selection); err != nil {
				return err
			}
			idx, _ := sheetIndex(session.Workbook, sheetName)
			session.Workbook.Sheets[idx].Merges = append([]CellRange(nil), before...)
			return nil
		},
		redo: func() error {
			if err := unmergeRangeRaw(session, sheetName, selection); err != nil {
				return err
			}
			idx, _ := sheetIndex(session.Workbook, sheetName)
			session.Workbook.Sheets[idx].Merges = append([]CellRange(nil), after...)
			return nil
		},
	})
	return nil
}

func snapshotRangeCells(sheet *Sheet, selection CellRange) map[Coord]cellState {
	states := make(map[Coord]cellState)
	for row := selection.Start.Row; row <= selection.End.Row; row++ {
		for col := selection.Start.Col; col <= selection.End.Col; col++ {
			coord := Coord{Row: row, Col: col}
			states[coord] = cellStateAt(sheet, coord)
		}
	}
	return states
}

func rangeCellNames(selection CellRange) (string, string, error) {
	start, err := excelize.CoordinatesToCellName(selection.Start.Col+1, selection.Start.Row+1)
	if err != nil {
		return "", "", fmt.Errorf("address range start: %w", err)
	}
	end, err := excelize.CoordinatesToCellName(selection.End.Col+1, selection.End.Row+1)
	if err != nil {
		return "", "", fmt.Errorf("address range end: %w", err)
	}
	return start, end, nil
}

func rangesOverlap(left, right CellRange) bool {
	return left.Start.Row <= right.End.Row && right.Start.Row <= left.End.Row && left.Start.Col <= right.End.Col && right.Start.Col <= left.End.Col
}

func containsMerge(merges []CellRange, selection CellRange) bool {
	for _, merged := range merges {
		if merged == selection {
			return true
		}
	}
	return false
}

func rangeDescription(selection CellRange) string {
	start, end, _ := rangeCellNames(selection)
	return start + ":" + end
}

func newExcelizeStyle(file *excelize.File, style CellStyle) (int, error) {
	converted, err := styleToExcelize(style)
	if err != nil {
		return 0, err
	}
	return file.NewStyle(&converted)
}

func styleToExcelize(style CellStyle) (excelize.Style, error) {
	var converted excelize.Style
	if style.FillRGB != "" {
		color, err := normalizeRGB(style.FillRGB)
		if err != nil {
			return converted, err
		}
		converted.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{color}}
	}
	if style.FontRGB != "" || style.Bold {
		font := &excelize.Font{Bold: style.Bold}
		if style.FontRGB != "" {
			color, err := normalizeRGB(style.FontRGB)
			if err != nil {
				return converted, err
			}
			font.Color = color
		}
		converted.Font = font
	}
	if style.NumberFormat != "" {
		if strings.HasPrefix(style.NumberFormat, "builtin:") {
			numberFormat, err := strconv.Atoi(strings.TrimPrefix(style.NumberFormat, "builtin:"))
			if err != nil || numberFormat <= 0 {
				return converted, fmt.Errorf("invalid built-in number format %q", style.NumberFormat)
			}
			converted.NumFmt = numberFormat
		} else {
			format := style.NumberFormat
			converted.CustomNumFmt = &format
		}
	}
	for _, border := range []struct {
		edge  string
		style BorderStyle
	}{
		{edge: "top", style: style.Borders.Top},
		{edge: "right", style: style.Borders.Right},
		{edge: "bottom", style: style.Borders.Bottom},
		{edge: "left", style: style.Borders.Left},
	} {
		if border.style.Preset == "" || border.style.Preset == "none" {
			continue
		}
		width, exists := excelBorderStyles[strings.ToLower(border.style.Preset)]
		if !exists {
			return converted, fmt.Errorf("unsupported border preset %q", border.style.Preset)
		}
		color := "000000"
		if border.style.ColorRGB != "" {
			var err error
			color, err = normalizeRGB(border.style.ColorRGB)
			if err != nil {
				return converted, err
			}
		}
		converted.Border = append(converted.Border, excelize.Border{Type: border.edge, Color: color, Style: width})
	}
	return converted, nil
}

var excelBorderStyles = map[string]int{
	"thin": 1, "medium": 2, "dashed": 3, "dotted": 4, "thick": 5, "double": 6, "hair": 7,
	"mediumdashed": 8, "dashdot": 9, "mediumdashdot": 10, "dashdotdot": 11, "mediumdashdotdot": 12, "slantdashdot": 13,
}

func borderPreset(style int) string {
	for name, value := range excelBorderStyles {
		if value == style {
			return name
		}
	}
	return ""
}

func styleFromExcelize(style *excelize.Style) CellStyle {
	if style == nil {
		return CellStyle{}
	}
	result := CellStyle{}
	if len(style.Fill.Color) > 0 {
		result.FillRGB = normalizeStoredColor(style.Fill.Color[0])
	}
	if style.Font != nil {
		result.FontRGB = normalizeStoredColor(style.Font.Color)
		result.Bold = style.Font.Bold
	}
	if style.CustomNumFmt != nil {
		result.NumberFormat = *style.CustomNumFmt
	} else if style.NumFmt != 0 {
		result.NumberFormat = fmt.Sprintf("builtin:%d", style.NumFmt)
	}
	for _, border := range style.Border {
		borderStyle := BorderStyle{Preset: borderPreset(border.Style), ColorRGB: normalizeStoredColor(border.Color)}
		switch strings.ToLower(border.Type) {
		case "top":
			result.Borders.Top = borderStyle
		case "right":
			result.Borders.Right = borderStyle
		case "bottom":
			result.Borders.Bottom = borderStyle
		case "left":
			result.Borders.Left = borderStyle
		}
	}
	return result
}

func normalizeStoredColor(color string) string {
	color = strings.TrimPrefix(strings.TrimSpace(color), "#")
	if len(color) == 8 {
		color = color[2:]
	}
	return strings.ToUpper(color)
}

func normalizeRGB(color string) (string, error) {
	value := normalizeStoredColor(color)
	if len(value) != 6 {
		return "", fmt.Errorf("color %q must be six-digit RGB", color)
	}
	for _, digit := range value {
		if !(digit >= '0' && digit <= '9' || digit >= 'A' && digit <= 'F') {
			return "", fmt.Errorf("color %q must be six-digit RGB", color)
		}
	}
	return value, nil
}
