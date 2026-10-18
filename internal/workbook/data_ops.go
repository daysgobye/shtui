package workbook

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ColumnFilter hides rows whose cell in the relative column is not allowed.
type ColumnFilter struct {
	Column        int
	AllowedValues map[string]bool
	Range         CellRange
}

// SortRange stably sorts whole rows in a selected range by one relative column.
func SortRange(session *Session, sheetName string, selection CellRange, sortCol int, ascending, hasHeader bool) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("sort range: session is nil")
	}
	if err := validateCellRange(selection); err != nil {
		return fmt.Errorf("sort range: %w", err)
	}
	idx, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return fmt.Errorf("sort range: sheet %q does not exist", sheetName)
	}
	width := selection.End.Col - selection.Start.Col + 1
	if sortCol < 0 || sortCol >= width {
		return fmt.Errorf("sort column %d is outside the selected range", sortCol)
	}
	sheet := &session.Workbook.Sheets[idx]
	for _, merged := range sheet.Merges {
		if rangesOverlap(selection, merged) {
			return fmt.Errorf("cannot sort range %s because it overlaps merged cells %s", rangeDescription(selection), rangeDescription(merged))
		}
	}
	firstRow := selection.Start.Row
	if hasHeader {
		firstRow++
	}
	if firstRow > selection.End.Row {
		return nil
	}
	rows := make([][]cellState, selection.End.Row-firstRow+1)
	before := make(map[Coord]cellState, (selection.End.Row-selection.Start.Row+1)*width)
	for offset := range rows {
		row := firstRow + offset
		rows[offset] = make([]cellState, width)
		for colOffset := range width {
			coord := Coord{Row: row, Col: selection.Start.Col + colOffset}
			state := cellStateAt(sheet, coord)
			rows[offset][colOffset] = state
			before[coord] = state
		}
	}
	sort.SliceStable(rows, func(left, right int) bool {
		leftValue := sortCellValue(rows[left][sortCol].cell)
		rightValue := sortCellValue(rows[right][sortCol].cell)
		less := compareSortValues(leftValue, rightValue)
		if ascending {
			return less
		}
		return compareSortValues(rightValue, leftValue)
	})
	after := make(map[Coord]cellState, len(before))
	for offset, row := range rows {
		rowIndex := firstRow + offset
		for colOffset, state := range row {
			after[Coord{Row: rowIndex, Col: selection.Start.Col + colOffset}] = state
		}
	}
	if err := applyCellStates(session, sheetName, after); err != nil {
		_ = applyCellStates(session, sheetName, before)
		return fmt.Errorf("sort range %s: %w", rangeDescription(selection), err)
	}
	recordHistory(session, historyEntry{
		undo: func() error { return applyCellStates(session, sheetName, before) },
		redo: func() error { return applyCellStates(session, sheetName, after) },
	})
	return nil
}

func compareSortValues(left, right string) bool {
	leftNumber, leftErr := strconv.ParseFloat(strings.TrimSpace(left), 64)
	rightNumber, rightErr := strconv.ParseFloat(strings.TrimSpace(right), 64)
	if leftErr == nil && rightErr == nil {
		return leftNumber < rightNumber
	}
	if leftErr == nil {
		return true
	}
	if rightErr == nil {
		return false
	}
	return strings.ToLower(left) < strings.ToLower(right)
}

func sortCellValue(cell Cell) string {
	if strings.HasPrefix(cell.Input, "=") && cell.CalculationState == Current {
		return cell.Result
	}
	return cell.Input
}

// Find returns row-major substring matches from exactly the requested sheet.
func Find(session *Session, sheetName, query string) []CellAddress {
	if session == nil || session.Workbook == nil || query == "" {
		return nil
	}
	idx, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return nil
	}
	needle := strings.ToLower(query)
	matches := make([]CellAddress, 0)
	for coord, cell := range session.Workbook.Sheets[idx].Cells {
		if strings.Contains(strings.ToLower(sortCellValue(cell)), needle) {
			matches = append(matches, CellAddress{Sheet: sheetName, Row: coord.Row, Col: coord.Col})
		}
	}
	sort.Slice(matches, func(left, right int) bool {
		if matches[left].Row != matches[right].Row {
			return matches[left].Row < matches[right].Row
		}
		return matches[left].Col < matches[right].Col
	})
	return matches
}

// SetFilter stores row filters and writes matching XLSX AutoFilter metadata.
func SetFilter(session *Session, sheetName string, selection CellRange, filters []ColumnFilter) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("set filter: session is nil")
	}
	if err := validateCellRange(selection); err != nil {
		return fmt.Errorf("set filter: %w", err)
	}
	if _, exists := sheetIndex(session.Workbook, sheetName); !exists {
		return fmt.Errorf("set filter: sheet %q does not exist", sheetName)
	}
	stored := make([]ColumnFilter, len(filters))
	seenColumns := make(map[int]struct{}, len(filters))
	options := make([]excelize.AutoFilterOptions, 0, len(filters))
	for index, filter := range filters {
		if filter.Column < 0 || filter.Column > selection.End.Col-selection.Start.Col {
			return fmt.Errorf("filter column %d is outside the selected range", filter.Column)
		}
		if len(filter.AllowedValues) == 0 {
			return fmt.Errorf("filter column %d must allow at least one value", filter.Column)
		}
		if _, duplicate := seenColumns[filter.Column]; duplicate {
			return fmt.Errorf("filter column %d is specified more than once", filter.Column)
		}
		seenColumns[filter.Column] = struct{}{}
		filter.Range = selection
		filter.AllowedValues = cloneAllowedValues(filter.AllowedValues)
		stored[index] = filter
		columnName, err := excelize.ColumnNumberToName(selection.Start.Col + filter.Column + 1)
		if err != nil {
			return fmt.Errorf("address filter column %d: %w", filter.Column, err)
		}
		values := make([]string, 0, len(filter.AllowedValues))
		for value, allowed := range filter.AllowedValues {
			if allowed {
				values = append(values, escapeFilterValue(value))
			}
		}
		if len(values) == 0 {
			return fmt.Errorf("filter column %d must allow at least one value", filter.Column)
		}
		sort.Strings(values)
		expression := make([]string, len(values))
		for valueIndex, value := range values {
			expression[valueIndex] = "x == " + value
		}
		options = append(options, excelize.AutoFilterOptions{Column: columnName, Expression: strings.Join(expression, " or ")})
	}
	if session.Format != CSV {
		if session.file == nil {
			return fmt.Errorf("set filter: XLSX session has no open workbook")
		}
		start, end, err := rangeCellNames(selection)
		if err != nil {
			return err
		}
		if err := session.file.AutoFilter(sheetName, start+":"+end, options); err != nil {
			return fmt.Errorf("set AutoFilter on %s!%s:%s: %w", sheetName, start, end, err)
		}
		session.Workbook.Dirty = true
	}
	if session.Filters == nil {
		session.Filters = make(map[string][]ColumnFilter)
	}
	if len(stored) == 0 {
		delete(session.Filters, sheetName)
	} else {
		session.Filters[sheetName] = stored
	}
	return nil
}

func cloneAllowedValues(values map[string]bool) map[string]bool {
	copy := make(map[string]bool, len(values))
	for value, allowed := range values {
		copy[value] = allowed
	}
	return copy
}

func escapeFilterValue(value string) string {
	value = strings.ReplaceAll(value, "~", "~~")
	value = strings.ReplaceAll(value, "*", "~*")
	value = strings.ReplaceAll(value, "?", "~?")
	return strconv.Quote(value)
}

// VisibleRows lists worksheet row indexes that remain visible under all filters.
func VisibleRows(sheet *Sheet, filters []ColumnFilter) []int {
	if sheet == nil || sheet.Rows <= 0 {
		return nil
	}
	visible := make([]int, 0, sheet.Rows)
	for row := range sheet.Rows {
		show := true
		for _, filter := range filters {
			selection := filter.Range
			if row < selection.Start.Row || row > selection.End.Row || row == selection.Start.Row {
				continue
			}
			column := selection.Start.Col + filter.Column
			if column < 0 || column > selection.End.Col {
				show = false
				break
			}
			value := sortCellValue(sheet.Cells[Coord{Row: row, Col: column}])
			if !filter.AllowedValues[value] {
				allowed := false
				for candidate, include := range filter.AllowedValues {
					if include && strings.EqualFold(value, candidate) {
						allowed = true
						break
					}
				}
				if !allowed {
					show = false
					break
				}
			}
		}
		if show {
			visible = append(visible, row)
		}
	}
	return visible
}
