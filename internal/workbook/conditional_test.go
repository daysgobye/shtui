package workbook

import "testing"

func TestConditionalRuleMatchingBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		rule  ConditionalRule
		value string
		want  bool
	}{
		{name: "greater boundary", rule: ConditionalRule{Operator: GreaterThan, Value: "5"}, value: "5", want: false},
		{name: "greater match", rule: ConditionalRule{Operator: GreaterThan, Value: "5"}, value: "5.01", want: true},
		{name: "less boundary", rule: ConditionalRule{Operator: LessThan, Value: "5"}, value: "5", want: false},
		{name: "less match", rule: ConditionalRule{Operator: LessThan, Value: "5"}, value: "4.99", want: true},
		{name: "numeric equal", rule: ConditionalRule{Operator: Equal, Value: "5"}, value: "5.0", want: true},
		{name: "unequal", rule: ConditionalRule{Operator: Equal, Value: "5"}, value: "6", want: false},
		{name: "blank", rule: ConditionalRule{Operator: Blank}, value: "", want: true},
		{name: "whitespace not blank", rule: ConditionalRule{Operator: Blank}, value: " ", want: false},
		{name: "text contains case-insensitive", rule: ConditionalRule{Operator: TextContains, Value: "orch"}, value: "Orchard", want: true},
		{name: "text does not contain", rule: ConditionalRule{Operator: TextContains, Value: "orch"}, value: "apple", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := conditionalMatches(test.rule, test.value); got != test.want {
				t.Fatalf("conditionalMatches(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestConditionalRuleEffectiveStyleAndRoundTrip(t *testing.T) {
	session := newOperationSession(t)
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "5"); err != nil {
		t.Fatal(err)
	}
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 1, Col: 0}, "7"); err != nil {
		t.Fatal(err)
	}
	selection := CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 1, Col: 0}}
	if err := AddConditionalRule(session, "Sheet1", selection, ConditionalRule{Operator: GreaterThan, Value: "5", FillRGB: "FFCC00", FontRGB: "112233"}); err != nil {
		t.Fatalf("AddConditionalRule: %v", err)
	}
	sheet := &session.Workbook.Sheets[0]
	if got := EffectiveStyle(sheet, Coord{Row: 0, Col: 0}); got.FillRGB != "" || got.FontRGB != "" {
		t.Fatalf("boundary style = %#v, want no conditional style", got)
	}
	if got := EffectiveStyle(sheet, Coord{Row: 1, Col: 0}); got.FillRGB != "FFCC00" || got.FontRGB != "112233" {
		t.Fatalf("matching style = %#v", got)
	}
	path := t.TempDir() + "/conditional.xlsx"
	if err := SaveFile(session, path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer reopened.Close()
	rules := reopened.Workbook.Sheets[0].ConditionalRules
	if len(rules) != 1 || rules[0].Range != selection || rules[0].Operator != GreaterThan || rules[0].FillRGB != "FFCC00" || rules[0].FontRGB != "112233" {
		t.Fatalf("reopened conditional rules = %#v", rules)
	}
	if got := EffectiveStyle(&reopened.Workbook.Sheets[0], Coord{Row: 1, Col: 0}); got.FillRGB != "FFCC00" || got.FontRGB != "112233" {
		t.Fatalf("reopened conditional style = %#v", got)
	}
	formats, err := reopened.file.GetConditionalFormats("Sheet1")
	if err != nil {
		t.Fatalf("GetConditionalFormats: %v", err)
	}
	if len(formats) != 1 {
		t.Fatalf("conditional ranges = %d, want 1", len(formats))
	}
	for _, rules := range formats {
		if len(rules) != 1 || rules[0].Type != "cell" || rules[0].Criteria != "greater than" || rules[0].Value != "5" || rules[0].Format == nil {
			t.Fatalf("saved conditional rules = %#v", rules)
		}
	}
}

func TestConditionalRulesAllOperatorsPersist(t *testing.T) {
	session := newOperationSession(t)
	operators := []ConditionalRule{
		{Operator: GreaterThan, Value: "2", FillRGB: "AA0000"},
		{Operator: LessThan, Value: "3", FillRGB: "00AA00"},
		{Operator: Equal, Value: "4", FontRGB: "0000AA"},
		{Operator: Blank, FillRGB: "AAAA00"},
		{Operator: TextContains, Value: "x", FontRGB: "AA00AA"},
	}
	for index, rule := range operators {
		selection := CellRange{Start: Coord{Row: index, Col: 0}, End: Coord{Row: index, Col: 0}}
		if err := AddConditionalRule(session, "Sheet1", selection, rule); err != nil {
			t.Fatalf("AddConditionalRule %q: %v", rule.Operator, err)
		}
	}
	if len(session.Workbook.Sheets[0].ConditionalRules) != len(operators) {
		t.Fatalf("conditional rule count = %d, want %d", len(session.Workbook.Sheets[0].ConditionalRules), len(operators))
	}
}

func TestUndoConditionalRule(t *testing.T) {
	session := newOperationSession(t)
	if err := SetCell(session, CellAddress{Sheet: "Sheet1", Row: 0, Col: 0}, "8"); err != nil {
		t.Fatal(err)
	}
	session.history = historyState{}
	selection := CellRange{Start: Coord{Row: 0, Col: 0}, End: Coord{Row: 0, Col: 0}}
	rule := ConditionalRule{Operator: GreaterThan, Value: "5", FillRGB: "FFCC00"}
	if err := AddConditionalRule(session, "Sheet1", selection, rule); err != nil {
		t.Fatal(err)
	}
	if len(session.Workbook.Sheets[0].ConditionalRules) != 1 {
		t.Fatal("conditional rule was not added")
	}
	if err := Undo(session); err != nil {
		t.Fatalf("Undo conditional rule: %v", err)
	}
	if len(session.Workbook.Sheets[0].ConditionalRules) != 0 {
		t.Fatal("undo retained conditional rule")
	}
	if err := Redo(session); err != nil {
		t.Fatalf("Redo conditional rule: %v", err)
	}
	if len(session.Workbook.Sheets[0].ConditionalRules) != 1 || EffectiveStyle(&session.Workbook.Sheets[0], Coord{Row: 0, Col: 0}).FillRGB != "FFCC00" {
		t.Fatal("redo did not restore conditional rule and effective style")
	}
}

func TestTwoColorScaleAppliesAndReloads(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{{Row: 0}: "1", {Row: 1}: "3"})
	selection := CellRange{Start: Coord{Row: 0}, End: Coord{Row: 1}}
	scale := ColorScale{MinRGB: "FF0000", MaxRGB: "0000FF"}
	if err := AddColorScaleRule(session, "Sheet1", selection, scale); err != nil {
		t.Fatalf("AddColorScaleRule: %v", err)
	}
	formats, err := session.file.GetConditionalFormats("Sheet1")
	if err != nil || len(formats) != 1 {
		t.Fatalf("GetConditionalFormats = %#v, %v", formats, err)
	}
	for _, options := range formats {
		if len(options) != 1 || options[0].Type != "2_color_scale" || options[0].MinType != "min" || options[0].MaxType != "max" || options[0].MinColor != "#FF0000" || options[0].MaxColor != "#0000FF" {
			t.Fatalf("two-color options = %#v", options)
		}
	}
	path := t.TempDir() + "/two-color-scale.xlsx"
	if err := SaveFile(session, path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if err := LoadColorScales(reopened); err != nil {
		t.Fatalf("LoadColorScales: %v", err)
	}
	rules := reopened.Workbook.Sheets[0].ConditionalRules
	if len(rules) != 1 || rules[0].Range != selection || rules[0].ColorScale == nil || *rules[0].ColorScale != scale {
		t.Fatalf("reloaded color scales = %#v", rules)
	}
}

func TestThreeColorScaleInterpolatesNumericValues(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{{Row: 0}: "0", {Row: 1}: "50", {Row: 2}: "100"})
	selection := CellRange{Start: Coord{Row: 0}, End: Coord{Row: 2}}
	scale := ColorScale{MinRGB: "FF0000", MidRGB: "FFFF00", MaxRGB: "00FF00"}
	if err := AddColorScaleRule(session, "Sheet1", selection, scale); err != nil {
		t.Fatalf("AddColorScaleRule: %v", err)
	}
	for _, test := range []struct {
		row  int
		want string
	}{{0, "FF0000"}, {1, "FFFF00"}, {2, "00FF00"}} {
		if got := EffectiveStyle(&session.Workbook.Sheets[0], Coord{Row: test.row}).FillRGB; got != test.want {
			t.Errorf("row %d color = %q, want %q", test.row, got, test.want)
		}
	}
}

func TestColorScaleHandlesSingleCellAndNonnumericValues(t *testing.T) {
	session := newOperationSession(t)
	setCellInputs(t, session, map[Coord]string{{Row: 0}: "7", {Row: 1}: "text"})
	if err := ApplyStyle(session, "Sheet1", CellRange{Start: Coord{Row: 1}, End: Coord{Row: 1}}, CellStyle{FillRGB: "112233"}); err != nil {
		t.Fatal(err)
	}
	if err := AddColorScaleRule(session, "Sheet1", CellRange{Start: Coord{}, End: Coord{}}, ColorScale{MinRGB: "FF0000", MaxRGB: "0000FF"}); err != nil {
		t.Fatalf("single-cell AddColorScaleRule: %v", err)
	}
	if got := EffectiveStyle(&session.Workbook.Sheets[0], Coord{}).FillRGB; got != "800080" {
		t.Fatalf("single-cell midpoint color = %q, want 800080", got)
	}
	if err := AddColorScaleRule(session, "Sheet1", CellRange{Start: Coord{Row: 1}, End: Coord{Row: 2}}, ColorScale{MinRGB: "FF0000", MaxRGB: "0000FF"}); err != nil {
		t.Fatalf("nonnumeric AddColorScaleRule: %v", err)
	}
	for _, coord := range []Coord{{Row: 1}, {Row: 2}} {
		if got := EffectiveStyle(&session.Workbook.Sheets[0], coord).FillRGB; got != "112233" && coord.Row == 1 || got != "" && coord.Row == 2 {
			t.Errorf("nonnumeric cell (%d,%d) fill = %q", coord.Row, coord.Col, got)
		}
	}
}
