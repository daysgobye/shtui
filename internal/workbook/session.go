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
	file          *excelize.File
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
		return &Session{Workbook: workbook, Path: path, Format: CSV}, nil
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
	return &Session{Workbook: workbook, Path: path, Format: format, OriginalParts: parts, file: file}, nil
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
