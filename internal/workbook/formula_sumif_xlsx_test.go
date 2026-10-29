package workbook

import (
	"path/filepath"
	"testing"
)

func TestSUMIFLoadedFromXLSXRecalculates(t *testing.T) {
	session, err := NewWorkbook(XLSX)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	sheet := session.Workbook.Sheets[0].Name
	for row, value := range []string{"tea", "coffee", "tea"} {
		if err := SetCell(session, CellAddress{Sheet: sheet, Row: row, Col: 0}, value); err != nil {
			t.Fatalf("SetCell category row %d: %v", row, err)
		}
	}
	for row, value := range []string{"4", "9", "6"} {
		if err := SetCell(session, CellAddress{Sheet: sheet, Row: row, Col: 1}, value); err != nil {
			t.Fatalf("SetCell amount row %d: %v", row, err)
		}
	}
	formula := `=SUMIF(A1:A3,"tea",B1:B3)`
	if err := SetCell(session, CellAddress{Sheet: sheet, Row: 0, Col: 2}, formula); err != nil {
		t.Fatalf("SetCell formula: %v", err)
	}
	path := filepath.Join(t.TempDir(), "sumif.xlsx")
	if err := SaveFile(session, path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	cell := reopened.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 2}]
	if cell.Input != formula || cell.Result != "10" || cell.CalculationState != Current {
		t.Fatalf("loaded SUMIF = (%q, %q, %q), want (%q, 10, current)", cell.Input, cell.Result, cell.CalculationState, formula)
	}
}
