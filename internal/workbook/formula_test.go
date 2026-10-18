package workbook

import (
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func setFormulaLiteral(t *testing.T, session *Session, sheet, address, input string) {
	t.Helper()
	var value any = input
	if integer, err := strconv.ParseInt(input, 10, 64); err == nil {
		value = integer
	} else if number, err := strconv.ParseFloat(input, 64); err == nil {
		value = number
	}
	if err := session.file.SetCellValue(sheet, address, value); err != nil {
		t.Fatalf("SetCellValue %s!%s: %v", sheet, address, err)
	}
	setModelCell(session, sheet, address, Cell{Input: input, Result: input, CalculationState: Literal})
}

func TestCommonFormulaFunctions(t *testing.T) {
	session := newFormulaSession(t)
	setFormulaLiteral(t, session, "Sheet1", "A1", "2")
	setFormulaLiteral(t, session, "Sheet1", "A2", "3")
	setFormula(t, session, "Sheet1", "B1", "A1+A2")
	setFormula(t, session, "Sheet1", "B2", "SUM(A1:A2)")
	setFormula(t, session, "Sheet1", "B3", "AVERAGE(A1:A2)")
	setFormula(t, session, "Sheet1", "B4", "COUNT(A1:A2)")
	setFormula(t, session, "Sheet1", "B5", "MIN(A1:A2)")
	setFormula(t, session, "Sheet1", "B6", "MAX(A1:A2)")
	setFormula(t, session, "Sheet1", "B7", `IF(A1>A2,"yes","no")`)
	setFormula(t, session, "Other", "A1", "Sheet1!A1+1")
	setFormula(t, session, "Other", "A2", "SUM(Sheet1!A1:A2)")

	if err := Recalculate(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	want := map[CellAddress]string{
		{Sheet: "Sheet1", Row: 0, Col: 1}: "5",
		{Sheet: "Sheet1", Row: 1, Col: 1}: "5",
		{Sheet: "Sheet1", Row: 2, Col: 1}: "2.5",
		{Sheet: "Sheet1", Row: 3, Col: 1}: "2",
		{Sheet: "Sheet1", Row: 4, Col: 1}: "2",
		{Sheet: "Sheet1", Row: 5, Col: 1}: "3",
		{Sheet: "Sheet1", Row: 6, Col: 1}: "no",
		{Sheet: "Other", Row: 0, Col: 0}:  "3",
		{Sheet: "Other", Row: 1, Col: 0}:  "5",
	}
	for address, result := range want {
		cell := formulaCell(t, session, address)
		if cell.Result != result || cell.CalculationState != Current {
			t.Errorf("%s!row %d col %d = (%q, %q), want (%q, current)", address.Sheet, address.Row, address.Col, cell.Result, cell.CalculationState, result)
		}
	}
}

func TestFormulaDependencyRecalculation(t *testing.T) {
	session := newFormulaSession(t)
	setFormulaLiteral(t, session, "Sheet1", "A1", "2")
	setFormulaLiteral(t, session, "Sheet1", "A2", "3")
	setFormula(t, session, "Sheet1", "B1", "A1+A2")
	setFormula(t, session, "Other", "A1", "Sheet1!B1*2")
	if err := Recalculate(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}); err != nil {
		t.Fatalf("initial Recalculate: %v", err)
	}

	if err := session.file.SetCellValue("Sheet1", "A1", 7); err != nil {
		t.Fatalf("change precedent: %v", err)
	}
	address := CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}
	cell := formulaCell(t, session, address)
	cell.Input = "7"
	cell.Result = "7"
	cell.CalculationState = Literal
	session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 0}] = cell
	if err := Recalculate(session, address); err != nil {
		t.Fatalf("Recalculate after edit: %v", err)
	}
	if got := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 1}).Result; got != "10" {
		t.Fatalf("dependent result = %q, want 10", got)
	}
	if got := formulaCell(t, session, CellAddress{Sheet: "Other", Row: 0, Col: 0}).Result; got != "20" {
		t.Fatalf("cross-sheet dependent result = %q, want 20", got)
	}
}

func TestUnsupportedFormulaIsPreservedAndStale(t *testing.T) {
	session := newFormulaSession(t)
	setFormula(t, session, "Sheet1", "A1", "NO_SUCH_FUNCTION(1)")
	cell := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0})
	cell.Result = "cached"
	session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 0}] = cell
	if err := Recalculate(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	cell = formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0})
	if cell.Input != "=NO_SUCH_FUNCTION(1)" {
		t.Fatalf("formula source changed to %q", cell.Input)
	}
	if cell.CalculationState != Stale {
		t.Fatalf("calculation state = %q, want stale", cell.CalculationState)
	}
	if cell.Result != "cached" {
		t.Fatalf("cached result = %q, want preserved stale result", cell.Result)
	}
}

func TestFormulaErrorIsDistinctFromLiteral(t *testing.T) {
	session := newFormulaSession(t)
	setFormula(t, session, "Sheet1", "A1", "1/0")
	setFormulaLiteral(t, session, "Sheet1", "A2", "#DIV/0!")
	if err := Recalculate(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	formula := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0})
	literal := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 1, Col: 0})
	if formula.CalculationState != Error || formula.Result != "#DIV/0!" {
		t.Fatalf("formula error cell = (%q, %q), want (#DIV/0!, error)", formula.Result, formula.CalculationState)
	}
	if literal.CalculationState != Literal || literal.Result != "#DIV/0!" {
		t.Fatalf("literal cell = (%q, %q), want (#DIV/0!, literal)", literal.Result, literal.CalculationState)
	}
}

func newFormulaSession(t *testing.T) *Session {
	t.Helper()
	file := excelize.NewFile()
	t.Cleanup(func() { _ = file.Close() })
	if _, err := file.NewSheet("Other"); err != nil {
		t.Fatalf("NewSheet: %v", err)
	}
	return &Session{
		Workbook: &Workbook{Format: XLSX, Sheets: []Sheet{
			{Name: "Sheet1", Cells: make(map[Coord]Cell)},
			{Name: "Other", Cells: make(map[Coord]Cell)},
		}},
		Format: XLSX,
		file:   file,
	}
}

func setFormula(t *testing.T, session *Session, sheet, address, formula string) {
	t.Helper()
	if err := session.file.SetCellFormula(sheet, address, formula); err != nil {
		t.Fatalf("SetCellFormula %s!%s: %v", sheet, address, err)
	}
	setModelCell(session, sheet, address, Cell{Input: "=" + formula, CalculationState: Stale})
}

func setModelCell(session *Session, sheet, address string, cell Cell) {
	column, row, err := excelize.CellNameToCoordinates(address)
	if err != nil {
		panic(err)
	}
	for index := range session.Workbook.Sheets {
		if session.Workbook.Sheets[index].Name != sheet {
			continue
		}
		sheetModel := &session.Workbook.Sheets[index]
		sheetModel.Cells[Coord{Row: row - 1, Col: column - 1}] = cell
		if row > sheetModel.Rows {
			sheetModel.Rows = row
		}
		if column > sheetModel.Cols {
			sheetModel.Cols = column
		}
		return
	}
}

func formulaCell(t *testing.T, session *Session, address CellAddress) Cell {
	t.Helper()
	for _, sheet := range session.Workbook.Sheets {
		if sheet.Name == address.Sheet {
			return sheet.Cells[Coord{Row: address.Row, Col: address.Col}]
		}
	}
	t.Fatalf("sheet %q not found", address.Sheet)
	return Cell{}
}

func TestCSVFormulaCalculationUsesSessionWorkbook(t *testing.T) {
	workbook, err := ReadCSV(strings.NewReader("2,=SUM(A1:A1)\n"), "values.csv")
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	session := &Session{Workbook: workbook, Path: "values.csv", Format: CSV}
	address := CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}
	if err := Recalculate(session, address); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	formula := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 1})
	if formula.Result != "2" || formula.CalculationState != Current {
		t.Fatalf("CSV formula = (%q, %q), want (2, current)", formula.Result, formula.CalculationState)
	}
	var output strings.Builder
	if err := WriteCSV(&output, session.Workbook, 0); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	if got := output.String(); got != "2,=SUM(A1:A1)\n" {
		t.Fatalf("CSV export = %q, want formula source", got)
	}
}
