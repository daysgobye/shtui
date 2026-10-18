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

func TestXLSXCellEditRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		check   func(*testing.T, *Session, string)
	}{
		{
			name:    "chart",
			fixture: "chart.xlsx",
			check: func(t *testing.T, session *Session, path string) {
				t.Helper()
				if !packageHasPrefix(t, path, "xl/charts/chart") {
					t.Fatal("chart package part disappeared after save")
				}
			},
		},
		{
			name:    "pivot table",
			fixture: "pivottable.xlsx",
			check: func(t *testing.T, session *Session, path string) {
				t.Helper()
				if !packageHasPrefix(t, path, "xl/pivotTables/pivotTable") || !packageHasPrefix(t, path, "xl/pivotCache/pivotCacheDefinition") {
					t.Fatal("PivotTable or cache package part disappeared after save")
				}
			},
		},
		{
			name:    "validation",
			fixture: "validation.xlsx",
			check: func(t *testing.T, session *Session, path string) {
				t.Helper()
				validations, err := session.file.GetDataValidations("Sheet1")
				if err != nil {
					t.Fatalf("GetDataValidations: %v", err)
				}
				if len(validations) != 1 {
					t.Fatalf("validation count = %d, want 1", len(validations))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := copyFixture(t, test.fixture)
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
					t.Fatal("refused save changed source bytes")
				}
				if !strings.Contains(strings.ToLower(err.Error()), "preserv") && !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
					t.Fatalf("save refusal is not actionable: %v", err)
				}
				return
			}
			reopened, err := OpenFile(path)
			if err != nil {
				t.Fatalf("reopen saved workbook: %v", err)
			}
			got, err := reopened.file.GetCellValue("Sheet1", "A1")
			if err != nil {
				t.Fatalf("GetCellValue: %v", err)
			}
			if got != "edited" {
				t.Fatalf("saved A1 = %q, want edited", got)
			}
			test.check(t, reopened, path)
		})
	}
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	source := filepath.Join("testdata", "preservation", name)
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read fixture %s: %v", source, err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("copy fixture %s: %v", source, err)
	}
	return path
}

func packageHasPrefix(t *testing.T, path, prefix string) bool {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open package %s: %v", path, err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, prefix) {
			return true
		}
	}
	return false
}

func packagePart(t *testing.T, path, name string) []byte {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open package %s: %v", path, err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open package part %s: %v", name, err)
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatalf("read package part %s: %v", name, readErr)
		}
		if closeErr != nil {
			t.Fatalf("close package part %s: %v", name, closeErr)
		}
		return data
	}
	return nil
}
