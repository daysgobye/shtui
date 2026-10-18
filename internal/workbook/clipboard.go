package workbook

import "fmt"

// CopyRange returns raw cell input, including formula source, for a rectangular selection.
func CopyRange(session *Session, sheetName string, selection CellRange) ([][]string, error) {
	if session == nil || session.Workbook == nil {
		return nil, fmt.Errorf("copy range: session is nil")
	}
	idx, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return nil, fmt.Errorf("copy range: sheet %q does not exist", sheetName)
	}
	if err := validateCellRange(selection); err != nil {
		return nil, fmt.Errorf("copy range: %w", err)
	}
	height := selection.End.Row - selection.Start.Row + 1
	width := selection.End.Col - selection.Start.Col + 1
	result := make([][]string, height)
	sheet := &session.Workbook.Sheets[idx]
	for rowOffset := range height {
		result[rowOffset] = make([]string, width)
		for colOffset := range width {
			coord := Coord{Row: selection.Start.Row + rowOffset, Col: selection.Start.Col + colOffset}
			if cell, exists := sheet.Cells[coord]; exists {
				result[rowOffset][colOffset] = cell.Input
			}
		}
	}
	return result, nil
}

// PasteRange writes a rectangular matrix starting at address; missing ragged fields become blank.
func PasteRange(session *Session, address CellAddress, rows [][]string) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("paste range: session is nil")
	}
	if address.Row < 0 || address.Col < 0 {
		return fmt.Errorf("paste range: negative coordinate (%d,%d)", address.Row, address.Col)
	}
	if len(rows) == 0 {
		return nil
	}
	idx, exists := sheetIndex(session.Workbook, address.Sheet)
	if !exists {
		return fmt.Errorf("paste range: sheet %q does not exist", address.Sheet)
	}
	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	if width == 0 {
		return nil
	}
	before := make(map[Coord]cellState, len(rows)*width)
	after := make(map[Coord]cellState, len(rows)*width)
	sheet := &session.Workbook.Sheets[idx]
	for rowOffset := range len(rows) {
		for colOffset := range width {
			coord := Coord{Row: address.Row + rowOffset, Col: address.Col + colOffset}
			old := cellStateAt(sheet, coord)
			input := ""
			if colOffset < len(rows[rowOffset]) {
				input = rows[rowOffset][colOffset]
			}
			before[coord] = old
			after[coord] = stateForInput(old, input)
		}
	}
	if err := applyCellStates(session, address.Sheet, after); err != nil {
		_ = applyCellStates(session, address.Sheet, before)
		return err
	}
	recordCellHistory(session, address.Sheet, before, after)
	return nil
}

func validateCellRange(selection CellRange) error {
	if selection.Start.Row < 0 || selection.Start.Col < 0 || selection.End.Row < selection.Start.Row || selection.End.Col < selection.Start.Col {
		return fmt.Errorf("invalid inclusive range (%d,%d)-(%d,%d)", selection.Start.Row, selection.Start.Col, selection.End.Row, selection.End.Col)
	}
	return nil
}
