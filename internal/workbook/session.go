package workbook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Session owns editable workbook state and, for XLSX-family files, the original Excelize package.
type Session struct {
	Workbook      *Workbook
	Path          string
	Format        Format
	OriginalParts map[string][32]byte
	Filters       map[string][]ColumnFilter
	file          *excelize.File
	history       historyState
}

// NewWorkbook creates an empty CSV or XLSX session for the no-path flow.
func NewWorkbook(format Format) (*Session, error) {
	switch format {
	case CSV:
		model := &Workbook{
			Format: format,
			Sheets: []Sheet{{Name: "Sheet1", Cells: make(map[Coord]Cell), ColumnWidths: make(map[int]float64)}},
		}
		return &Session{Workbook: model, Format: format}, nil
	case XLSX:
		file := excelize.NewFile()
		model, err := workbookFromExcelize(file, "", format)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		return &Session{Workbook: model, Format: format, file: file}, nil
	default:
		return nil, fmt.Errorf("cannot create a new workbook in format %s", format)
	}
}

// OpenFile loads a CSV, XLSX, or XLSM workbook without reconstructing XLSX-family files.
func OpenFile(path string) (*Session, error) {
	format, err := formatForPath(path)
	if err != nil {
		return nil, err
	}
	if format == CSV {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %q: %w", path, err)
		}
		defer file.Close()
		workbook, err := ReadCSV(file, path)
		if err != nil {
			return nil, err
		}
		session := &Session{Workbook: workbook, Path: path, Format: CSV}
		if err := recalculateLoadedFormulas(session); err != nil {
			return nil, fmt.Errorf("recalculate formulas in %q: %w", path, err)
		}
		return session, nil
	}

	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open workbook %q: %w", path, err)
	}
	workbook, err := workbookFromExcelize(file, path, format)
	if err != nil {
		file.Close()
		return nil, err
	}
	parts, err := SnapshotParts(path)
	if err != nil {
		file.Close()
		return nil, err
	}
	session := &Session{Workbook: workbook, Path: path, Format: format, OriginalParts: parts, file: file}
	if err := recalculateLoadedFormulas(session); err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("recalculate formulas in %q: %w", path, err)
	}
	return session, nil
}
func recalculateLoadedFormulas(session *Session) error {
	for _, sheet := range session.Workbook.Sheets {
		for coord, cell := range sheet.Cells {
			if strings.HasPrefix(cell.Input, "=") {
				return Recalculate(session, CellAddress{Sheet: sheet.Name, Row: coord.Row, Col: coord.Col})
			}
		}
	}
	return nil
}

// Close releases Excelize resources held by the session.
func (session *Session) Close() error {
	if session == nil || session.file == nil {
		return nil
	}
	return session.file.Close()
}

func formatForPath(path string) (Format, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return CSV, nil
	case ".xlsx":
		return XLSX, nil
	case ".xlsm":
		return XLSM, nil
	default:
		return "", fmt.Errorf("unsupported workbook format for %q; use .csv, .xlsx, or .xlsm", path)
	}
}
