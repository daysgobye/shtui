package workbook

import (
	"strings"
	"testing"
)

func TestCSVRoundTripQuotedAndFormulaCells(t *testing.T) {
	input := "name,quoted,notes,formula,tail\nalpha,\"say \"\"hi\"\"\",\"line one\nline two\",=SUM(A1:A2),\n"
	book, err := ReadCSV(strings.NewReader(input), "input.csv")
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(book.Sheets) != 1 {
		t.Fatalf("sheet count = %d, want 1", len(book.Sheets))
	}
	sheet := book.Sheets[0]
	if sheet.Rows != 2 || sheet.Cols != 5 {
		t.Fatalf("dimensions = %dx%d, want 2x5", sheet.Rows, sheet.Cols)
	}
	want := map[Coord]string{
		{Row: 0, Col: 0}: "name",
		{Row: 1, Col: 0}: "alpha",
		{Row: 1, Col: 1}: `say "hi"`,
		{Row: 1, Col: 2}: "line one\nline two",
		{Row: 1, Col: 3}: "=SUM(A1:A2)",
	}
	for coord, value := range want {
		cell, exists := sheet.Cells[coord]
		if !exists || cell.Input != value {
			t.Errorf("cell (%d,%d) = (%q, present %v), want %q", coord.Row, coord.Col, cell.Input, exists, value)
		}
	}
	if _, exists := sheet.Cells[Coord{Row: 1, Col: 4}]; exists {
		t.Fatal("trailing empty cell must remain absent from sparse storage")
	}

	var output strings.Builder
	if err := WriteCSV(&output, book, 0); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	roundTrip, err := ReadCSV(strings.NewReader(output.String()), "round-trip.csv")
	if err != nil {
		t.Fatalf("ReadCSV after WriteCSV: %v", err)
	}
	if roundTrip.Sheets[0].Rows != 2 || roundTrip.Sheets[0].Cols != 5 {
		t.Fatalf("round-trip dimensions = %dx%d, want 2x5", roundTrip.Sheets[0].Rows, roundTrip.Sheets[0].Cols)
	}
	for coord, value := range want {
		if got := roundTrip.Sheets[0].Cells[coord].Input; got != value {
			t.Errorf("round-trip cell (%d,%d) = %q, want %q", coord.Row, coord.Col, got, value)
		}
	}
	if got := output.String(); got != input {
		t.Fatalf("CSV output differs from original\n got: %q\nwant: %q", got, input)
	}
}
