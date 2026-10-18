package workbook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

func workbookFromExcelize(file *excelize.File, path string, format Format) (*Workbook, error) {
	workbook := &Workbook{Format: format, Path: path}
	for _, sheetName := range file.GetSheetList() {
		rows, err := file.GetRows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("read sheet %q from %q: %w", sheetName, path, err)
		}
		sheet := Sheet{Name: sheetName, Rows: len(rows), Cells: make(map[Coord]Cell), ColumnWidths: make(map[int]float64)}
		styleCache := make(map[int]CellStyle)
		for rowIndex, row := range rows {
			if len(row) > sheet.Cols {
				sheet.Cols = len(row)
			}
			for colIndex, result := range row {
				cellName, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
				if err != nil {
					return nil, fmt.Errorf("address cell in sheet %q: %w", sheetName, err)
				}
				formula, err := file.GetCellFormula(sheetName, cellName)
				if err != nil {
					return nil, fmt.Errorf("read formula %s!%s: %w", sheetName, cellName, err)
				}
				styleID, err := file.GetCellStyle(sheetName, cellName)
				if err != nil {
					return nil, fmt.Errorf("read style %s!%s: %w", sheetName, cellName, err)
				}
				cellStyle, cached := styleCache[styleID]
				if !cached {
					style, err := file.GetStyle(styleID)
					if err != nil {
						return nil, fmt.Errorf("read style definition %s!%s: %w", sheetName, cellName, err)
					}
					cellStyle = styleFromExcelize(style)
					styleCache[styleID] = cellStyle
				}
				input := result
				state := Literal
				if formula != "" {
					input = "=" + formula
					state = Stale
				}
				cell := Cell{Input: input, Result: result, CalculationState: state, Style: cellStyle}
				if input == "" && result == "" && !hasCellStyle(cell.Style) {
					continue
				}
				sheet.Cells[Coord{Row: rowIndex, Col: colIndex}] = cell
			}
		}
		for col := range sheet.Cols {
			columnName, err := excelize.ColumnNumberToName(col + 1)
			if err != nil {
				return nil, fmt.Errorf("address column %d in sheet %q: %w", col, sheetName, err)
			}
			width, err := file.GetColWidth(sheetName, columnName)
			if err != nil {
				return nil, fmt.Errorf("read width %s!%s: %w", sheetName, columnName, err)
			}
			sheet.ColumnWidths[col] = width
		}
		merges, err := file.GetMergeCells(sheetName, true)
		if err != nil {
			return nil, fmt.Errorf("read merged cells in sheet %q: %w", sheetName, err)
		}
		for _, merge := range merges {
			start, startRow, err := excelize.CellNameToCoordinates(merge.GetStartAxis())
			if err != nil {
				return nil, fmt.Errorf("read merge start in sheet %q: %w", sheetName, err)
			}
			end, endRow, err := excelize.CellNameToCoordinates(merge.GetEndAxis())
			if err != nil {
				return nil, fmt.Errorf("read merge end in sheet %q: %w", sheetName, err)
			}
			sheet.Merges = append(sheet.Merges, CellRange{Start: Coord{Row: startRow - 1, Col: start - 1}, End: Coord{Row: endRow - 1, Col: end - 1}})
			sheet.Rows = max(sheet.Rows, endRow)
			sheet.Cols = max(sheet.Cols, end)
		}
		rules, err := loadConditionalRules(file, sheetName)
		if err != nil {
			return nil, fmt.Errorf("read conditional formats in sheet %q: %w", sheetName, err)
		}
		sheet.ConditionalRules = rules
		workbook.Sheets = append(workbook.Sheets, sheet)
	}
	if len(workbook.Sheets) == 0 {
		return nil, fmt.Errorf("workbook %q contains no worksheets", path)
	}
	return workbook, nil
}

func loadConditionalRules(file *excelize.File, sheetName string) ([]ConditionalRule, error) {
	formats, err := file.GetConditionalFormats(sheetName)
	if err != nil {
		return nil, err
	}
	var rules []ConditionalRule
	for reference, options := range formats {
		selection, err := cellRangeFromExcelize(reference)
		if err != nil {
			continue
		}
		for _, option := range options {
			operator, supported := conditionalOperator(option)
			if !supported || option.Format == nil {
				continue
			}
			style, err := file.GetConditionalStyle(*option.Format)
			if err != nil {
				return nil, err
			}
			converted := styleFromExcelize(style)
			rule := ConditionalRule{Range: selection, Operator: operator, Value: option.Value, FillRGB: converted.FillRGB, FontRGB: converted.FontRGB}
			if validateConditionalRule(rule) == nil {
				rules = append(rules, rule)
			}
		}
	}
	return rules, nil
}

func conditionalOperator(option excelize.ConditionalFormatOptions) (RuleOperator, bool) {
	criteria := strings.ToLower(strings.TrimSpace(option.Criteria))
	switch {
	case option.Type == "cell" && (criteria == ">" || criteria == "greater than"):
		return GreaterThan, true
	case option.Type == "cell" && (criteria == "<" || criteria == "less than"):
		return LessThan, true
	case option.Type == "cell" && (criteria == "==" || criteria == "=" || criteria == "equal to"):
		return Equal, true
	case option.Type == "blanks":
		return Blank, true
	case option.Type == "text" && criteria == "containing":
		return TextContains, true
	default:
		return "", false
	}
}

func cellRangeFromExcelize(reference string) (CellRange, error) {
	parts := strings.Split(reference, ":")
	if len(parts) == 1 {
		parts = append(parts, parts[0])
	}
	if len(parts) != 2 {
		return CellRange{}, fmt.Errorf("invalid cell range %q", reference)
	}
	startCol, startRow, err := excelize.CellNameToCoordinates(parts[0])
	if err != nil {
		return CellRange{}, err
	}
	endCol, endRow, err := excelize.CellNameToCoordinates(parts[1])
	if err != nil {
		return CellRange{}, err
	}
	return CellRange{Start: Coord{Row: startRow - 1, Col: startCol - 1}, End: Coord{Row: endRow - 1, Col: endCol - 1}}, nil
}

// SaveFile writes through a sibling temporary file and refuses package changes outside the safe allowlist.
func SaveFile(session *Session, path string) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("save workbook: session is nil")
	}
	if path == "" {
		path = session.Path
	}
	format, err := formatForPath(path)
	if err != nil {
		return err
	}
	if format != session.Format {
		return fmt.Errorf("save %q as %s would change workbook format from %s", path, format, session.Format)
	}
	if format == CSV {
		return saveCSV(session, path)
	}
	if session.file == nil {
		return fmt.Errorf("save %q: XLSX session has no open workbook", path)
	}

	tempPath, err := createSiblingTemp(path)
	if err != nil {
		return fmt.Errorf("prepare save for %q: %w", path, err)
	}
	defer os.Remove(tempPath)
	if err := session.file.SaveAs(tempPath); err != nil {
		return fmt.Errorf("save temporary workbook for %q: %w", path, err)
	}
	if session.Path != "" && len(session.OriginalParts) > 0 {
		if err := AuditPackage(session.Path, tempPath, safeWorkbookChanges(session.OriginalParts)); err != nil {
			return fmt.Errorf("refusing save of %q to preserve workbook content: %w", path, err)
		}
	}
	parts, err := SnapshotParts(tempPath)
	if err != nil {
		return fmt.Errorf("snapshot temporary workbook for %q: %w", path, err)
	}
	if err := preserveMode(tempPath, path); err != nil {
		return fmt.Errorf("prepare replacement for %q: %w", path, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace workbook %q: %w", path, err)
	}
	session.Path = path
	session.Workbook.Path = path
	session.OriginalParts = parts
	session.Workbook.Dirty = false
	return nil
}

func saveCSV(session *Session, path string) error {
	tempPath, err := createSiblingTemp(path)
	if err != nil {
		return fmt.Errorf("prepare save for %q: %w", path, err)
	}
	defer os.Remove(tempPath)
	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return fmt.Errorf("open temporary CSV for %q: %w", path, err)
	}
	writeErr := WriteCSV(file, session.Workbook, session.Workbook.ActiveSheet)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write CSV %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close temporary CSV for %q: %w", path, closeErr)
	}
	if err := preserveMode(tempPath, path); err != nil {
		return fmt.Errorf("prepare replacement for %q: %w", path, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace CSV %q: %w", path, err)
	}
	session.Path = path
	session.Workbook.Path = path
	session.Workbook.Dirty = false
	return nil
}

func createSiblingTemp(path string) (string, error) {
	extension := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), extension)
	file, err := os.CreateTemp(filepath.Dir(path), "."+base+".*"+extension)
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

func preserveMode(tempPath, targetPath string) error {
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Chmod(tempPath, info.Mode().Perm())
}
