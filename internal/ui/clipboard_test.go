package ui

import (
	"reflect"
	"testing"
)

func TestClipboardTSVRoundTripPreservesEmptyEdges(t *testing.T) {
	want := [][]string{{"", "formula", ""}, {"left", "", "right"}}
	if got := parseTSV(matrixTSV(want)); !reflect.DeepEqual(got, want) {
		t.Fatalf("TSV round trip = %#v, want %#v", got, want)
	}
}

func TestParseTSVDoesNotAddRowForTrailingLineTerminator(t *testing.T) {
	for _, text := range []string{"first\t\t\r\nsecond\t\r\n", "first\t\t\nsecond\t\n"} {
		want := [][]string{{"first", "", ""}, {"second", ""}}
		if got := parseTSV(text); !reflect.DeepEqual(got, want) {
			t.Errorf("parseTSV(%q) = %#v, want %#v", text, got, want)
		}
	}
}
