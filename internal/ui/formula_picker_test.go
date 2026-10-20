package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFormulaPickerAdditionalFunctions(t *testing.T) {
	tests := []struct {
		name          string
		arguments     string
		argumentValue string
		wantPrompt    string
		wantFormula   string
	}{
		{
			name:          "COUNTIF",
			arguments:     "range, criteria",
			argumentValue: `A1:A3,">0"`,
			wantPrompt:    "Arguments: ",
			wantFormula:   `=COUNTIF(A1:A3,">0")`,
		},
		{
			name:          "IFERROR",
			arguments:     "value, value_if_error",
			argumentValue: `A1/0,"safe"`,
			wantPrompt:    "Arguments: ",
			wantFormula:   `=IFERROR(A1/0,"safe")`,
		},
		{
			name:          "COUNTA",
			arguments:     "range",
			argumentValue: "A1:A3",
			wantPrompt:    "Range: ",
			wantFormula:   "=COUNTA(A1:A3)",
		},
		{
			name:          "COUNT",
			arguments:     "range",
			argumentValue: "A1:A3",
			wantPrompt:    "Range: ",
			wantFormula:   "=COUNT(A1:A3)",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			picker := newFormulaPicker("A1:A3", "C1")
			index := -1
			for i, item := range picker.Functions.Items() {
				function, ok := item.(functionItem)
				if ok && function.definition.name == test.name {
					index = i
					break
				}
			}
			if index < 0 {
				t.Fatalf("%s is missing from the supported function list", test.name)
			}
			for picker.Functions.Index() < index {
				picker.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
			}
			if got := picker.selectedFunctionName(); got != test.name {
				t.Fatalf("selected function = %q, want %q", got, test.name)
			}
			if picker.RangeInput.Prompt != test.wantPrompt {
				t.Errorf("argument prompt = %q, want %q", picker.RangeInput.Prompt, test.wantPrompt)
			}
			picker.RangeInput.SetValue(test.argumentValue)
			if want := test.name + "(" + test.arguments + ")"; picker.Syntax() != want {
				t.Errorf("syntax = %q, want %q", picker.Syntax(), want)
			}
			if got, err := picker.Formula(); err != nil || got != test.wantFormula {
				t.Errorf("formula = %q, error %v; want %q", got, err, test.wantFormula)
			}
		})
	}

	for _, function := range supportedFunctions {
		if function.name == "SUMIF" {
			t.Fatal("SUMIF must remain unavailable while Excelize returns incorrect numeric-criteria results")
		}
	}
}
