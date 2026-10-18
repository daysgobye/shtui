package workbook

import (
	"archive/zip"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestSortKeepsRowsAligned(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{
		{Row: 0, Col: 0}: "Amount", {Row: 0, Col: 1}: "Name", {Row: 0, Col: 2}: "Tag",
		{Row: 1, Col: 0}: "10", {Row: 1, Col: 1}: "ten", {Row: 1, Col: 2}: "first",
		{Row: 2, Col: 0}: "2", {Row: 2, Col: 1}: "two-a", {Row: 2, Col: 2}: "second",
		{Row: 3, Col: 0}: "Apple", {Row: 3, Col: 1}: "fruit", {Row: 3, Col: 2}: "third",
		{Row: 4, Col: 0}: "2", {Row: 4, Col: 1}: "two-b", {Row: 4, Col: 2}: "fourth",
	})
	session.history = historyState{}
	selection := CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 4, Col: 2}}
	if err := SortRange(session, "Sheet1", selection, 0, true, true); err != nil {
		t.Fatalf("SortRange: %v", err)
	}
	want := map[Coord]string{
		{Row: 0, Col: 0}: "Amount", {Row: 0, Col: 1}: "Name", {Row: 0, Col: 2}: "Tag",
		{Row: 1, Col: 0}: "2", {Row: 1, Col: 1}: "two-a", {Row: 1, Col: 2}: "second",
		{Row: 2, Col: 0}: "2", {Row: 2, Col: 1}: "two-b", {Row: 2, Col: 2}: "fourth",
		{Row: 3, Col: 0}: "10", {Row: 3, Col: 1}: "ten", {Row: 3, Col: 2}: "first",
		{Row: 4, Col: 0}: "Apple", {Row: 4, Col: 1}: "fruit", {Row: 4, Col: 2}: "third",
	}
	for coord, expected := range want {
		if got := session.Workbook.Sheets[0].Cells[coord].Input; got != expected {
			t.Errorf("cell %v = %q, want %q", coord, got, expected)
		}
	}
}

func TestSortRejectsMergedRange(t *testing.T) {
	session := newOperationSession(t)
	selection := CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 1, Col: 1}}
	if _, err := MergeRange(session, "Sheet1", selection, true); err != nil {
		t.Fatal(err)
	}
	if err := SortRange(session, "Sheet1", selection, 0, true, false); err == nil {
		t.Fatal("SortRange succeeded on a merged range")
	}
}

func TestSortUndoRedo(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{
		{Row: 0, Col: 0}: "b", {Row: 0, Col: 1}: "row-b",
		{Row: 1, Col: 0}: "a", {Row: 1, Col: 1}: "row-a",
	})
	session.history = historyState{}
	selection := CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 1, Col: 1}}
	if err := SortRange(session, "Sheet1", selection, 0, true, false); err != nil {
		t.Fatal(err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 1}].Input; got != "row-a" {
		t.Fatalf("sorted row value = %q, want row-a", got)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo sort: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 1}].Input; got != "row-b" {
		t.Fatalf("undo row value = %q, want row-b", got)
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo sort: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 1}].Input; got != "row-a" {
		t.Fatalf("redo row value = %q, want row-a", got)
	}
}

func TestFilterDoesNotMutateCells(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{
		{Row: 0, Col: 0}: "Status", {Row: 0, Col: 1}: "Name",
		{Row: 1, Col: 0}: "keep", {Row: 1, Col: 1}: "one",
		{Row: 2, Col: 0}: "drop", {Row: 2, Col: 1}: "two",
		{Row: 3, Col: 0}: "KEEP", {Row: 3, Col: 1}: "three",
	})
	sheet := &session.Workbook.Sheets[0]
	before := make(map[Coord]Cell, len(sheet.Cells))
	for coord, cell := range sheet.Cells {
		before[coord] = cell
	}
	selection := CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 3, Col: 1}}
	filter := ColumnFilter{Column: 0, AllowedValues: map[string]bool{"keep": true}}
	if err := SetFilter(session, "Sheet1", selection, []ColumnFilter{filter}); err != nil {
		t.Fatalf("SetFilter: %v", err)
	}
	if got, want := VisibleRows(sheet, session.Filters["Sheet1"]), []int{0, 1, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("VisibleRows = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(sheet.Cells, before) {
		t.Fatal("filter changed cell contents")
	}
}

func TestFilterPersistsXLSX(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{
		{Row: 0, Col: 0}: "Status",
		{Row: 1, Col: 0}: "keep value",
		{Row: 2, Col: 0}: "drop",
	})
	selection := CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 2, Col: 0}}
	if err := SetFilter(session, "Sheet1", selection, []ColumnFilter{{Column: 0, AllowedValues: map[string]bool{"keep value": true}}}); err != nil {
		t.Fatalf("SetFilter: %v", err)
	}
	path := t.TempDir() + "/filtered.xlsx"
	if err := SaveFile(session, path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		xml := string(content)
		if !strings.Contains(xml, "<autoFilter") || !strings.Contains(xml, "filterColumn") || !strings.Contains(xml, "keep") {
			t.Fatalf("worksheet did not serialize filter criteria: %s", xml)
		}
		return
	}
	t.Fatal("worksheet XML part not found")
}

func TestFindActiveSheetSubstring(t *testing.T) {
	session := newOperationSession(t)
	if err := AddSheet(session, "Other"); err != nil {
		t.Fatal(err)
	}
	setCellInputs(t, session, map[Coord]string{{Row: 0, Col: 0}: "Orchard", {Row: 2, Col: 1}: "red orchard"})
	if err := SetCell(session, CellAddress{Sheet: "Other", Row: 1, Col: 0}, "Orchid"); err != nil {
		t.Fatal(err)
	}
	got := Find(session, "Sheet1", "ORCH")
	want := []CellAddress{{Sheet: "Sheet1", Row: 0, Col: 0}, {Sheet: "Sheet1", Row: 2, Col: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Find = %#v, want %#v", got, want)
	}
}
