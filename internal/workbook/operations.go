package workbook

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type cellState struct {
	cell   Cell
	exists bool
}

type sessionSnapshot struct {
	data        []byte
	activeSheet int
}

// SetCell updates one cell and recalculates dependent formulas.
func SetCell(session *Session, address CellAddress, input string) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("set cell: session is nil")
	}
	if address.Row < 0 || address.Col < 0 {
		return fmt.Errorf("set cell: negative coordinate (%d,%d)", address.Row, address.Col)
	}
	sheetIndex, exists := sheetIndex(session.Workbook, address.Sheet)
	if !exists {
		return fmt.Errorf("set cell: sheet %q does not exist", address.Sheet)
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	sheet := &session.Workbook.Sheets[sheetIndex]
	coord := Coord{Row: address.Row, Col: address.Col}
	before := cellStateAt(sheet, coord)
	after := stateForInput(before, input)
	if before == after {
		return nil
	}
	changesBefore := map[Coord]cellState{coord: before}
	changesAfter := map[Coord]cellState{coord: after}
	if err := applyCellStates(session, address.Sheet, changesAfter); err != nil {
		_ = applyCellStates(session, address.Sheet, changesBefore)
		return err
	}
	recordCellHistory(session, address.Sheet, changesBefore, changesAfter)
	return nil
}

func stateForInput(previous cellState, input string) cellState {
	cell := previous.cell
	cell.Input = input
	if input == "" {
		cell.Result = ""
		cell.CalculationState = Literal
		return cellState{cell: cell, exists: hasCellStyle(cell.Style)}
	}
	if strings.HasPrefix(input, "=") {
		cell.CalculationState = Stale
		return cellState{cell: cell, exists: true}
	}
	cell.Result = input
	cell.CalculationState = Literal
	return cellState{cell: cell, exists: true}
}

func hasCellStyle(style CellStyle) bool {
	return style != (CellStyle{})
}

func cellStateAt(sheet *Sheet, coord Coord) cellState {
	cell, exists := sheet.Cells[coord]
	return cellState{cell: cell, exists: exists}
}

func applyCellStates(session *Session, sheetName string, states map[Coord]cellState) error {
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	sheetIndex, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return fmt.Errorf("apply cell changes: sheet %q does not exist", sheetName)
	}
	sheet := &session.Workbook.Sheets[sheetIndex]
	var changed Coord
	first := true
	for coord, state := range states {
		if coord.Row < 0 || coord.Col < 0 {
			return fmt.Errorf("apply cell changes: negative coordinate (%d,%d)", coord.Row, coord.Col)
		}
		if first {
			changed = coord
			first = false
		}
		address, err := excelize.CoordinatesToCellName(coord.Col+1, coord.Row+1)
		if err != nil {
			return fmt.Errorf("address cell (%d,%d): %w", coord.Row, coord.Col, err)
		}
		if state.exists {
			sheet.Cells[coord] = state.cell
			if state.cell.Input == "" {
				if err := session.file.SetCellValue(sheetName, address, nil); err != nil {
					return fmt.Errorf("clear cell %s!%s: %w", sheetName, address, err)
				}
			} else if err := writeFormulaInput(session.file, sheetName, coord, state.cell.Input); err != nil {
				return err
			}
		} else {
			delete(sheet.Cells, coord)
			if err := session.file.SetCellValue(sheetName, address, nil); err != nil {
				return fmt.Errorf("clear cell %s!%s: %w", sheetName, address, err)
			}
		}
	}
	if first {
		return nil
	}
	updateSheetDimensions(sheet)
	session.Workbook.Dirty = true
	return Recalculate(session, CellAddress{Sheet: sheetName, Row: changed.Row, Col: changed.Col})
}

func updateSheetDimensions(sheet *Sheet) {
	sheet.Rows = 0
	sheet.Cols = 0
	for coord := range sheet.Cells {
		if coord.Row+1 > sheet.Rows {
			sheet.Rows = coord.Row + 1
		}
		if coord.Col+1 > sheet.Cols {
			sheet.Cols = coord.Col + 1
		}
	}
}

func recordCellHistory(session *Session, sheet string, before, after map[Coord]cellState) {
	beforeCopy := cloneCellStates(before)
	afterCopy := cloneCellStates(after)
	recordHistory(session, historyEntry{
		undo: func() error { return applyCellStates(session, sheet, beforeCopy) },
		redo: func() error { return applyCellStates(session, sheet, afterCopy) },
	})
}

func cloneCellStates(states map[Coord]cellState) map[Coord]cellState {
	cloned := make(map[Coord]cellState, len(states))
	for coord, state := range states {
		cloned[coord] = state
	}
	return cloned
}

// InsertRows inserts count blank rows before the zero-based row at.
func InsertRows(session *Session, sheetName string, at, count int) error {
	if err := validateInsertion(session, sheetName, at, count); err != nil {
		return err
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	beforeRows, beforeCols := session.Workbook.Sheets[idx].Rows, session.Workbook.Sheets[idx].Cols
	if err := shiftRows(session, sheetName, at, count, true); err != nil {
		return err
	}
	idx, _ = sheetIndex(session.Workbook, sheetName)
	afterRows, afterCols := session.Workbook.Sheets[idx].Rows, session.Workbook.Sheets[idx].Cols
	recordHistory(session, historyEntry{
		undo: func() error {
			if err := shiftRows(session, sheetName, at, count, false); err != nil {
				return err
			}
			setSheetDimensions(session, sheetName, beforeRows, beforeCols)
			return nil
		},
		redo: func() error {
			if err := shiftRows(session, sheetName, at, count, true); err != nil {
				return err
			}
			setSheetDimensions(session, sheetName, afterRows, afterCols)
			return nil
		},
	})
	return nil
}

// InsertColumns inserts count blank columns before the zero-based column at.
func InsertColumns(session *Session, sheetName string, at, count int) error {
	if err := validateInsertion(session, sheetName, at, count); err != nil {
		return err
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	beforeRows, beforeCols := session.Workbook.Sheets[idx].Rows, session.Workbook.Sheets[idx].Cols
	if err := shiftColumns(session, sheetName, at, count, true); err != nil {
		return err
	}
	idx, _ = sheetIndex(session.Workbook, sheetName)
	afterRows, afterCols := session.Workbook.Sheets[idx].Rows, session.Workbook.Sheets[idx].Cols
	recordHistory(session, historyEntry{
		undo: func() error {
			if err := shiftColumns(session, sheetName, at, count, false); err != nil {
				return err
			}
			setSheetDimensions(session, sheetName, beforeRows, beforeCols)
			return nil
		},
		redo: func() error {
			if err := shiftColumns(session, sheetName, at, count, true); err != nil {
				return err
			}
			setSheetDimensions(session, sheetName, afterRows, afterCols)
			return nil
		},
	})
	return nil
}

// DeleteRows removes count rows beginning at the zero-based row at.
func DeleteRows(session *Session, sheetName string, at, count int) error {
	if err := validateDeletion(session, sheetName, at, count); err != nil {
		return err
	}
	before, err := captureSession(session)
	if err != nil {
		return fmt.Errorf("prepare row delete undo: %w", err)
	}
	if err := shiftRows(session, sheetName, at, count, false); err != nil {
		_ = restoreSession(session, before)
		return err
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	updateSheetDimensions(&session.Workbook.Sheets[idx])
	after, err := captureSession(session)
	if err != nil {
		_ = restoreSession(session, before)
		return fmt.Errorf("prepare row delete redo: %w", err)
	}
	recordHistory(session, historyEntry{
		undo: func() error { return restoreSession(session, before) },
		redo: func() error { return restoreSession(session, after) },
	})
	return nil
}

// DeleteColumns removes count columns beginning at the zero-based column at.
func DeleteColumns(session *Session, sheetName string, at, count int) error {
	if err := validateDeletion(session, sheetName, at, count); err != nil {
		return err
	}
	before, err := captureSession(session)
	if err != nil {
		return fmt.Errorf("prepare column delete undo: %w", err)
	}
	if err := shiftColumns(session, sheetName, at, count, false); err != nil {
		_ = restoreSession(session, before)
		return err
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	updateSheetDimensions(&session.Workbook.Sheets[idx])
	after, err := captureSession(session)
	if err != nil {
		_ = restoreSession(session, before)
		return fmt.Errorf("prepare column delete redo: %w", err)
	}
	recordHistory(session, historyEntry{
		undo: func() error { return restoreSession(session, before) },
		redo: func() error { return restoreSession(session, after) },
	})
	return nil
}

func validateDeletion(session *Session, sheetName string, at, count int) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("delete cells: session is nil")
	}
	if at < 0 || count <= 0 {
		return fmt.Errorf("delete cells: position must be nonnegative and count positive")
	}
	if _, exists := sheetIndex(session.Workbook, sheetName); !exists {
		return fmt.Errorf("delete cells: sheet %q does not exist", sheetName)
	}
	return ensureCalculationFile(session)
}

func validateInsertion(session *Session, sheetName string, at, count int) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("insert cells: session is nil")
	}
	if at < 0 || count <= 0 {
		return fmt.Errorf("insert cells: position must be nonnegative and count positive")
	}
	if _, exists := sheetIndex(session.Workbook, sheetName); !exists {
		return fmt.Errorf("insert cells: sheet %q does not exist", sheetName)
	}
	return ensureCalculationFile(session)
}

func shiftRows(session *Session, sheetName string, at, count int, insert bool) error {
	if insert {
		if err := session.file.InsertRows(sheetName, at+1, count); err != nil {
			return fmt.Errorf("insert %d rows in %q: %w", count, sheetName, err)
		}
	} else {
		for range count {
			if err := session.file.RemoveRow(sheetName, at+1); err != nil {
				return fmt.Errorf("remove inserted row from %q: %w", sheetName, err)
			}
		}
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	sheet := &session.Workbook.Sheets[idx]
	cells := make(map[Coord]Cell, len(sheet.Cells))
	for coord, cell := range sheet.Cells {
		if insert && coord.Row >= at {
			coord.Row += count
		} else if !insert && coord.Row >= at+count {
			coord.Row -= count
		} else if !insert && coord.Row >= at {
			continue
		}
		cells[coord] = cell
	}
	sheet.Cells = cells
	if insert {
		sheet.Rows = max(sheet.Rows+count, at+count)
	} else {
		sheet.Rows = max(sheet.Rows-count, 0)
	}
	adjustFormulaReferences(session.Workbook, sheetName, false, at, count, insert)
	if err := writeAllFormulaInputs(session); err != nil {
		return err
	}
	session.Workbook.Dirty = true
	return recalculateWorkbook(session, sheetName)
}

func shiftColumns(session *Session, sheetName string, at, count int, insert bool) error {
	column, err := excelize.ColumnNumberToName(at + 1)
	if err != nil {
		return fmt.Errorf("address column %d: %w", at, err)
	}
	if insert {
		if err := session.file.InsertCols(sheetName, column, count); err != nil {
			return fmt.Errorf("insert %d columns in %q: %w", count, sheetName, err)
		}
	} else {
		for range count {
			if err := session.file.RemoveCol(sheetName, column); err != nil {
				return fmt.Errorf("remove inserted column from %q: %w", sheetName, err)
			}
		}
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	sheet := &session.Workbook.Sheets[idx]
	cells := make(map[Coord]Cell, len(sheet.Cells))
	for coord, cell := range sheet.Cells {
		if insert && coord.Col >= at {
			coord.Col += count
		} else if !insert && coord.Col >= at+count {
			coord.Col -= count
		} else if !insert && coord.Col >= at {
			continue
		}
		cells[coord] = cell
	}
	sheet.Cells = cells
	widths := make(map[int]float64, len(sheet.ColumnWidths))
	for col, width := range sheet.ColumnWidths {
		if insert && col >= at {
			col += count
		} else if !insert && col >= at+count {
			col -= count
		} else if !insert && col >= at {
			continue
		}
		widths[col] = width
	}
	sheet.ColumnWidths = widths
	if insert {
		sheet.Cols = max(sheet.Cols+count, at+count)
	} else {
		sheet.Cols = max(sheet.Cols-count, 0)
	}
	adjustFormulaReferences(session.Workbook, sheetName, true, at, count, insert)
	if err := writeAllFormulaInputs(session); err != nil {
		return err
	}
	session.Workbook.Dirty = true
	return recalculateWorkbook(session, sheetName)
}

func adjustFormulaReferences(workbook *Workbook, targetSheet string, columns bool, at, count int, insert bool) {
	threshold, delta := at, count
	if !insert {
		threshold, delta = at+count, -count
	}
	for sheetIndex := range workbook.Sheets {
		sheet := &workbook.Sheets[sheetIndex]
		for coord, cell := range sheet.Cells {
			if !strings.HasPrefix(cell.Input, "=") {
				continue
			}
			formula := strings.TrimPrefix(cell.Input, "=")
			adjusted := transformFormulaReferences(formula, sheet.Name, targetSheet, columns, threshold, delta)
			if adjusted != formula {
				cell.Input = "=" + adjusted
				cell.CalculationState = Stale
				sheet.Cells[coord] = cell
			}
		}
	}
}

func writeAllFormulaInputs(session *Session) error {
	for sheetIndex := range session.Workbook.Sheets {
		sheet := &session.Workbook.Sheets[sheetIndex]
		for coord, cell := range sheet.Cells {
			if strings.HasPrefix(cell.Input, "=") {
				if err := writeFormulaInput(session.file, sheet.Name, coord, cell.Input); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func recalculateWorkbook(session *Session, changedSheet string) error {
	found := false
	for _, sheet := range session.Workbook.Sheets {
		if sheet.Name != changedSheet {
			continue
		}
		found = true
		for coord := range sheet.Cells {
			if err := Recalculate(session, CellAddress{Sheet: sheet.Name, Row: coord.Row, Col: coord.Col}); err != nil {
				return err
			}
			break
		}
		if len(sheet.Cells) == 0 {
			if err := Recalculate(session, CellAddress{Sheet: sheet.Name}); err != nil {
				return err
			}
		}
		break
	}
	if !found {
		return fmt.Errorf("recalculate workbook: sheet %q does not exist", changedSheet)
	}
	markReferenceErrors(session.Workbook)
	return nil
}

func markReferenceErrors(workbook *Workbook) {
	for sheetIndex := range workbook.Sheets {
		sheet := &workbook.Sheets[sheetIndex]
		for coord, cell := range sheet.Cells {
			if cell.Input == "=#REF!" {
				cell.Result = "#REF!"
				cell.CalculationState = Error
				sheet.Cells[coord] = cell
			}
		}
	}
}

func setSheetDimensions(session *Session, name string, rows, cols int) {
	idx, exists := sheetIndex(session.Workbook, name)
	if exists {
		session.Workbook.Sheets[idx].Rows = rows
		session.Workbook.Sheets[idx].Cols = cols
	}
}

// DuplicateSheet copies a worksheet's modeled state and activates the copy.
func DuplicateSheet(session *Session, sourceName, newName string) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("duplicate sheet: session is nil")
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	sourceIndex, exists := sheetIndex(session.Workbook, sourceName)
	if !exists {
		return fmt.Errorf("duplicate sheet: sheet %q does not exist", sourceName)
	}
	if err := validateNewSheetName(session.Workbook, newName); err != nil {
		return err
	}
	before, err := captureSession(session)
	if err != nil {
		return fmt.Errorf("prepare sheet duplicate undo: %w", err)
	}
	targetIndex, err := session.file.NewSheet(newName)
	if err != nil {
		return fmt.Errorf("duplicate sheet %q as %q: %w", sourceName, newName, err)
	}
	if err := session.file.CopySheet(sourceIndex, targetIndex); err != nil {
		_ = restoreSession(session, before)
		return fmt.Errorf("copy sheet %q: %w", sourceName, err)
	}
	sheet := cloneSheet(session.Workbook.Sheets[sourceIndex])
	sheet.Name = newName
	session.Workbook.Sheets = append(session.Workbook.Sheets, sheet)
	session.Workbook.ActiveSheet = targetIndex
	session.file.SetActiveSheet(targetIndex)
	session.Workbook.Dirty = true
	after, err := captureSession(session)
	if err != nil {
		_ = restoreSession(session, before)
		return fmt.Errorf("prepare sheet duplicate redo: %w", err)
	}
	recordHistory(session, historyEntry{
		undo: func() error { return restoreSession(session, before) },
		redo: func() error { return restoreSession(session, after) },
	})
	return nil
}

func cloneSheet(source Sheet) Sheet {
	copy := source
	copy.Cells = make(map[Coord]Cell, len(source.Cells))
	for coord, cell := range source.Cells {
		copy.Cells[coord] = cell
	}
	copy.ColumnWidths = make(map[int]float64, len(source.ColumnWidths))
	for col, width := range source.ColumnWidths {
		copy.ColumnWidths[col] = width
	}
	copy.Merges = append([]CellRange(nil), source.Merges...)
	copy.ConditionalRules = append([]ConditionalRule(nil), source.ConditionalRules...)
	for index := range copy.ConditionalRules {
		if scale := copy.ConditionalRules[index].ColorScale; scale != nil {
			cloned := *scale
			copy.ConditionalRules[index].ColorScale = &cloned
		}
	}
	return copy
}

// AddSheet appends a sheet and makes it active.
func AddSheet(session *Session, name string) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("add sheet: session is nil")
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	if err := validateNewSheetName(session.Workbook, name); err != nil {
		return err
	}
	if err := addSheetRaw(session, name); err != nil {
		return err
	}
	recordHistory(session, historyEntry{
		undo: func() error { return deleteSheetRaw(session, name) },
		redo: func() error { return addSheetRaw(session, name) },
	})
	return nil
}

func addSheetRaw(session *Session, name string) error {
	index, err := session.file.NewSheet(name)
	if err != nil {
		return fmt.Errorf("add sheet %q: %w", name, err)
	}
	session.Workbook.Sheets = append(session.Workbook.Sheets, Sheet{Name: name, Cells: make(map[Coord]Cell), ColumnWidths: make(map[int]float64)})
	session.Workbook.ActiveSheet = index
	session.Workbook.Dirty = true
	return nil
}

func validateNewSheetName(workbook *Workbook, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("sheet name cannot be empty")
	}
	for _, sheet := range workbook.Sheets {
		if strings.EqualFold(sheet.Name, name) {
			return fmt.Errorf("sheet %q already exists", name)
		}
	}
	return nil
}

// RenameSheet renames a sheet and updates formula references in the workbook model.
func RenameSheet(session *Session, oldName, newName string) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("rename sheet: session is nil")
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	if _, exists := sheetIndex(session.Workbook, oldName); !exists {
		return fmt.Errorf("rename sheet: sheet %q does not exist", oldName)
	}
	if err := validateNewSheetName(session.Workbook, newName); err != nil && !strings.EqualFold(oldName, newName) {
		return err
	}
	if strings.TrimSpace(newName) == "" {
		return fmt.Errorf("sheet name cannot be empty")
	}
	if err := renameSheetRaw(session, oldName, newName); err != nil {
		return err
	}
	recordHistory(session, historyEntry{
		undo: func() error { return renameSheetRaw(session, newName, oldName) },
		redo: func() error { return renameSheetRaw(session, oldName, newName) },
	})
	return nil
}

func renameSheetRaw(session *Session, oldName, newName string) error {
	if err := session.file.SetSheetName(oldName, newName); err != nil {
		return fmt.Errorf("rename sheet %q to %q: %w", oldName, newName, err)
	}
	idx, _ := sheetIndex(session.Workbook, oldName)
	session.Workbook.Sheets[idx].Name = newName
	for sheetIndex := range session.Workbook.Sheets {
		sheet := &session.Workbook.Sheets[sheetIndex]
		for coord, cell := range sheet.Cells {
			if !strings.HasPrefix(cell.Input, "=") {
				continue
			}
			formula := strings.TrimPrefix(cell.Input, "=")
			renamed := renameFormulaReferences(formula, oldName, newName)
			if renamed != formula {
				cell.Input = "=" + renamed
				cell.CalculationState = Stale
				sheet.Cells[coord] = cell
			}
		}
	}
	if err := writeAllFormulaInputs(session); err != nil {
		return err
	}
	session.Workbook.Dirty = true
	return recalculateWorkbook(session, newName)
}

// DeleteSheet removes a sheet; its full workbook package is retained for an undo.
func DeleteSheet(session *Session, name string) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("delete sheet: session is nil")
	}
	if len(session.Workbook.Sheets) <= 1 {
		return fmt.Errorf("delete sheet: workbook must retain at least one sheet")
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	snapshot, err := captureSession(session)
	if err != nil {
		return fmt.Errorf("prepare sheet delete undo: %w", err)
	}
	if err := deleteSheetRaw(session, name); err != nil {
		return err
	}
	recordHistory(session, historyEntry{
		undo: func() error { return restoreSession(session, snapshot) },
		redo: func() error { return deleteSheetRaw(session, name) },
	})
	return nil
}

func deleteSheetRaw(session *Session, name string) error {
	if len(session.Workbook.Sheets) <= 1 {
		return fmt.Errorf("delete sheet: workbook must retain at least one sheet")
	}
	idx, exists := sheetIndex(session.Workbook, name)
	if !exists {
		return fmt.Errorf("delete sheet: sheet %q does not exist", name)
	}
	if err := session.file.DeleteSheet(name); err != nil {
		return fmt.Errorf("delete sheet %q: %w", name, err)
	}
	session.Workbook.Sheets = append(session.Workbook.Sheets[:idx], session.Workbook.Sheets[idx+1:]...)
	if session.Workbook.ActiveSheet > idx {
		session.Workbook.ActiveSheet--
	} else if session.Workbook.ActiveSheet == idx {
		session.Workbook.ActiveSheet = min(idx, len(session.Workbook.Sheets)-1)
	}
	session.Workbook.Dirty = true
	return nil
}

func captureSession(session *Session) (sessionSnapshot, error) {
	buffer, err := session.file.WriteToBuffer()
	if err != nil {
		return sessionSnapshot{}, err
	}
	return sessionSnapshot{data: bytes.Clone(buffer.Bytes()), activeSheet: session.Workbook.ActiveSheet}, nil
}

func restoreSession(session *Session, snapshot sessionSnapshot) error {
	file, err := excelize.OpenReader(bytes.NewReader(snapshot.data))
	if err != nil {
		return fmt.Errorf("restore workbook package: %w", err)
	}
	workbook, err := workbookFromExcelize(file, session.Path, session.Format)
	if err != nil {
		file.Close()
		return err
	}
	markReferenceErrors(workbook)
	workbook.ActiveSheet = min(snapshot.activeSheet, len(workbook.Sheets)-1)
	workbook.Dirty = true
	old := session.file
	session.file = file
	session.Workbook = workbook
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func transformFormulaReferences(formula, formulaSheet, targetSheet string, columns bool, threshold, delta int) string {
	references := make([]formulaReference, 0, 4)
	starts := make([]int, 0, 4)
	for index := 0; index < len(formula); {
		if formula[index] == '"' {
			index = skipFormulaString(formula, index)
			continue
		}
		if reference, ok := parseCellReference(formula, index); ok {
			references = append(references, reference)
			starts = append(starts, index)
			index = reference.end
			continue
		}
		index++
	}

	var output strings.Builder
	output.Grow(len(formula))
	referenceIndex := 0
	for index := 0; index < len(formula); {
		if formula[index] == '"' {
			end := skipFormulaString(formula, index)
			output.WriteString(formula[index:end])
			index = end
			continue
		}
		if referenceIndex < len(references) && index == starts[referenceIndex] {
			reference := references[referenceIndex]
			refStart := starts[referenceIndex]
			referenceIndex++
			refSheet := formulaSheet
			if reference.qualified {
				refSheet = reference.sheet
			} else if referenceIndex >= 2 && formula[references[referenceIndex-2].end] == ':' {
				refSheet = references[referenceIndex-2].sheet
				if refSheet == "" {
					refSheet = formulaSheet
				}
			}
			if !strings.EqualFold(refSheet, targetSheet) {
				output.WriteString(formula[refStart:reference.end])
				index = reference.end
				continue
			}
			coord := reference.row - 1
			if columns {
				coord = reference.columnNumber - 1
			}
			previousEndpoint := referenceIndex > 1 && formula[references[referenceIndex-2].end:min(references[referenceIndex-2].end+1, len(formula))] == ":"
			nextEndpoint := formula[reference.end:min(reference.end+1, len(formula))] == ":"
			if delta < 0 && (previousEndpoint || nextEndpoint) {
				endpointIndex := referenceIndex - 1
				if previousEndpoint {
					endpointIndex--
				}
				startReference, endReference := references[endpointIndex], references[endpointIndex+1]
				startCoord, endCoord := startReference.row-1, endReference.row-1
				if columns {
					startCoord, endCoord = startReference.columnNumber-1, endReference.columnNumber-1
				}
				rangeStart, rangeEnd := min(startCoord, endCoord), max(startCoord, endCoord)
				deletedStart, deletedEnd := threshold+delta, threshold-1
				survivesBefore, survivesAfter := rangeStart < deletedStart, rangeEnd > deletedEnd
				if !survivesBefore && !survivesAfter {
					output.WriteString("#REF!")
				} else {
					newLow, newHigh := rangeStart, rangeEnd
					switch {
					case survivesBefore && survivesAfter:
						newHigh += delta
					case survivesBefore:
						newHigh = deletedStart - 1
					case survivesAfter:
						newLow = deletedStart
						newHigh += delta
					}
					newStart, newEnd := newLow, newHigh
					if startCoord > endCoord {
						newStart, newEnd = newHigh, newLow
					}
					formatEndpoint := func(ref formulaReference, value int) string {
						if columns {
							ref.columnNumber = value + 1
						} else {
							ref.row = value + 1
						}
						return formatFormulaReference(ref)
					}
					output.WriteString(formatEndpoint(startReference, newStart))
					output.WriteByte(':')
					output.WriteString(formatEndpoint(endReference, newEnd))
				}
				index = endReference.end
				referenceIndex = endpointIndex + 2
				continue
			}
			inDeletedRange := delta < 0 && coord >= threshold+delta && coord < threshold
			if inDeletedRange && !previousEndpoint && !nextEndpoint {
				output.WriteString("#REF!")
			} else {
				if coord >= threshold {
					if columns {
						reference.columnNumber += delta
					} else {
						reference.row += delta
					}
				}
				output.WriteString(formatFormulaReference(reference))
			}
			index = reference.end
			continue
		}
		output.WriteByte(formula[index])
		index++
	}
	return output.String()
}

func formatFormulaReference(reference formulaReference) string {
	var output strings.Builder
	output.WriteString(reference.prefix)
	if reference.columnAbsolute {
		output.WriteByte('$')
	}
	columnName, _ := excelize.ColumnNumberToName(reference.columnNumber)
	output.WriteString(columnName)
	if reference.rowAbsolute {
		output.WriteByte('$')
	}
	output.WriteString(strconv.Itoa(reference.row))
	return output.String()
}

type formulaReference struct {
	end            int
	prefix         string
	sheet          string
	qualified      bool
	columnNumber   int
	columnAbsolute bool
	row            int
	rowAbsolute    bool
}

func parseCellReference(formula string, start int) (formulaReference, bool) {
	if start > 0 && isFormulaIdentifier(formula[start-1]) {
		return formulaReference{}, false
	}
	position := start
	sheet := ""
	qualified := false
	if formula[position] == '\'' {
		candidate, end, ok := parseQuotedSheet(formula, position)
		if ok && end < len(formula) && formula[end] == '!' {
			sheet, qualified, position = candidate, true, end+1
		}
	} else {
		end := position
		for end < len(formula) && (isFormulaIdentifier(formula[end]) || formula[end] == '.') {
			end++
		}
		if end > position && end < len(formula) && formula[end] == '!' {
			sheet, qualified, position = formula[position:end], true, end+1
		}
	}
	prefix := formula[start:position]
	columnAbsolute := false
	if position < len(formula) && formula[position] == '$' {
		columnAbsolute = true
		position++
	}
	columnStart := position
	for position < len(formula) && (formula[position] >= 'A' && formula[position] <= 'Z' || formula[position] >= 'a' && formula[position] <= 'z') {
		position++
	}
	if position == columnStart || position-columnStart > 3 {
		return formulaReference{}, false
	}
	columnNumber, err := excelize.ColumnNameToNumber(formula[columnStart:position])
	if err != nil {
		return formulaReference{}, false
	}
	rowAbsolute := false
	if position < len(formula) && formula[position] == '$' {
		rowAbsolute = true
		position++
	}
	rowStart := position
	for position < len(formula) && formula[position] >= '0' && formula[position] <= '9' {
		position++
	}
	if position == rowStart || position < len(formula) && isFormulaIdentifier(formula[position]) {
		return formulaReference{}, false
	}
	row, err := strconv.Atoi(formula[rowStart:position])
	if err != nil || row < 1 {
		return formulaReference{}, false
	}
	return formulaReference{
		end:            position,
		prefix:         prefix,
		sheet:          sheet,
		qualified:      qualified,
		columnNumber:   columnNumber,
		columnAbsolute: columnAbsolute,
		row:            row,
		rowAbsolute:    rowAbsolute,
	}, true
}

func parseQuotedSheet(formula string, start int) (string, int, bool) {
	var sheet strings.Builder
	for index := start + 1; index < len(formula); index++ {
		if formula[index] != '\'' {
			sheet.WriteByte(formula[index])
			continue
		}
		if index+1 < len(formula) && formula[index+1] == '\'' {
			sheet.WriteByte('\'')
			index++
			continue
		}
		return sheet.String(), index + 1, true
	}
	return "", start, false
}

func skipFormulaString(formula string, start int) int {
	for index := start + 1; index < len(formula); index++ {
		if formula[index] == '"' {
			if index+1 < len(formula) && formula[index+1] == '"' {
				index++
				continue
			}
			return index + 1
		}
	}
	return len(formula)
}

func renameFormulaReferences(formula, oldName, newName string) string {
	var output strings.Builder
	output.Grow(len(formula))
	for index := 0; index < len(formula); {
		if formula[index] == '"' {
			end := skipFormulaString(formula, index)
			output.WriteString(formula[index:end])
			index = end
			continue
		}
		if reference, ok := parseCellReference(formula, index); ok {
			if reference.qualified && strings.EqualFold(reference.sheet, oldName) {
				output.WriteString(formatSheetQualifier(newName))
				cellStart := index + len(reference.prefix)
				output.WriteString(formula[cellStart:reference.end])
			} else {
				output.WriteString(formula[index:reference.end])
			}
			index = reference.end
			continue
		}
		output.WriteByte(formula[index])
		index++
	}
	return output.String()
}

func formatSheetQualifier(name string) string {
	simple := name != ""
	if simple {
		simple = isFormulaIdentifierStart(name[0])
	}
	for index := range name {
		if index > 0 && !isFormulaIdentifier(name[index]) {
			simple = false
			break
		}
	}
	if _, _, err := excelize.CellNameToCoordinates(name); err == nil {
		simple = false
	}
	if simple {
		return name + "!"
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'!"
}
