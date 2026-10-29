package workbook

import (
	"reflect"
	"testing"
)

func TestCopyRangePreservesRectangularMatrixAndFormulaSource(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{
		{Row: 0, Col: 1}: "=SUM(A1:A2)",
		{Row: 1, Col: 0}: "south",
	})
	got, err := CopyRange(session, "Sheet1", CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 1, Col: 2}})
	if err != nil {
		t.Fatalf("CopyRange: %v", err)
	}
	want := [][]string{{"", "=SUM(A1:A2)", ""}, {"south", "", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CopyRange = %#v, want %#v", got, want)
	}
}

func TestClearRangeUndoRedo(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{
		{Row: 0, Col: 0}: "left",
		{Row: 0, Col: 1}: "=1+2",
		{Row: 1, Col: 0}: "right",
	})
	selection := CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 1, Col: 1}}
	initialHistory := len(session.history.undo)
	if err := ClearRange(session, "Sheet1", selection); err != nil {
		t.Fatalf("ClearRange: %v", err)
	}
	if got := len(session.history.undo); got != initialHistory+1 {
		t.Fatalf("undo history entries after clear = %d, want %d", got, initialHistory+1)
	}
	cleared, err := CopyRange(session, "Sheet1", selection)
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"", ""}, {"", ""}}; !reflect.DeepEqual(cleared, want) {
		t.Fatalf("range after clear = %#v, want %#v", cleared, want)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo clear: %v", err)
	}
	restored, err := CopyRange(session, "Sheet1", selection)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"left", "=1+2"}, {"right", ""}}
	if !reflect.DeepEqual(restored, want) {
		t.Fatalf("range after undo = %#v, want %#v", restored, want)
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo clear: %v", err)
	}
	cleared, err = CopyRange(session, "Sheet1", selection)
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"", ""}, {"", ""}}; !reflect.DeepEqual(cleared, want) {
		t.Fatalf("range after redo = %#v, want %#v", cleared, want)
	}
}

func TestPasteRangePreservesEmptyBoundariesAndUndoRedo(t *testing.T) {
	session := newOperationSession(t)
	values := [][]string{{"", "first", ""}, {"", "=SUM(A1:A2)", ""}}
	address := CellAddress{Sheet: "Sheet1", Row: 1, Col: 1}
	if err := PasteRange(session, address, values); err != nil {
		t.Fatalf("PasteRange: %v", err)
	}
	selection := CellRange{Start: Coord{Row: 1, Col: 1}, End: Coord{Row: 2, Col: 3}}
	got, err := CopyRange(session, "Sheet1", selection)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, values) {
		t.Fatalf("pasted range = %#v, want %#v", got, values)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo paste: %v", err)
	}
	if _, exists := session.Workbook.Sheets[0].Cells[Coord{Row: 1, Col: 2}]; exists {
		t.Fatal("undo left pasted cell behind")
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo paste: %v", err)
	}
	got, err = CopyRange(session, "Sheet1", selection)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, values) {
		t.Fatalf("range after redo = %#v, want %#v", got, values)
	}
}
