package workbook

import "testing"

func TestBuiltinNumberFormatKeepsExcelizeID(t *testing.T) {
	style, err := styleToExcelize(CellStyle{NumberFormat: "builtin:4"})
	if err != nil {
		t.Fatal(err)
	}
	if style.NumFmt != 4 || style.CustomNumFmt != nil {
		t.Fatalf("built-in format conversion = %#v", style)
	}
}

func TestStyleAndColumnWidthRoundTrip(t *testing.T) {
	session := newOperationSession(t)
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "42"); err != nil {
		t.Fatal(err)
	}
	style := CellStyle{
		FillRGB:      "CCFFCC",
		FontRGB:      "003366",
		NumberFormat: "0.00",
		Bold:         true,
		Borders:      BorderSet{Bottom: BorderStyle{Preset: "thin", ColorRGB: "112233"}},
	}
	if err := ApplyStyle(session, "Sheet1", CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 0, Col: 0}}, style); err != nil {
		t.Fatalf("ApplyStyle: %v", err)
	}
	if err := SetColumnWidth(session, "Sheet1", 0, 22.5); err != nil {
		t.Fatalf("SetColumnWidth: %v", err)
	}
	path := t.TempDir() + "/styled.xlsx"
	if err := SaveFile(session, path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer reopened.Close()
	got := reopened.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 0}].Style
	if got.FillRGB != style.FillRGB || got.FontRGB != style.FontRGB || got.NumberFormat != style.NumberFormat || !got.Bold || got.Borders.Bottom.Preset != "thin" {
		t.Fatalf("reopened style = %#v, want %#v", got, style)
	}
	width, err := reopened.file.GetColWidth("Sheet1", "A")
	if err != nil {
		t.Fatalf("GetColWidth: %v", err)
	}
	if width != 22.5 {
		t.Fatalf("reopened column width = %v, want 22.5", width)
	}
}

func TestUndoColumnWidth(t *testing.T) {
	session := newOperationSession(t)
	session.history = historyState{}
	initialWidth, err := session.file.GetColWidth("Sheet1", "B")
	if err != nil {
		t.Fatal(err)
	}
	if err := SetColumnWidth(session, "Sheet1", 1, 24); err != nil {
		t.Fatal(err)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo column width: %v", err)
	}
	restoredWidth, err := session.file.GetColWidth("Sheet1", "B")
	if err != nil {
		t.Fatal(err)
	}
	if restoredWidth != initialWidth {
		t.Fatalf("undo width = %v, want prior width %v", restoredWidth, initialWidth)
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo column width: %v", err)
	}
	if got := session.Workbook.Sheets[0].ColumnWidths[1]; got != 24 {
		t.Fatalf("redo width = %v, want 24", got)
	}
}

func TestMergeConfirmationUnmergeAndUndo(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{
		{Row: 0, Col: 0}: "top-left",
		{Row: 1, Col: 1}: "discard me",
	})
	session.history = historyState{}
	selection := CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 1, Col: 1}}
	beforeDirty := session.Workbook.Dirty
	result, err := MergeRange(session, "Sheet1", selection, false)
	if err != nil {
		t.Fatalf("MergeRange without confirmation: %v", err)
	}
	if result != ConfirmationRequired {
		t.Fatalf("merge result = %q, want confirmation required", result)
	}
	if session.Workbook.Sheets[0].Cells[Coord{Row: 1, Col: 1}].Input != "discard me" || len(session.Workbook.Sheets[0].Merges) != 0 || session.Workbook.Dirty != beforeDirty {
		t.Fatal("confirmation-required merge changed workbook state")
	}
	result, err = MergeRange(session, "Sheet1", selection, true)
	if err != nil {
		t.Fatalf("MergeRange confirmed: %v", err)
	}
	if result != Merged {
		t.Fatalf("merge result = %q, want merged", result)
	}
	sheet := &session.Workbook.Sheets[0]
	if sheet.Cells[Coord{Row: 0, Col: 0}].Input != "top-left" || sheet.Cells[Coord{Row: 1, Col: 1}].Input != "" || len(sheet.Merges) != 1 {
		t.Fatalf("merged state = %#v", sheet)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo merge: %v", err)
	}
	sheet = &session.Workbook.Sheets[0]
	if sheet.Cells[Coord{Row: 1, Col: 1}].Input != "discard me" || len(sheet.Merges) != 0 {
		t.Fatal("undo did not restore merge data and metadata")
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo merge: %v", err)
	}
	if len(session.Workbook.Sheets[0].Merges) != 1 {
		t.Fatal("redo did not restore merge metadata")
	}
	if err := UnmergeRange(session, "Sheet1", selection); err != nil {
		t.Fatalf("UnmergeRange: %v", err)
	}
	if len(session.Workbook.Sheets[0].Merges) != 0 {
		t.Fatal("unmerge retained merge metadata")
	}

	if err := Undo(session); err != nil {
		t.Fatalf("Undo unmerge: %v", err)
	}
	if len(session.Workbook.Sheets[0].Merges) != 1 {
		t.Fatal("undo unmerge did not restore merged range")
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo unmerge: %v", err)
	}
	if len(session.Workbook.Sheets[0].Merges) != 0 {
		t.Fatal("redo unmerge did not remove merged range")
	}
}

func TestUndoStyleAndMerge(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{
		{Row: 0, Col: 0}: "top-left",
		{Row: 0, Col: 1}: "right",
	})
	session.history = historyState{}
	selection := CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 0, Col: 1}}
	style := CellStyle{FillRGB: "ABCDEF", Bold: true}
	if err := ApplyStyle(session, "Sheet1", selection, style); err != nil {
		t.Fatal(err)
	}
	if _, err := MergeRange(session, "Sheet1", selection, true); err != nil {
		t.Fatal(err)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo merge: %v", err)
	}
	if len(session.Workbook.Sheets[0].Merges) != 0 || session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 1}].Input != "right" {
		t.Fatal("undo merge did not restore cells")
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 0}].Style; got != style {
		t.Fatalf("merge undo changed direct style: %#v", got)
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo style: %v", err)
	}
	if got := session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 0}].Style; got != (CellStyle{}) {
		t.Fatalf("style after undo = %#v, want empty", got)
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo style: %v", err)
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo merge: %v", err)
	}
	if len(session.Workbook.Sheets[0].Merges) != 1 || session.Workbook.Sheets[0].Cells[Coord{Row: 0, Col: 0}].Style != style {
		t.Fatal("redo did not restore style and merge")
	}
}
