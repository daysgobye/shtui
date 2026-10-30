package workbook

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXLSXCellEditRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		check   func(*testing.T, *Session, string)
	}{
		{
			name:    "chart",
			fixture: "chart.xlsx",
			check: func(t *testing.T, session *Session, path string) {
				t.Helper()
				if !packageHasPrefix(t, path, "xl/charts/chart") {
					t.Fatal("chart package part disappeared after save")
				}
			},
		},
		{
			name:    "pivot table",
			fixture: "pivottable.xlsx",
			check: func(t *testing.T, session *Session, path string) {
				t.Helper()
				if !packageHasPrefix(t, path, "xl/pivotTables/pivotTable") || !packageHasPrefix(t, path, "xl/pivotCache/pivotCacheDefinition") {
					t.Fatal("PivotTable or cache package part disappeared after save")
				}
			},
		},
		{
			name:    "validation",
			fixture: "validation.xlsx",
			check: func(t *testing.T, session *Session, path string) {
				t.Helper()
				validations, err := session.file.GetDataValidations("Sheet1")
				if err != nil {
					t.Fatalf("GetDataValidations: %v", err)
				}
				if len(validations) != 1 {
					t.Fatalf("validation count = %d, want 1", len(validations))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := copyFixture(t, test.fixture)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			session, err := OpenFile(path)
			if err != nil {
				t.Fatalf("OpenFile: %v", err)
			}
			if err := session.file.SetCellValue("Sheet1", "A1", "edited"); err != nil {
				t.Fatalf("SetCellValue: %v", err)
			}
			if err := SaveFile(session, path); err != nil {
				after, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if !bytes.Equal(before, after) {
					t.Fatal("refused save changed source bytes")
				}
				if !strings.Contains(strings.ToLower(err.Error()), "preserv") && !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
					t.Fatalf("save refusal is not actionable: %v", err)
				}
				return
			}
			reopened, err := OpenFile(path)
			if err != nil {
				t.Fatalf("reopen saved workbook: %v", err)
			}
			got, err := reopened.file.GetCellValue("Sheet1", "A1")
			if err != nil {
				t.Fatalf("GetCellValue: %v", err)
			}
			if got != "edited" {
				t.Fatalf("saved A1 = %q, want edited", got)
			}
			test.check(t, reopened, path)
		})
	}
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	source := filepath.Join("testdata", "preservation", name)
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read fixture %s: %v", source, err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("copy fixture %s: %v", source, err)
	}
	return path
}

func packageHasPrefix(t *testing.T, path, prefix string) bool {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open package %s: %v", path, err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, prefix) {
			return true
		}
	}
	return false
}

func packagePart(t *testing.T, path, name string) []byte {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open package %s: %v", path, err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open package part %s: %v", name, err)
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatalf("read package part %s: %v", name, readErr)
		}
		if closeErr != nil {
			t.Fatalf("close package part %s: %v", name, closeErr)
		}
		return data
	}
	return nil
}
func TestOpenXLSXRecalculatesSupportedFormula(t *testing.T) {
	session, err := NewWorkbook(XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	sheet := session.Workbook.Sheets[0].Name
	for row, value := range []string{"1", "2", "=SUM(A1:A2)"} {
		if err := SetCell(session, CellAddress{Sheet: sheet, Row: row, Col: 0}, value); err != nil {
			t.Fatalf("SetCell row %d: %v", row, err)
		}
	}
	path := filepath.Join(t.TempDir(), "formula.xlsx")
	if err := SaveFile(session, path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	cell := reopened.Workbook.Sheets[0].Cells[Coord{Row: 2}]
	if cell.Result != "3" || cell.CalculationState != Current {
		t.Fatalf("reopened formula = (%q, %q), want (3, current)", cell.Result, cell.CalculationState)
	}
}

func TestSaveFileConvertsCSVSessionToXLSX(t *testing.T) {
	session := newCSVSessionWithSupportedFeatures(t)
	path := filepath.Join(t.TempDir(), "converted.xlsx")
	if err := SaveFile(session, path); err != nil {
		t.Fatalf("SaveFile CSV as XLSX: %v", err)
	}
	if session.Format != XLSX || session.Workbook.Format != XLSX {
		t.Fatalf("formats after conversion = session %s, workbook %s; want xlsx", session.Format, session.Workbook.Format)
	}
	if session.Path != path || session.Workbook.Path != path || session.Workbook.Dirty {
		t.Fatalf("session after conversion: path=%q workbookPath=%q dirty=%t", session.Path, session.Workbook.Path, session.Workbook.Dirty)
	}

	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile converted workbook: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if got := len(reopened.Workbook.Sheets); got != 2 {
		t.Fatalf("reopened sheet count = %d, want 2", got)
	}
	formula := reopened.Workbook.Sheets[0].Cells[Coord{Row: 2}]
	if formula.Input != "=SUM(A1:A2)" || formula.Result != "5" || formula.CalculationState != Current {
		t.Fatalf("reopened formula = (%q, %q, %q)", formula.Input, formula.Result, formula.CalculationState)
	}
	style := reopened.Workbook.Sheets[0].Cells[Coord{}].Style
	if style.FillRGB != "123456" || style.FontRGB != "ABCDEF" || style.Borders.Top.Preset != "thick" {
		t.Fatalf("reopened direct style = %#v", style)
	}
	width, err := reopened.file.GetColWidth("Sheet1", "B")
	if err != nil {
		t.Fatalf("GetColWidth after reopen: %v", err)
	}
	if width != 22 {
		t.Fatalf("reopened XLSX column B width = %v, want 22", width)
	}
	if got := reopened.Workbook.Sheets[0].Merges; len(got) != 1 || got[0] != (CellRange{Start: Coord{Row: 3}, End: Coord{Row: 3, Col: 1}}) {
		t.Fatalf("reopened merges = %#v", got)
	}
	if err := LoadColorScales(reopened); err != nil {
		t.Fatalf("LoadColorScales: %v", err)
	}
	rules := reopened.Workbook.Sheets[0].ConditionalRules
	if len(rules) != 2 || rules[0].Operator != GreaterThan || rules[1].ColorScale == nil {
		t.Fatalf("reopened conditional rules = %#v", rules)
	}
	if got := reopened.Workbook.Sheets[1].Cells[Coord{}].Input; got != "second sheet" {
		t.Fatalf("reopened second sheet A1 = %q, want %q", got, "second sheet")
	}
}

func TestSaveFileConversionInitializesUntouchedCSVSession(t *testing.T) {
	session, err := NewWorkbook(CSV)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	session.Workbook.Sheets[0].Cells[Coord{}] = Cell{Input: "untouched", Result: "untouched", CalculationState: Literal}
	session.Workbook.Sheets[0].Rows = 1
	session.Workbook.Sheets[0].Cols = 1
	session.Workbook.Dirty = true
	if session.file != nil {
		t.Fatal("new CSV session unexpectedly has an Excelize package")
	}

	path := filepath.Join(t.TempDir(), "initialized.xlsx")
	if err := SaveFile(session, path); err != nil {
		t.Fatalf("SaveFile untouched CSV session: %v", err)
	}
	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile converted workbook: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if got := reopened.Workbook.Sheets[0].Cells[Coord{}].Input; got != "untouched" {
		t.Fatalf("reopened A1 = %q, want untouched", got)
	}
}

func TestSaveFileConversionFailureLeavesCSVSessionUnchanged(t *testing.T) {
	session, err := NewWorkbook(CSV)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	session.Path = "original.csv"
	session.Workbook.Path = session.Path
	session.Workbook.Sheets[0].Cells[Coord{}] = Cell{Input: "keep", Result: "keep", CalculationState: Literal}
	session.Workbook.Sheets[0].Rows = 1
	session.Workbook.Sheets[0].Cols = 1
	session.Workbook.Dirty = true
	destination := filepath.Join(t.TempDir(), "missing", "converted.xlsx")
	if err := SaveFile(session, destination); err == nil {
		t.Fatal("SaveFile to a missing parent succeeded")
	}
	if session.Format != CSV || session.Workbook.Format != CSV {
		t.Fatalf("failed conversion formats = session %s, workbook %s; want csv", session.Format, session.Workbook.Format)
	}
	if session.Path != "original.csv" || session.Workbook.Path != "original.csv" || !session.Workbook.Dirty {
		t.Fatalf("failed conversion changed session metadata: path=%q workbookPath=%q dirty=%t", session.Path, session.Workbook.Path, session.Workbook.Dirty)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{}].Input; got != "keep" {
		t.Fatalf("failed conversion changed A1 to %q", got)
	}
}

func newCSVSessionWithSupportedFeatures(t *testing.T) *Session {
	t.Helper()
	session, err := NewWorkbook(CSV)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	for row, value := range []string{"2", "3", "=SUM(A1:A2)"} {
		if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: row, Col: 0}, value); err != nil {
			t.Fatalf("SetCell row %d: %v", row, err)
		}
	}
	if err := AddSheet(session, "Data"); err != nil {
		t.Fatalf("AddSheet: %v", err)
	}
	if err := SetCell(session, CellAddress{Sheet: "Data", Row: 0, Col: 0}, "second sheet"); err != nil {
		t.Fatalf("SetCell second sheet: %v", err)
	}
	if err := SetColumnWidth(session, "Sheet1", 1, 22); err != nil {
		t.Fatalf("SetColumnWidth: %v", err)
	}
	style := CellStyle{
		FillRGB: "123456",
		FontRGB: "ABCDEF",
		Borders: BorderSet{Top: BorderStyle{Preset: "thick", ColorRGB: "FF0000"}},
	}
	if err := ApplyStyle(session, "Sheet1", CellRange{}, style); err != nil {
		t.Fatalf("ApplyStyle: %v", err)
	}
	if _, err := MergeRange(session, "Sheet1", CellRange{Start: Coord{Row: 3}, End: Coord{Row: 3, Col: 1}}, false); err != nil {
		t.Fatalf("MergeRange: %v", err)
	}
	if err := AddConditionalRule(session, "Sheet1", CellRange{Start: Coord{}, End: Coord{Row: 1}}, ConditionalRule{
		Operator: GreaterThan, Value: "0", FillRGB: "00FF00",
	}); err != nil {
		t.Fatalf("AddConditionalRule: %v", err)
	}
	if err := AddColorScaleRule(session, "Sheet1", CellRange{Start: Coord{Col: 1}, End: Coord{Row: 1, Col: 1}}, ColorScale{
		MinRGB: "FF0000", MidRGB: "FFFF00", MaxRGB: "00FF00",
	}); err != nil {
		t.Fatalf("AddColorScaleRule: %v", err)
	}
	return session
}
func TestForceSaveFileBacksUpOriginalBeforeReplacement(t *testing.T) {
	path := copyFixture(t, "chart.xlsx")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	session, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	existingBackup := path + ".bak"
	if err := os.WriteFile(existingBackup, []byte("earlier backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "forced edit"); err != nil {
		t.Fatalf("SetCell: %v", err)
	}

	backupPath, err := ForceSaveFile(session, path)
	if err != nil {
		t.Fatalf("ForceSaveFile: %v", err)
	}
	if backupPath != path+".bak.1" {
		t.Fatalf("backup path = %q, want a new backup beside the existing one", backupPath)
	}
	prior, err := os.ReadFile(existingBackup)
	if err != nil {
		t.Fatal(err)
	}
	if string(prior) != "earlier backup" {
		t.Fatal("force save overwrote the earlier backup")
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup %q: %v", backupPath, err)
	}
	if !bytes.Equal(backup, original) {
		t.Fatal("backup does not exactly preserve the original workbook")
	}
	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatalf("reopen forced save: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	got, err := reopened.file.GetCellValue("Sheet1", "A1")
	if err != nil {
		t.Fatalf("GetCellValue: %v", err)
	}
	if got != "forced edit" {
		t.Fatalf("saved A1 = %q, want forced edit", got)
	}
}

func TestForceSaveAsBacksUpOriginalWhenTargetIsNew(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source.xlsx")
	session, err := NewWorkbook(XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if err := SaveFile(session, sourcePath); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	original, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetCell(session, CellAddress{Sheet: "Sheet1"}, "forced edit"); err != nil {
		t.Fatalf("SetCell: %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "copy.xlsx")
	backupPath, err := ForceSaveFile(session, targetPath)
	if err != nil {
		t.Fatalf("ForceSaveFile: %v", err)
	}
	if backupPath != sourcePath+".bak" {
		t.Fatalf("backup path = %q, want source backup %q", backupPath, sourcePath+".bak")
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(backup, original) {
		t.Fatal("backup does not preserve the source workbook")
	}
	reopened, err := OpenFile(targetPath)
	if err != nil {
		t.Fatalf("reopen target: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	got, err := reopened.file.GetCellValue("Sheet1", "A1")
	if err != nil {
		t.Fatalf("GetCellValue: %v", err)
	}
	if got != "forced edit" {
		t.Fatalf("target A1 = %q, want forced edit", got)
	}
}
