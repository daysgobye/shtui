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

func TestAdditionalFormulaFunctions(t *testing.T) {
	session := newFormulaSession(t)
	setFormulaLiteral(t, session, "Sheet1", "A1", "2")
	setFormulaLiteral(t, session, "Sheet1", "A2", "4")
	setFormulaLiteral(t, session, "Sheet1", "A3", "pear")
	formulas := map[CellAddress]string{
		{Sheet: "Sheet1", Row: 0, Col: 2}: `COUNTIF(A1:A3,"pear")`,
		{Sheet: "Sheet1", Row: 1, Col: 2}: `COUNTIF(A1:A3,">0")`,
		{Sheet: "Sheet1", Row: 2, Col: 2}: `IFERROR(1/0,"safe")`,
		{Sheet: "Sheet1", Row: 3, Col: 2}: `COUNTA(A1:A3)`,
	}
	for address, formula := range formulas {
		cell := Cell{Input: "=" + formula, CalculationState: Stale}
		session.Workbook.Sheets[0].Cells[Coord{Row: address.Row, Col: address.Col}] = cell
		if err := session.file.SetCellFormula("Sheet1", mustCellName(t, address.Row, address.Col), formula); err != nil {
			t.Fatalf("SetCellFormula %s: %v", formula, err)
		}
	}
	if err := Recalculate(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	want := map[CellAddress]string{
		{Sheet: "Sheet1", Row: 0, Col: 2}: "1",
		{Sheet: "Sheet1", Row: 1, Col: 2}: "2",
		{Sheet: "Sheet1", Row: 2, Col: 2}: "safe",
		{Sheet: "Sheet1", Row: 3, Col: 2}: "3",
	}
	for address, result := range want {
		cell := formulaCell(t, session, address)
		if cell.Result != result || cell.CalculationState != Current {
			t.Errorf("%+v formula %q result = (%q, %q), want (%q, current)", address, formulas[address], cell.Result, cell.CalculationState, result)
		}
		if cell.Input != "="+formulas[address] {
			t.Errorf("formula source = %q, want %q", cell.Input, "="+formulas[address])
		}
	}
}
func TestSUMIFUsesCriteriaAndSumRanges(t *testing.T) {
	session := newFormulaSession(t)
	setFormulaLiteral(t, session, "Sheet1", "A1", "Food")
	setFormulaLiteral(t, session, "Sheet1", "A2", "food")
	setFormulaLiteral(t, session, "Sheet1", "A3", "Travel")
	setFormulaLiteral(t, session, "Sheet1", "B1", "12.5")
	setFormulaLiteral(t, session, "Sheet1", "B2", "7.5")
	setFormulaLiteral(t, session, "Sheet1", "B3", "20")
	formula := `SUMIF(A1:A3,"food",B1:B3)`
	setFormula(t, session, "Sheet1", "C1", formula)

	if err := Recalculate(session, CellAddress{Sheet: "Sheet1"}); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	cell := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 2})
	if cell.Input != "="+formula || cell.Result != "20" || cell.CalculationState != Current {
		t.Fatalf("SUMIF cell = (%q, %q, %q), want (%q, 20, current)", cell.Input, cell.Result, cell.CalculationState, "="+formula)
	}
}

func TestSUMIFDefaultsSumRange(t *testing.T) {
	session := newFormulaSession(t)
	setFormulaLiteral(t, session, "Sheet1", "A1", "3")
	setFormulaLiteral(t, session, "Sheet1", "A2", "9")
	setFormulaLiteral(t, session, "Sheet1", "A3", "not a number")
	setFormula(t, session, "Sheet1", "B1", `SUMIF(A1:A3,">2")`)

	if err := Recalculate(session, CellAddress{Sheet: "Sheet1"}); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	cell := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 1})
	if cell.Result != "12" || cell.CalculationState != Current {
		t.Fatalf("SUMIF cell = (%q, %q), want (12, current)", cell.Result, cell.CalculationState)
	}
}

func TestSUMIFMismatchedSumRangeUsesCriteriaShape(t *testing.T) {
	session := newFormulaSession(t)
	setFormulaLiteral(t, session, "Sheet1", "A1", "yes")
	setFormulaLiteral(t, session, "Sheet1", "A2", "yes")
	setFormulaLiteral(t, session, "Sheet1", "A3", "yes")
	setFormulaLiteral(t, session, "Sheet1", "B1", "4")
	setFormulaLiteral(t, session, "Sheet1", "B2", "6")
	setFormulaLiteral(t, session, "Sheet1", "B3", "100")
	setFormula(t, session, "Sheet1", "C1", `SUMIF(A1:A2,"yes",B1:B3)`)
	setFormula(t, session, "Sheet1", "C2", `SUMIF(A1:A3,"yes",B1:B2)`)
	if err := Recalculate(session, CellAddress{Sheet: "Sheet1"}); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	longer := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 2})
	if longer.Result != "10" || longer.CalculationState != Current {
		t.Fatalf("long sum range result = (%q, %q), want (10, current)", longer.Result, longer.CalculationState)
	}
	shorter := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 1, Col: 2})
	if shorter.Result != "110" || shorter.CalculationState != Current {
		t.Fatalf("short sum range result = (%q, %q), want (110, current)", shorter.Result, shorter.CalculationState)
	}
}

func TestSUMIFSourceEditRecalculatesTotal(t *testing.T) {
	session := newFormulaSession(t)
	setFormulaLiteral(t, session, "Sheet1", "A1", "yes")
	setFormulaLiteral(t, session, "Sheet1", "A2", "no")
	setFormulaLiteral(t, session, "Sheet1", "B1", "5")
	setFormulaLiteral(t, session, "Sheet1", "B2", "100")
	setFormula(t, session, "Sheet1", "C1", `SUMIF(A1:A2,"yes",B1:B2)`)
	setFormula(t, session, "Sheet1", "D1", "C1*2")
	address := CellAddress{Sheet: "Sheet1"}
	if err := Recalculate(session, address); err != nil {
		t.Fatalf("initial Recalculate: %v", err)
	}
	if got := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 2}).Result; got != "5" {
		t.Fatalf("initial total = %q, want 5", got)
	}
	if got := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 3}).Result; got != "10" {
		t.Fatalf("initial dependent = %q, want 10", got)
	}
	setFormulaLiteral(t, session, "Sheet1", "A2", "yes")
	if err := Recalculate(session, CellAddress{Sheet: "Sheet1", Row: 1}); err != nil {
		t.Fatalf("Recalculate after source edit: %v", err)
	}
	cell := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 2})
	if cell.Result != "105" || cell.CalculationState != Current {
		t.Fatalf("edited total = (%q, %q), want (105, current)", cell.Result, cell.CalculationState)
	}
	dependent := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 3})
	if dependent.Result != "210" || dependent.CalculationState != Current {
		t.Fatalf("edited dependent = (%q, %q), want (210, current)", dependent.Result, dependent.CalculationState)
	}
}

func TestSUMIFUsesRecalculatedFormulaSumRange(t *testing.T) {
	session := newFormulaSession(t)
	setFormulaLiteral(t, session, "Sheet1", "A1", "1")
	setFormula(t, session, "Sheet1", "B1", "A1*2")
	formula := `SUMIF(A1:A1,">0",B1:B1)`
	setFormula(t, session, "Sheet1", "C1", formula)
	address := CellAddress{Sheet: "Sheet1"}
	if err := Recalculate(session, address); err != nil {
		t.Fatalf("initial Recalculate: %v", err)
	}
	if got := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 1}).Result; got != "2" {
		t.Fatalf("initial formula sum range = %q, want 2", got)
	}

	setFormulaLiteral(t, session, "Sheet1", "A1", "3")
	if err := Recalculate(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}); err != nil {
		t.Fatalf("Recalculate after source edit: %v", err)
	}
	sumRange := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 1})
	sumif := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 2})
	if sumRange.Result != "6" || sumRange.CalculationState != Current {
		t.Fatalf("formula sum range = (%q, %q), want (6, current)", sumRange.Result, sumRange.CalculationState)
	}
	if sumif.Input != "="+formula || sumif.Result != "6" || sumif.CalculationState != Current {
		t.Fatalf("SUMIF = (%q, %q, %q), want (%q, 6, current)", sumif.Input, sumif.Result, sumif.CalculationState, "="+formula)
	}
}


func TestSUMIFDependentUsesCustomNumericCriteriaResult(t *testing.T) {
	session := newFormulaSession(t)
	for row, value := range []string{"2", "4", "-"} {
		setFormulaLiteral(t, session, "Sheet1", mustCellName(t, row, 0), value)
	}
	for row, value := range []string{"10", "20", "30"} {
		setFormulaLiteral(t, session, "Sheet1", mustCellName(t, row, 1), value)
	}
	setFormula(t, session, "Sheet1", "C1", `SUMIF(A1:A3,">0",B1:B3)`)
	setFormula(t, session, "Sheet1", "D1", "C1*2")

	if err := Recalculate(session, CellAddress{Sheet: "Sheet1"}); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	sumif := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 2})
	if sumif.Result != "30" || sumif.CalculationState != Current {
		t.Fatalf("SUMIF = (%q, %q), want (30, current)", sumif.Result, sumif.CalculationState)
	}
	dependent := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 3})
	if dependent.Result != "60" || dependent.CalculationState != Current {
		t.Fatalf("dependent = (%q, %q), want (60, current)", dependent.Result, dependent.CalculationState)
	}
}

func TestSUMIFErrorDoesNotFeedDependentFormula(t *testing.T) {
	session := newFormulaSession(t)
	setFormulaLiteral(t, session, "Sheet1", "A1", "2")
	setFormulaLiteral(t, session, "Sheet1", "B1", "10")
	setFormula(t, session, "Sheet1", "C1", `SUMIF(A1:A1,">",B1:B1)`)
	setFormula(t, session, "Sheet1", "D1", "C1*2")
	if err := Recalculate(session, CellAddress{Sheet: "Sheet1"}); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	sumif := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 2})
	if sumif.CalculationState != Error {
		t.Fatalf("SUMIF state = %q, want error", sumif.CalculationState)
	}
	dependent := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 3})
	if dependent.CalculationState != Error || dependent.Result == "" {
		t.Fatalf("dependent = (%q, %q), want a non-empty calculation error", dependent.Result, dependent.CalculationState)
	}
}

func TestSUMIFMalformedCriteriaOrArgumentsSetError(t *testing.T) {
	for _, formula := range []string{
		`SUMIF(A1:A2,">")`,
		`SUMIF(A2:A1,"yes")`,
		`SUMIF(A1:A2,"yes",B1:B2,C1:C2)`,
	} {
		t.Run(formula, func(t *testing.T) {
			session := newFormulaSession(t)
			setFormula(t, session, "Sheet1", "C1", formula)
			if err := Recalculate(session, CellAddress{Sheet: "Sheet1"}); err != nil {
				t.Fatalf("Recalculate: %v", err)
			}
			cell := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 2})
			if cell.Input != "="+formula || cell.CalculationState != Error || cell.Result == "" {
				t.Fatalf("malformed SUMIF = (%q, %q, %q), want preserved source and non-empty error", cell.Input, cell.Result, cell.CalculationState)
			}
		})
	}
}

func TestNestedSupportedFunctionsRecalculate(t *testing.T) {
	session := newFormulaSession(t)
	setFormulaLiteral(t, session, "Sheet1", "A1", "2")
	setFormulaLiteral(t, session, "Sheet1", "A2", "4")
	setFormulaLiteral(t, session, "Sheet1", "A3", "pear")
	formula := `IFERROR(COUNTIF(A1:A3,">0")/COUNT(A1:A3),0)`
	setFormula(t, session, "Sheet1", "B1", formula)
	if err := Recalculate(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}
	cell := formulaCell(t, session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 1})
	if cell.Input != "="+formula || cell.Result != "1" || cell.CalculationState != Current {
		t.Fatalf("nested formula cell = (%q, %q, %q), want (%q, 1, current)", cell.Input, cell.Result, cell.CalculationState, "="+formula)
	}
}

func mustCellName(t *testing.T, row, col int) string {
	t.Helper()
	name, err := excelize.CoordinatesToCellName(col+1, row+1)
	if err != nil {
		t.Fatal(err)
	}
	return name
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
