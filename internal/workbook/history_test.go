package workbook

import (
	"strconv"
	"testing"
)

func TestUndoRedoCellEdit(t *testing.T) {
	session := newOperationSession(t)
	address := CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}
	if err := SetCell(session, address, "first"); err != nil {
		t.Fatal(err)
	}
	if err := SetCell(session, address, "second"); err != nil {
		t.Fatal(err)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 0}].Input; got != "first" {
		t.Fatalf("undo value = %q, want first", got)
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 0}].Input; got != "second" {
		t.Fatalf("redo value = %q, want second", got)
	}
}

func TestUndoHistoryIsBoundedToMostRecentHundred(t *testing.T) {
	session := newOperationSession(t)
	address := CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}
	if err := SetCell(session, address, "0"); err != nil {
		t.Fatal(err)
	}
	for value := range 101 {
		if err := SetCell(session, address, strconv.Itoa(value+1)); err != nil {
			t.Fatalf("SetCell %d: %v", value+1, err)
		}
	}
	for index := range 100 {
		if err := Undo(session); err != nil {
			t.Fatalf("Undo %d: %v", index+1, err)
		}
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 0}].Input; got != "1" {
		t.Fatalf("value after 100 undo steps = %q, want 1", got)
	}
	if err := Undo(session); err == nil {
		t.Fatal("history retained more than the most recent 100 operations")
	}
}
