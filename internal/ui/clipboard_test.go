package ui

import (
	"reflect"
	"testing"
)

func TestClipboardTSVRoundTripPreservesEmptyEdges(t *testing.T) {
	want := [][]string{{"", "formula", ""}, {"left", "", "right"}}
	got, err := parseTSV(matrixTSV(want))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TSV round trip = %#v, want %#v", got, want)
	}
}

func TestClipboardTSVRoundTripPreservesEmbeddedSeparators(t *testing.T) {
	want := [][]string{{`=IF(A1="x	inside
the cell",A1,0)`, "tab\tand\nnewline\t"}}
	got, err := parseTSV(matrixTSV(want))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TSV round trip = %#v, want %#v", got, want)
	}
}

func TestParseTSVDoesNotAddRowForTrailingLineTerminator(t *testing.T) {
	for _, text := range []string{"first\t\t\r\nsecond\t\r\n", "first\t\t\nsecond\t\n"} {
		want := [][]string{{"first", "", ""}, {"second", ""}}
		got, err := parseTSV(text)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parseTSV(%q) = %#v, want %#v", text, got, want)
		}
	}
}

func TestParseTSVEmptyTextRepresentsBlankCell(t *testing.T) {
	got, err := parseTSV("")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{""}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTSV empty text = %#v, want %#v", got, want)
	}
}
