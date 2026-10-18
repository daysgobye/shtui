package workbook

import (
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestEditMarksDirtyAndRecalculates(t *testing.T) {
	session := newOperationSession(t)
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "2"); err != nil {
		t.Fatalf("SetCell A1: %v", err)
	}
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 1, Col: 0}, "3"); err != nil {
		t.Fatalf("SetCell A2: %v", err)
	}
	formula := CellAddress{Sheet: "Sheet1", Row: 0, Col: 1}
	if err := SetCell(session, formula, "=A1+A2"); err != nil {
		t.Fatalf("SetCell B1: %v", err)
	}
	if !session.Workbook.Dirty {
		t.Fatal("cell edit did not dirty the workbook")
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 1}].Result; got != "5" {
		t.Fatalf("initial formula result = %q, want 5", got)
	}
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "7"); err != nil {
		t.Fatalf("edit precedent: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 1}].Result; got != "10" {
		t.Fatalf("formula result after edit = %q, want 10", got)
	}
}

func TestCopyRangeReturnsRawFormulaSource(t *testing.T) {
	session := newOperationSession(t)
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "1"); err != nil {
		t.Fatal(err)
	}
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 1, Col: 0}, "2"); err != nil {
		t.Fatal(err)
	}
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 1}, "=SUM(A1:A2)"); err != nil {
		t.Fatal(err)
	}
	got, err := CopyRange(session, "Sheet1", CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 1, Col: 1}})
	if err != nil {
		t.Fatalf("CopyRange: %v", err)
	}
	want := [][]string{{"1", "=SUM(A1:A2)"}, {"2", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CopyRange = %#v, want %#v", got, want)
	}
}

func TestPasteRangePreservesAlignmentAndUndoRedo(t *testing.T) {
	session := newOperationSession(t)
	values := [][]string{{"north", "east"}, {"south", ""}}
	if err := PasteRange(session, CellAddress{Sheet: "Sheet1", Row: 1, Col: 1}, values); err != nil {
		t.Fatalf("PasteRange: %v", err)
	}
	got, err := CopyRange(session, "Sheet1", CellRange{Start: Coord{Row: 1, Col: 1}, End: Coord{Row: 2, Col: 2}})
	if err != nil {
		t.Fatalf("CopyRange: %v", err)
	}
	if !reflect.DeepEqual(got, values) {
		t.Fatalf("pasted range = %#v, want %#v", got, values)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo paste: %v", err)
	}
	if _, exists := session.Workbook.Sheets[0].Cells[Coord{Row: 1, Col: 1}]; exists {
		t.Fatal("undo left pasted value behind")
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo paste: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 2, Col: 1}].Input; got != "south" {
		t.Fatalf("redo pasted value = %q, want south", got)
	}
}

func TestInsertRowsShiftDataAndFormulaReferences(t *testing.T) {
	session := newOperationSession(t)

	if err := AddSheet(session, "Other"); err != nil {
		t.Fatal(err)
	}
	setCellInputs(t, session, map[Coord]string{
		{Row: 0, Col: 0}: "1",
		{Row: 1, Col: 0}: "2",
		{Row: 2, Col: 1}: "=A1+A2",
	})
	if err := SetCell(session, CellAddress{Sheet: "Other", Row: 0, Col: 0}, "=SUM(Sheet1!A1:A2)"); err != nil {
		t.Fatal(err)
	}
	if err := InsertRows(session, "Sheet1", 1, 1); err != nil {
		t.Fatalf("InsertRows: %v", err)
	}
	shifted := session.Workbook.Sheets[0]
	if shifted.Cells[Coord{Row: 2, Col: 0}].Input != "2" {
		t.Fatal("row insertion lost the cell below the insertion point")
	}
	formula := shifted.Cells[Coord{Row: 3, Col: 1}]
	if formula.Input != "=A1+A3" || formula.Result != "3" {
		t.Fatalf("shifted formula = (%q, %q), want (=A1+A3, 3)", formula.Input, formula.Result)
	}
	crossSheetFormula := session.Workbook.Sheets[1].Cells[Coord{Row: 0, Col: 0}]
	if crossSheetFormula.Input != "=SUM(Sheet1!A1:A3)" || crossSheetFormula.Result != "3" {
		t.Fatalf("cross-sheet formula after insert = (%q, %q), want (=SUM(Sheet1!A1:A3), 3)", crossSheetFormula.Input, crossSheetFormula.Result)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo insert rows: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 2, Col: 1}].Input; got != "=A1+A2" {
		t.Fatalf("formula after undo = %q, want =A1+A2", got)
	}
	if got := session.Workbook.Sheets[1].Cells[Coord{Row: 0, Col: 0}].Input; got != "=SUM(Sheet1!A1:A2)" {
		t.Fatalf("cross-sheet formula after undo = %q, want original source", got)
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo insert rows: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 3, Col: 1}].Input; got != "=A1+A3" {
		t.Fatalf("formula after redo = %q, want =A1+A3", got)
	}
}

func TestInsertColumnsShiftDataAndFormulaReferences(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{
		{Row: 0, Col: 0}: "1",
		{Row: 0, Col: 1}: "2",
		{Row: 1, Col: 2}: "=A1+B1",
	})
	if err := InsertColumns(session, "Sheet1", 1, 1); err != nil {
		t.Fatalf("InsertColumns: %v", err)
	}
	sheet := session.Workbook.Sheets[0]
	if sheet.Cells[Coord{Row: 0, Col: 2}].Input != "2" {
		t.Fatal("column insertion lost the cell to the right of insertion point")
	}
	formula := sheet.Cells[Coord{Row: 1, Col: 3}]
	if formula.Input != "=A1+C1" || formula.Result != "3" {
		t.Fatalf("shifted formula = (%q, %q), want (=A1+C1, 3)", formula.Input, formula.Result)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo insert columns: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 1, Col: 2}].Input; got != "=A1+B1" {
		t.Fatalf("formula after undo = %q, want =A1+B1", got)
	}
}

func TestSheetOperationsMaintainActiveSheet(t *testing.T) {
	session := newOperationSession(t)
	if err := AddSheet(session, "Totals"); err != nil {
		t.Fatalf("AddSheet: %v", err)
	}
	if session.Workbook.ActiveSheet != 1 || len(session.Workbook.Sheets) != 2 {
		t.Fatalf("after add: active=%d sheets=%d", session.Workbook.ActiveSheet, len(session.Workbook.Sheets))
	}
	if err := AddSheet(session, "totals"); err == nil {
		t.Fatal("duplicate sheet name was accepted")
	}
	if err := RenameSheet(session, "Totals", "Summary"); err != nil {
		t.Fatalf("RenameSheet: %v", err)
	}
	if err := DeleteSheet(session, "Summary"); err != nil {
		t.Fatalf("DeleteSheet: %v", err)
	}
	if session.Workbook.ActiveSheet != 0 || len(session.Workbook.Sheets) != 1 {
		t.Fatalf("after delete: active=%d sheets=%d", session.Workbook.ActiveSheet, len(session.Workbook.Sheets))
	}
	if err := DeleteSheet(session, "Sheet1"); err == nil {
		t.Fatal("deleting the final sheet was accepted")
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo sheet delete: %v", err)
	}
	if len(session.Workbook.Sheets) != 2 || session.Workbook.Sheets[1].Name != "Summary" {
		t.Fatalf("undo delete did not restore sheet: %#v", session.Workbook.Sheets)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo sheet rename: %v", err)
	}
	if session.Workbook.Sheets[1].Name != "Totals" {
		t.Fatalf("undo rename name = %q, want Totals", session.Workbook.Sheets[1].Name)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo sheet add: %v", err)
	}
	if len(session.Workbook.Sheets) != 1 || session.Workbook.ActiveSheet != 0 {
		t.Fatalf("undo add left invalid workbook: active=%d sheets=%d", session.Workbook.ActiveSheet, len(session.Workbook.Sheets))
	}
}

func newOperationSession(t *testing.T) *Session {
	t.Helper()
	file := excelize.NewFile()
	t.Cleanup(func() { _ = file.Close() })
	return &Session{
		Workbook: &Workbook{Format: XLSX, Sheets: []Sheet{{Name: "Sheet1", Cells: make(map[Coord]Cell), ColumnWidths: make(map[int]float64)}}},
		Format:   XLSX,
		file:     file,
	}
}

func setCellInputs(t *testing.T, session *Session, inputs map[Coord]string) {
	t.Helper()
	for coord, input := range inputs {
		if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: coord.Row, Col: coord.Col}, input); err != nil {
			t.Fatalf("SetCell (%d,%d): %v", coord.Row, coord.Col, err)
		}
	}
}

func TestRenameSheetUpdatesFormulaReferences(t *testing.T) {
	session := newOperationSession(t)
	if err := AddSheet(session, "Data"); err != nil {
		t.Fatal(err)
	}
	if err := SetCell(session, CellAddress{Sheet: "Data", Row: 0, Col: 0}, "4"); err != nil {
		t.Fatal(err)
	}
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "2"); err != nil {
		t.Fatal(err)
	}
	formulaAddress := CellAddress{Sheet: "Sheet1", Row: 0, Col: 1}
	if err := SetCell(session, formulaAddress, "=SUM(Data!A1,A1)"); err != nil {
		t.Fatal(err)
	}
	if err := RenameSheet(session, "Data", "Data Sheet"); err != nil {
		t.Fatalf("RenameSheet: %v", err)
	}
	formula := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 1}]
	if formula.Input != "=SUM('Data Sheet'!A1,A1)" || formula.Result != "6" {
		t.Fatalf("renamed formula = (%q, %q), want (=SUM('Data Sheet'!A1,A1), 6)", formula.Input, formula.Result)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo rename: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 1}].Input; got != "=SUM(Data!A1,A1)" {
		t.Fatalf("formula after undo = %q, want original source", got)
	}
}
