package workbook

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveRefusesUnsupportedPartLoss(t *testing.T) {
	source := copyFixture(t, "chart.xlsx")
	candidate := filepath.Join(t.TempDir(), "candidate.xlsx")
	copyZipWithoutPart(t, source, candidate, "xl/charts/chart1.xml")
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := AuditPackage(source, candidate, map[string]struct{}{"xl/worksheets/sheet1.xml": {}}); err == nil {
		t.Fatal("AuditPackage accepted a candidate that dropped a chart part")
	}
	after, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("package audit changed the source workbook")
	}
}

func TestSaveRefusesUnknownElementLoss(t *testing.T) {
	path := copyFixture(t, "validation.xlsx")
	injectUnknownWorksheetExtension(t, path)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	session, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if err := session.file.SetCellValue("Sheet1", "A1", "edited"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	candidate := filepath.Join(t.TempDir(), "candidate.xlsx")
	if err := session.file.SaveAs(candidate); err != nil {
		t.Fatalf("SaveAs candidate: %v", err)
	}
	stripWorksheetExtension(t, candidate)
	err = AuditPackage(path, candidate, safeWorkbookChanges(session.OriginalParts))
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unsupported xml") {
		t.Fatalf("AuditPackage error = %v, want unsupported XML refusal", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("package audit changed the source workbook")
	}
}

func stripWorksheetExtension(t *testing.T, path string) {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	tempPath := path + ".stripped"
	output, err := os.Create(tempPath)
	if err != nil {
		reader.Close()
		t.Fatal(err)
	}
	writer := zip.NewWriter(output)
	for _, part := range reader.File {
		rc, err := part.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if part.Name == "xl/worksheets/sheet1.xml" {
			start := bytes.Index(data, []byte("<extLst"))
			if start < 0 {
				t.Fatal("candidate worksheet has no extension list to remove")
			}
			end := bytes.Index(data[start:], []byte("</extLst>"))
			if end < 0 {
				t.Fatal("candidate worksheet extension list is incomplete")
			}
			end += start + len("</extLst>")
			data = append(data[:start], data[end:]...)
		}
		partWriter, err := writer.Create(part.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := partWriter.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		t.Fatal(err)
	}
}

func TestXLSMVBAProjectRetainedOrRefused(t *testing.T) {
	path := copyFixture(t, "macro.xlsm")
	originalVBA := packagePart(t, path, "xl/vbaProject.bin")
	if len(originalVBA) == 0 {
		t.Fatal("macro fixture has no VBA project")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	session, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if err := session.file.SetCellValue("Sheet1", "A1", "edited"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	if err := SaveFile(session, path); err != nil {
		after, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("refused macro save changed source bytes")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "preserv") && !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
			t.Fatalf("macro save refusal is not actionable: %v", err)
		}
		return
	}
	if got := packagePart(t, path, "xl/vbaProject.bin"); !bytes.Equal(got, originalVBA) {
		t.Fatal("VBA project bytes changed after save")
	}
}

func copyZipWithoutPart(t *testing.T, source, destination, omitted string) {
	t.Helper()
	reader, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	file, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, part := range reader.File {
		if part.Name == omitted {
			continue
		}
		rc, err := part.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		header := part.FileHeader
		wc, err := writer.CreateHeader(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := wc.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func injectUnknownWorksheetExtension(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := path + ".source"
	if err := os.Rename(path, oldPath); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, part := range reader.File {
		rc, openErr := part.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		content, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if part.Name == "xl/worksheets/sheet1.xml" {
			content = bytes.Replace(content, []byte("</worksheet>"), []byte(`<extLst><ext uri="{9A3F}" xmlns:x="urn:unsupported"><x:futureFeature value="retain-me"/></ext></extLst></worksheet>`), 1)
		}
		header := part.FileHeader
		wc, createErr := writer.CreateHeader(&header)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, err := wc.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("source fixture is empty")
	}
}
