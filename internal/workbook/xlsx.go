package workbook

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

func workbookFromExcelize(file *excelize.File, path string, format Format) (*Workbook, error) {
	worksheetParts, err := worksheetPackageParts(file)
	if err != nil {
		return nil, fmt.Errorf("read worksheet parts: %w", err)
	}
	styledCellRefs, err := styledCellReferences(worksheetParts)
	if err != nil {
		return nil, fmt.Errorf("read styled cell references: %w", err)
	}
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
		for _, cellName := range styledCellRefs[sheetName] {
			column, row, err := excelize.CellNameToCoordinates(cellName)
			if err != nil {
				return nil, fmt.Errorf("address styled cell %s!%s: %w", sheetName, cellName, err)
			}
			coord := Coord{Row: row - 1, Col: column - 1}
			if _, exists := sheet.Cells[coord]; exists {
				continue
			}
			styleID, err := file.GetCellStyle(sheetName, cellName)
			if err != nil {
				return nil, fmt.Errorf("read style %s!%s: %w", sheetName, cellName, err)
			}
			if styleID == 0 {
				continue
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
			if !hasCellStyle(cellStyle) {
				continue
			}
			sheet.Cells[coord] = Cell{Style: cellStyle, CalculationState: Literal}
			sheet.Rows = max(sheet.Rows, row)
			sheet.Cols = max(sheet.Cols, column)
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
		widthRanges, err := worksheetColumnWidthRanges(worksheetParts[sheetName])
		if err != nil {
			return nil, fmt.Errorf("read column width ranges in sheet %q: %w", sheetName, err)
		}
		for _, widthRange := range widthRanges {
			for col := max(widthRange.min-1, sheet.Cols); col < widthRange.max; col++ {
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

func worksheetPackageParts(file *excelize.File) (map[string][]byte, error) {
	workbookData, ok := excelizePackagePart(file, "xl/workbook.xml")
	if !ok {
		return nil, nil
	}
	var workbookXML struct {
		Sheets struct {
			Items []struct {
				Name string `xml:"name,attr"`
				Rel  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
			} `xml:"sheet"`
		} `xml:"sheets"`
	}
	if err := xml.Unmarshal(workbookData, &workbookXML); err != nil {
		return nil, fmt.Errorf("decode workbook XML: %w", err)
	}
	relationshipData, ok := excelizePackagePart(file, "xl/_rels/workbook.xml.rels")
	if !ok {
		return nil, nil
	}
	var relationshipsXML struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
			Type   string `xml:"Type,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(relationshipData, &relationshipsXML); err != nil {
		return nil, fmt.Errorf("decode workbook relationships: %w", err)
	}
	relationships := make(map[string]struct {
		target string
		kind   string
	}, len(relationshipsXML.Items))
	for _, relationship := range relationshipsXML.Items {
		relationships[relationship.ID] = struct {
			target string
			kind   string
		}{target: relationship.Target, kind: relationship.Type}
	}
	result := make(map[string][]byte)
	for _, sheet := range workbookXML.Sheets.Items {
		relationship, exists := relationships[sheet.Rel]
		if !exists || !strings.HasSuffix(relationship.kind, "/worksheet") {
			continue
		}
		target := strings.TrimPrefix(strings.TrimSpace(relationship.target), "/")
		if !strings.HasPrefix(target, "xl/") {
			target = path.Join("xl", target)
		}
		target = path.Clean(target)
		if !strings.HasPrefix(target, "xl/worksheets/") || !strings.HasSuffix(target, ".xml") {
			continue
		}
		worksheetData, ok := excelizePackagePart(file, target)
		if ok {
			result[sheet.Name] = worksheetData
		}
	}
	return result, nil
}

func styledCellReferences(worksheetParts map[string][]byte) (map[string][]string, error) {
	result := make(map[string][]string, len(worksheetParts))
	for sheetName, worksheetData := range worksheetParts {
		references, err := styledCellReferencesFromWorksheet(worksheetData)
		if err != nil {
			return nil, fmt.Errorf("decode worksheet %q: %w", sheetName, err)
		}
		result[sheetName] = references
	}
	return result, nil
}

type columnRange struct {
	min int
	max int
}

func worksheetColumnWidthRanges(data []byte) ([]columnRange, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var worksheetXML struct {
		Cols struct {
			Items []struct {
				Min   int     `xml:"min,attr"`
				Max   int     `xml:"max,attr"`
				Width float64 `xml:"width,attr"`
			} `xml:"col"`
		} `xml:"cols"`
	}
	if err := xml.Unmarshal(data, &worksheetXML); err != nil {
		return nil, err
	}
	ranges := make([]columnRange, 0, len(worksheetXML.Cols.Items))
	for _, column := range worksheetXML.Cols.Items {
		if column.Min < 1 || column.Max < column.Min || column.Width <= 0 {
			continue
		}
		ranges = append(ranges, columnRange{min: column.Min, max: column.Max})
	}
	return ranges, nil
}

func styledCellReferencesFromWorksheet(data []byte) ([]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	references := make([]string, 0)
	inSheetData := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return references, nil
		}
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Local == "sheetData" {
				inSheetData = true
				continue
			}
			if !inSheetData || token.Name.Local != "c" {
				continue
			}
			var reference, style string
			for _, attribute := range token.Attr {
				switch attribute.Name.Local {
				case "r":
					reference = attribute.Value
				case "s":
					style = attribute.Value
				}
			}
			if reference != "" && style != "" && style != "0" {
				references = append(references, reference)
			}
		case xml.EndElement:
			if token.Name.Local == "sheetData" {
				inSheetData = false
			}
		}
	}
}

func excelizePackagePart(file *excelize.File, name string) ([]byte, bool) {
	value, ok := file.Pkg.Load(name)
	if !ok {
		return nil, false
	}
	data, ok := value.([]byte)
	return data, ok
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
	_, err := saveFile(session, path, false)
	return err
}

// ForceSaveFile saves despite package-preservation audit failures after backing up an existing target.
func ForceSaveFile(session *Session, path string) (string, error) {
	return saveFile(session, path, true)
}

func saveFile(session *Session, path string, force bool) (string, error) {
	if session == nil || session.Workbook == nil {
		return "", fmt.Errorf("save workbook: session is nil")
	}
	if path == "" {
		path = session.Path
	}
	format, err := formatForPath(path)
	if err != nil {
		return "", err
	}
	conversion := session.Format == CSV && format == XLSX
	if format != session.Format && !conversion {
		return "", fmt.Errorf("save %q as %s would change workbook format from %s", path, format, session.Format)
	}
	if format == CSV {
		return "", saveCSV(session, path)
	}
	if conversion {
		if err := ensureCalculationFile(session); err != nil {
			return "", fmt.Errorf("prepare XLSX conversion for %q: %w", path, err)
		}
	} else if session.file == nil {
		return "", fmt.Errorf("save %q: XLSX session has no open workbook", path)
	}

	backupPath := ""
	if force {
		backupPath, err = backupExistingFile(path)
		if err != nil {
			return "", fmt.Errorf("create backup before force save of %q: %w", path, err)
		}
		if backupPath == "" && session.Path != "" && filepath.Clean(session.Path) != filepath.Clean(path) {
			backupPath, err = backupExistingFile(session.Path)
			if err != nil {
				return "", fmt.Errorf("create backup before force save of %q: %w", path, err)
			}
		}
	}

	tempPath, err := createSiblingTemp(path)
	if err != nil {
		return backupPath, fmt.Errorf("prepare save for %q: %w", path, err)
	}
	defer os.Remove(tempPath)
	if err := session.file.SaveAs(tempPath); err != nil {
		return backupPath, fmt.Errorf("save temporary workbook for %q: %w", path, err)
	}
	if !force && !conversion && session.Path != "" && len(session.OriginalParts) > 0 {
		if err := AuditPackage(session.Path, tempPath, safeWorkbookChanges(session.OriginalParts)); err != nil {
			return "", fmt.Errorf("refusing save of %q to preserve workbook content: %w", path, err)
		}
	}
	parts, err := SnapshotParts(tempPath)
	if err != nil {
		return backupPath, fmt.Errorf("snapshot temporary workbook for %q: %w", path, err)
	}
	if err := preserveMode(tempPath, path); err != nil {
		return backupPath, fmt.Errorf("prepare replacement for %q: %w", path, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return backupPath, fmt.Errorf("replace workbook %q: %w", path, err)
	}
	session.Format = format
	session.Workbook.Format = format
	session.Path = path
	session.Workbook.Path = path
	session.OriginalParts = parts
	session.Workbook.Dirty = false
	return backupPath, nil
}

func backupExistingFile(path string) (string, error) {
	source, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return "", err
	}

	for suffix := 0; ; suffix++ {
		backupPath := path + ".bak"
		if suffix > 0 {
			backupPath = fmt.Sprintf("%s.bak.%d", path, suffix)
		}
		backup, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(backup, source)
		syncErr := backup.Sync()
		closeErr := backup.Close()
		if copyErr != nil || syncErr != nil || closeErr != nil {
			_ = os.Remove(backupPath)
			if copyErr != nil {
				return "", copyErr
			}
			if syncErr != nil {
				return "", syncErr
			}
			return "", closeErr
		}
		return backupPath, nil
	}
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
