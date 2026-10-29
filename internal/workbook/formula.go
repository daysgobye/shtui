package workbook

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// CellAddress identifies a zero-based cell location on a workbook sheet.
type CellAddress struct {
	Sheet string
	Row   int
	Col   int
}

var supportedFormulaFunctions = map[string]struct{}{
	"SUM": {}, "AVERAGE": {}, "COUNT": {}, "MIN": {}, "MAX": {}, "IF": {},
	"COUNTIF": {}, "IFERROR": {}, "COUNTA": {},
}

// Recalculate updates all formula results so cross-cell and cross-sheet dependents stay current.
func Recalculate(session *Session, changed CellAddress) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("recalculate: session is nil")
	}
	if changed.Row < 0 || changed.Col < 0 {
		return fmt.Errorf("recalculate: negative cell coordinate (%d,%d)", changed.Row, changed.Col)
	}
	if _, ok := sheetIndex(session.Workbook, changed.Sheet); !ok {
		return fmt.Errorf("recalculate: sheet %q does not exist", changed.Sheet)
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	if err := syncChangedCell(session, changed); err != nil {
		return err
	}

	if err := calculateSUMIFResults(session); err != nil {
		return err
	}

	for sheetIndex := range session.Workbook.Sheets {
		sheet := &session.Workbook.Sheets[sheetIndex]
		for coord, cell := range sheet.Cells {
			if !strings.HasPrefix(cell.Input, "=") {
				continue
			}
			address, err := excelize.CoordinatesToCellName(coord.Col+1, coord.Row+1)
			if err != nil {
				if restoreErr := restoreSUMIFFormulas(session); restoreErr != nil {
					return restoreErr
				}
				return fmt.Errorf("recalculate %s!%d,%d: %w", sheet.Name, coord.Row, coord.Col, err)
			}
			formula := strings.TrimPrefix(cell.Input, "=")
			if isSUMIFFormula(formula) {
				continue
			}
			if hasUnsupportedFunction(formula) {
				cell.CalculationState = Stale
				sheet.Cells[coord] = cell
				continue
			}
			if err := session.file.SetCellFormula(sheet.Name, address, formula); err != nil {
				cell.CalculationState = Error
				cell.Result = err.Error()
				sheet.Cells[coord] = cell
				continue
			}
			result, err := session.file.CalcCellValue(sheet.Name, address)
			if err != nil {
				cell.CalculationState = Error
				cell.Result = err.Error()
				sheet.Cells[coord] = cell
				continue
			}
			if isFormulaError(result) {
				if strings.EqualFold(result, "#NAME?") {
					cell.CalculationState = Stale
				} else {
					cell.CalculationState = Error
					cell.Result = result
				}
				sheet.Cells[coord] = cell
				continue
			}
			cell.Result = result
			cell.CalculationState = Current
			sheet.Cells[coord] = cell
		}
	}
	return restoreSUMIFFormulas(session)
}

func calculateSUMIFResults(session *Session) error {
	for sheetIndex := range session.Workbook.Sheets {
		sheet := &session.Workbook.Sheets[sheetIndex]
		for coord, cell := range sheet.Cells {
			if !strings.HasPrefix(cell.Input, "=") {
				continue
			}
			formula := strings.TrimPrefix(cell.Input, "=")
			result, handled, err := evaluateSUMIF(session, sheet.Name, formula)
			if !handled {
				continue
			}
			address, addressErr := excelize.CoordinatesToCellName(coord.Col+1, coord.Row+1)
			if addressErr != nil {
				if restoreErr := restoreSUMIFFormulas(session); restoreErr != nil {
					return restoreErr
				}
				return fmt.Errorf("recalculate %s!%d,%d: %w", sheet.Name, coord.Row, coord.Col, addressErr)
			}
			if err == nil {
				err = session.file.SetCellValue(sheet.Name, address, excelizeLiteral(result))
			}
			if err != nil {
				cell.CalculationState = Error
				cell.Result = err.Error()
				if setErr := session.file.SetCellFormula(sheet.Name, address, "NA()"); setErr != nil {
					if restoreErr := restoreSUMIFFormulas(session); restoreErr != nil {
						return restoreErr
					}
					return fmt.Errorf("set calculation error for %s!%s: %w", sheet.Name, address, setErr)
				}
			} else {
				cell.Result = result
				cell.CalculationState = Current
			}
			sheet.Cells[coord] = cell
		}
	}
	return nil
}

func restoreSUMIFFormulas(session *Session) error {
	for sheetIndex := range session.Workbook.Sheets {
		sheet := &session.Workbook.Sheets[sheetIndex]
		for coord, cell := range sheet.Cells {
			if !strings.HasPrefix(cell.Input, "=") || !isSUMIFFormula(strings.TrimPrefix(cell.Input, "=")) {
				continue
			}
			if err := writeFormulaInput(session.file, sheet.Name, coord, cell.Input); err != nil {
				return err
			}
		}
	}
	return nil
}
func ensureCalculationFile(session *Session) error {
	if session.file != nil {
		return nil
	}
	file := excelize.NewFile()
	if len(session.Workbook.Sheets) == 0 {
		file.Close()
		return fmt.Errorf("recalculate: workbook has no sheets")
	}
	for index := range session.Workbook.Sheets {
		sheet := &session.Workbook.Sheets[index]
		if index == 0 {
			if sheet.Name != "Sheet1" {
				if err := file.SetSheetName("Sheet1", sheet.Name); err != nil {
					file.Close()
					return fmt.Errorf("create calculation sheet %q: %w", sheet.Name, err)
				}
			}
		} else if _, err := file.NewSheet(sheet.Name); err != nil {
			file.Close()
			return fmt.Errorf("create calculation sheet %q: %w", sheet.Name, err)
		}
		for coord, cell := range sheet.Cells {
			if err := writeFormulaInput(file, sheet.Name, coord, cell.Input); err != nil {
				file.Close()
				return err
			}
		}
	}
	session.file = file
	return nil
}

func syncChangedCell(session *Session, changed CellAddress) error {
	sheetIndex, _ := sheetIndex(session.Workbook, changed.Sheet)
	sheet := &session.Workbook.Sheets[sheetIndex]
	coord := Coord{Row: changed.Row, Col: changed.Col}
	cell, exists := sheet.Cells[coord]
	if !exists {
		address, err := excelize.CoordinatesToCellName(changed.Col+1, changed.Row+1)
		if err != nil {
			return fmt.Errorf("clear changed cell: %w", err)
		}
		if err := session.file.SetCellValue(changed.Sheet, address, nil); err != nil {
			return fmt.Errorf("clear changed cell %s!%s: %w", changed.Sheet, address, err)
		}
		return nil
	}
	return writeFormulaInput(session.file, changed.Sheet, coord, cell.Input)
}

func writeFormulaInput(file *excelize.File, sheet string, coord Coord, input string) error {
	address, err := excelize.CoordinatesToCellName(coord.Col+1, coord.Row+1)
	if err != nil {
		return fmt.Errorf("address cell %s!%d,%d: %w", sheet, coord.Row, coord.Col, err)
	}
	if strings.HasPrefix(input, "=") {
		if err := file.SetCellFormula(sheet, address, strings.TrimPrefix(input, "=")); err != nil {
			return fmt.Errorf("set formula %s!%s: %w", sheet, address, err)
		}
		return nil
	}
	if err := file.SetCellValue(sheet, address, excelizeLiteral(input)); err != nil {
		return fmt.Errorf("set value %s!%s: %w", sheet, address, err)
	}
	return nil
}

func excelizeLiteral(input string) any {
	if integer, err := strconv.ParseInt(input, 10, 64); err == nil {
		return integer
	}
	if number, err := strconv.ParseFloat(input, 64); err == nil {
		return number
	}
	return input
}

func hasUnsupportedFunction(formula string) bool {
	for index := 0; index < len(formula); {
		if formula[index] == '"' {
			index++
			for index < len(formula) {
				if formula[index] == '"' {
					if index+1 < len(formula) && formula[index+1] == '"' {
						index += 2
						continue
					}
					index++
					break
				}
				index++
			}
			continue
		}
		if !isFormulaIdentifierStart(formula[index]) {
			index++
			continue
		}
		start := index
		for index < len(formula) && isFormulaIdentifier(formula[index]) {
			index++
		}
		name := strings.ToUpper(formula[start:index])
		for index < len(formula) && (formula[index] == ' ' || formula[index] == '\t') {
			index++
		}
		if index < len(formula) && formula[index] == '(' {
			name = strings.TrimPrefix(name, "_XLFN.")
			if _, supported := supportedFormulaFunctions[name]; !supported {
				return true
			}
		}
	}
	return false
}

func isFormulaIdentifierStart(char byte) bool {
	return char == '_' || char == '.' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z'
}

func isFormulaIdentifier(char byte) bool {
	return isFormulaIdentifierStart(char) || char >= '0' && char <= '9'
}

func isFormulaError(result string) bool {
	upper := strings.ToUpper(result)
	return strings.HasPrefix(upper, "#")
}

func sheetIndex(workbook *Workbook, name string) (int, bool) {
	for index := range workbook.Sheets {
		if workbook.Sheets[index].Name == name {
			return index, true
		}
	}
	return 0, false
}
