package workbook

import "testing"

func TestWorkbookModel(t *testing.T) {
	book := Workbook{
		Format: CSV,
		Sheets: []Sheet{{
			Name: "Sheet1",
			Rows: 2,
			Cols: 3,
			Cells: map[Coord]Cell{
				{Row: 0, Col: 0}: {Input: "Name", Result: "Name", CalculationState: Literal},
				{Row: 1, Col: 1}: {Input: "Ada", Result: "Ada", CalculationState: Literal},
			},
		}},
		ActiveSheet: 0,
	}

	if got := len(book.Sheets); got != 1 {
		t.Fatalf("sheet count = %d, want 1", got)
	}
	if got := book.Sheets[0].Cells[Coord{Row: 1, Col: 1}].Input; got != "Ada" {
		t.Fatalf("cell B2 input = %q, want Ada", got)
	}
	if _, exists := book.Sheets[0].Cells[Coord{Row: 1, Col: 2}]; exists {
		t.Fatal("blank cell C2 must not be stored in the sparse cell map")
	}
	if book.Sheets[0].Rows != 2 || book.Sheets[0].Cols != 3 {
		t.Fatalf("sheet dimensions = %dx%d, want 2x3", book.Sheets[0].Rows, book.Sheets[0].Cols)
	}
}
