package app

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
	"omasheet/internal/workbook"
)

func BenchmarkOpenEditSave25kBy20(b *testing.B) {
	fixture := filepath.Join(b.TempDir(), "25k-by-20.xlsx")
	file := excelize.NewFile()
	row := make([]interface{}, 20)
	for col := range row {
		row[col] = fmt.Sprintf("column-%d", col+1)
	}
	for rowIndex := range 25_000 {
		for col := range row {
			row[col] = fmt.Sprintf("row-%d-cell-%d", rowIndex+1, col+1)
		}
		address := fmt.Sprintf("A%d", rowIndex+1)
		if err := file.SetSheetRow("Sheet1", address, &row); err != nil {
			b.Fatal(err)
		}
	}
	if err := file.SaveAs(fixture); err != nil {
		b.Fatal(err)
	}
	if err := file.Close(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		session, err := workbook.OpenFile(fixture)
		if err != nil {
			b.Fatal(err)
		}
		if err := workbook.SetCell(session, workbook.CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "updated"); err != nil {
			b.Fatal(err)
		}
		if err := workbook.SaveFile(session, fixture); err != nil {
			b.Fatal(err)
		}
		if err := session.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
