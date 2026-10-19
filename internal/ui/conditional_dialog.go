package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"omasheet/internal/workbook"
)

const (
	conditionalSingleColor = iota
	conditionalColorScale
)

type conditionalDialogState struct {
	Open       bool
	RangeInput textinput.Model
	ValueInput textinput.Model
	Mode       int
	Focus      int
	Operator   int
	FillRGB    string
	FontRGB    string
	Scale      workbook.ColorScale
	ThreeColor bool
	Rules      []workbook.ConditionalRule
	ColorField string
	Error      string
}

var conditionalOperators = []struct {
	operator workbook.RuleOperator
	label    string
}{
	{operator: workbook.GreaterThan, label: "Greater than"},
	{operator: workbook.LessThan, label: "Less than"},
	{operator: workbook.Equal, label: "Equal to"},
	{operator: workbook.Blank, label: "Is blank"},
	{operator: workbook.TextContains, label: "Text contains"},
}

func newConditionalDialog(rangeName string) conditionalDialogState {
	rangeInput := textinput.New()
	rangeInput.Prompt = "Apply to range: "
	rangeInput.SetValue(rangeName)
	rangeInput.Focus()
	rangeInput.CursorEnd()
	rangeInput.Width = 40
	rangeInput.PromptStyle = lipgloss.NewStyle().Foreground(ANSIColor(8))
	rangeInput.TextStyle = lipgloss.NewStyle().Foreground(ANSIColor(7))
	rangeInput.Cursor.Style = lipgloss.NewStyle().Foreground(ANSIColor(6))

	valueInput := textinput.New()
	valueInput.Prompt = "Value: "
	valueInput.Width = 26
	valueInput.PromptStyle = rangeInput.PromptStyle
	valueInput.TextStyle = rangeInput.TextStyle
	valueInput.Cursor.Style = rangeInput.Cursor.Style
	return conditionalDialogState{
		Open:       true,
		RangeInput: rangeInput,
		ValueInput: valueInput,
		Scale:      workbook.ColorScale{MinRGB: "CD0000", MidRGB: "FFFF00", MaxRGB: "00CD00"},
		ThreeColor: true,
	}
}

func (dialog *conditionalDialogState) HandleKey(message tea.KeyMsg) string {
	if !dialog.Open {
		return ""
	}
	switch message.Type {
	case tea.KeyEsc:
		dialog.Open = false
		return "cancel"
	case tea.KeyTab:
		dialog.Focus = (dialog.Focus + 1) % dialog.focusCount()
		dialog.Error = ""
		dialog.syncInputFocus()
		return ""
	case tea.KeyEnter:
		action := dialog.enter()
		dialog.syncInputFocus()
		return action
	case tea.KeyLeft:
		if dialog.Focus == 1 {
			dialog.Mode = conditionalSingleColor
			dialog.Error = ""
		}
	case tea.KeyRight:
		if dialog.Focus == 1 {
			dialog.Mode = conditionalColorScale
			dialog.Error = ""
		}
	case tea.KeyUp, tea.KeyDown:
		if dialog.Mode == conditionalSingleColor && dialog.Focus == 2 {
			delta := -1
			if message.Type == tea.KeyDown {
				delta = 1
			}
			dialog.Operator = (dialog.Operator + delta + len(conditionalOperators)) % len(conditionalOperators)
			dialog.Error = ""
			return ""
		}
	case tea.KeyRunes:
		if dialog.Focus == 1 {
			switch string(message.Runes) {
			case "2":
				dialog.ThreeColor = false
				dialog.Error = ""
			case "3":
				dialog.ThreeColor = true
				dialog.Error = ""
			}
			return ""
		}
	}
	if message.Type == tea.KeySpace {
		message.Type = tea.KeyRunes
		message.Runes = []rune{' '}
	}
	switch dialog.Focus {
	case 0:
		dialog.RangeInput, _ = dialog.RangeInput.Update(message)
	case 3:
		if dialog.Mode == conditionalSingleColor {
			dialog.ValueInput, _ = dialog.ValueInput.Update(message)
		}
	}
	dialog.Error = ""
	return ""
}

func (dialog *conditionalDialogState) enter() string {
	switch dialog.Focus {
	case 0, 1:
		dialog.Focus++
	case 2:
		if dialog.Mode == conditionalSingleColor {
			dialog.Focus++
			break
		}
		dialog.ColorField = "min"
		return "color"
	case 3:
		if dialog.Mode == conditionalSingleColor {
			dialog.Focus++
			break
		}
		dialog.ColorField = "mid"
		return "color"
	case 4:
		if dialog.Mode == conditionalSingleColor {
			dialog.ColorField = "fill"
		} else {
			dialog.ColorField = "max"
		}
		return "color"
	case 5:
		if dialog.Mode == conditionalSingleColor {
			dialog.ColorField = "font"
			return "color"
		}
		return dialog.addRule()
	case 6:
		if dialog.Mode == conditionalSingleColor {
			return dialog.addRule()
		}
		return "done"
	case 7:
		if dialog.Mode == conditionalSingleColor {
			return "done"
		}
		return "cancel"
	case 8:
		return "cancel"
	}
	return ""
}

func (dialog *conditionalDialogState) focusCount() int {
	if dialog.Mode == conditionalSingleColor {
		return 9
	}
	return 8
}
func (dialog *conditionalDialogState) syncInputFocus() {
	if dialog.Focus == 0 {
		dialog.RangeInput.Focus()
	} else {
		dialog.RangeInput.Blur()
	}
	if dialog.Mode == conditionalSingleColor && dialog.Focus == 3 {
		dialog.ValueInput.Focus()
	} else {
		dialog.ValueInput.Blur()
	}
}

func (dialog *conditionalDialogState) addRule() string {
	rules, err := dialog.currentRules()
	if err != nil {
		dialog.Error = err.Error()
		return ""
	}
	if len(rules) != 1 {
		dialog.Error = "Complete this rule before adding another"
		return ""
	}
	dialog.Rules = append(dialog.Rules, rules[0])
	dialog.ValueInput.Reset()
	dialog.FillRGB = ""
	dialog.FontRGB = ""
	dialog.Operator = 0
	dialog.Focus = 2
	dialog.Error = ""
	return ""
}

func (dialog *conditionalDialogState) BuildRules() (workbook.CellRange, []workbook.ConditionalRule, error) {
	startCol, startRow, endCol, endRow, err := parseA1Range(dialog.RangeInput.Value())
	if err != nil {
		return workbook.CellRange{}, nil, fmt.Errorf("invalid range: %w", err)
	}
	selection := workbook.CellRange{Start: workbook.Coord{Row: startRow, Col: startCol}, End: workbook.Coord{Row: endRow, Col: endCol}}
	rules := append([]workbook.ConditionalRule(nil), dialog.Rules...)
	current, err := dialog.currentRules()
	if err != nil {
		return workbook.CellRange{}, nil, err
	}
	rules = append(rules, current...)
	if len(rules) == 0 {
		return workbook.CellRange{}, nil, fmt.Errorf("add at least one formatting rule")
	}
	for index := range rules {
		rules[index].Range = selection
	}
	return selection, rules, nil
}

func (dialog *conditionalDialogState) currentRules() ([]workbook.ConditionalRule, error) {
	if dialog.Mode == conditionalColorScale {
		scale := dialog.Scale
		if !dialog.ThreeColor {
			scale.MidRGB = ""
		}
		if scale.MinRGB == "" || scale.MaxRGB == "" || (dialog.ThreeColor && scale.MidRGB == "") {
			return nil, fmt.Errorf("choose minimum, midpoint, and maximum colors for the scale")
		}
		return []workbook.ConditionalRule{{ColorScale: &scale}}, nil
	}
	value := strings.TrimSpace(dialog.ValueInput.Value())
	hasDraft := value != "" || dialog.FillRGB != "" || dialog.FontRGB != ""
	if !hasDraft {
		return nil, nil
	}
	if dialog.Operator < 0 || dialog.Operator >= len(conditionalOperators) {
		return nil, fmt.Errorf("choose a supported condition")
	}
	operator := conditionalOperators[dialog.Operator].operator
	switch operator {
	case workbook.GreaterThan, workbook.LessThan:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return nil, fmt.Errorf("%s needs a number", conditionalOperators[dialog.Operator].label)
		}
	case workbook.Equal, workbook.TextContains:
		if value == "" {
			return nil, fmt.Errorf("%s needs a value", conditionalOperators[dialog.Operator].label)
		}
	case workbook.Blank:
		value = ""
	}
	if dialog.FillRGB == "" && dialog.FontRGB == "" {
		return nil, fmt.Errorf("choose a fill or font color")
	}
	rule := workbook.ConditionalRule{Operator: operator, Value: value, FillRGB: dialog.FillRGB, FontRGB: dialog.FontRGB}
	return []workbook.ConditionalRule{rule}, nil
}

func (dialog *conditionalDialogState) SetColor(rgb string) {
	switch dialog.ColorField {
	case "fill":
		dialog.FillRGB = rgb
	case "font":
		dialog.FontRGB = rgb
	case "min":
		dialog.Scale.MinRGB = rgb
	case "mid":
		dialog.Scale.MidRGB = rgb
	case "max":
		dialog.Scale.MaxRGB = rgb
	}
	dialog.Error = ""
}

func (dialog *conditionalDialogState) View(width, height int) string {
	frameWidth := min(76, max(width-4, 1))
	innerWidth := max(frameWidth-4, 1)
	compact := width <= 42 || height <= 12
	if height <= 10 {
		return dialog.tinyView(innerWidth)
	}
	dialog.RangeInput.Prompt = "Apply to range: "
	dialog.RangeInput.Width = max(innerWidth-18, 1)
	dialog.ValueInput.Width = max(innerWidth-12, 1)
	accent := lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6))
	dim := lipgloss.NewStyle().Foreground(ANSIColor(8))
	selected := func(value string, focused bool) string {
		style := lipgloss.NewStyle()
		if focused {
			style = style.Reverse(true).Bold(true)
		}
		return style.Render(value)
	}
	tabLine := selected("Single color", dialog.Focus == 1 && dialog.Mode == conditionalSingleColor) + "   " + selected("Color scale", dialog.Focus == 1 && dialog.Mode == conditionalColorScale)
	lines := []string{accent.Render("Conditional formatting"), tabLine, dialog.RangeInput.View()}
	if dialog.Mode == conditionalSingleColor {
		operator := conditionalOperators[dialog.Operator].label
		if compact {
			lines = append(lines, "Condition: "+selected(operator, dialog.Focus == 2), dialog.ValueInput.View(), selected("Fill:"+compactColor(dialog.FillRGB), dialog.Focus == 4)+" "+selected("Font:"+compactColor(dialog.FontRGB), dialog.Focus == 5), "Preview: "+dialog.Preview(), selected("+ Add another rule", dialog.Focus == 6)+"   "+selected("Done", dialog.Focus == 7)+"   "+selected("Cancel", dialog.Focus == 8))
		} else {
			lines = append(lines, "Format rules", "Condition: "+selected(operator, dialog.Focus == 2)+"  ↑↓ change", dialog.ValueInput.View(), "Fill color: "+selected(colorName(dialog.FillRGB)+"  Enter choose", dialog.Focus == 4), "Font color: "+selected(colorName(dialog.FontRGB)+"  Enter choose", dialog.Focus == 5), "Preview: "+dialog.Preview(), selected("+ Add another rule", dialog.Focus == 6), dim.Render("Tab: next   Enter: select   Esc: cancel"), selected("Cancel", dialog.Focus == 8)+"    "+selected("Done", dialog.Focus == 7))
		}
	} else {
		variant := "3-color scale"
		if !dialog.ThreeColor {
			variant = "2-color scale"
		}
		if compact {
			lines = append(lines, selected(variant+" · press 2 or 3", dialog.Focus == 1), "Min: "+colorName(dialog.Scale.MinRGB), "Mid: "+colorName(dialog.Scale.MidRGB)+"   Max: "+colorName(dialog.Scale.MaxRGB), "Preview: "+dialog.Preview(), selected("+ Add rule", dialog.Focus == 5)+"   "+selected("Done", dialog.Focus == 6)+"   "+selected("Cancel", dialog.Focus == 7))
		} else {
			lines = append(lines, selected(variant+" · press 2 or 3", dialog.Focus == 1), selected("Minimum: "+colorName(dialog.Scale.MinRGB)+"  Enter choose", dialog.Focus == 2), selected("Midpoint: "+colorName(dialog.Scale.MidRGB)+"  Enter choose", dialog.Focus == 3), selected("Maximum: "+colorName(dialog.Scale.MaxRGB)+"  Enter choose", dialog.Focus == 4), "Preview: "+dialog.Preview(), selected("+ Add another rule", dialog.Focus == 5), dim.Render("Tab: next   Enter: select   Esc: cancel"), selected("Cancel", dialog.Focus == 7)+"    "+selected("Done", dialog.Focus == 6))
		}
	}
	if dialog.Error != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(ANSIColor(1)).Render(dialog.Error))
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ANSIColor(6)).Padding(0, 1).Width(innerWidth).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
func (dialog *conditionalDialogState) tinyView(width int) string {
	dialog.RangeInput.Prompt = "Range: "
	dialog.RangeInput.Width = max(width-8, 1)
	dialog.ValueInput.Width = max(width-8, 1)
	selected := func(value string, focused bool) string {
		style := lipgloss.NewStyle()
		if focused {
			style = style.Reverse(true).Bold(true)
		}
		return style.Render(value)
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6)).Render("Cond: ") +
		selected("Single", dialog.Focus == 1 && dialog.Mode == conditionalSingleColor) + " / " +
		selected("Scale", dialog.Focus == 1 && dialog.Mode == conditionalColorScale)
	lines := []string{header, dialog.RangeInput.View()}
	if dialog.Mode == conditionalColorScale {
		variant := "3-color"
		if !dialog.ThreeColor {
			variant = "2-color"
		}
		lines = append(lines,
			selected(variant, dialog.Focus == 1),
			selected("Min:"+compactColor(dialog.Scale.MinRGB), dialog.Focus == 2),
		)
		if dialog.ThreeColor {
			lines = append(lines, selected("Mid:"+compactColor(dialog.Scale.MidRGB), dialog.Focus == 3))
		}
		lines = append(lines, selected("Max:"+compactColor(dialog.Scale.MaxRGB), dialog.Focus == 4))
	} else {
		operator := ">"
		switch conditionalOperators[dialog.Operator].operator {
		case workbook.LessThan:
			operator = "<"
		case workbook.Equal:
			operator = "="
		case workbook.Blank:
			operator = "blank"
		case workbook.TextContains:
			operator = "contains"
		}
		lines = append(lines,
			selected("If "+operator, dialog.Focus == 2),
			dialog.ValueInput.View(),
			selected("F:"+compactColor(dialog.FillRGB), dialog.Focus == 4)+" "+selected("T:"+compactColor(dialog.FontRGB), dialog.Focus == 5),
			truncate("Preview: "+dialog.Preview(), max(width-2, 1)),
		)
	}
	if dialog.Error != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(ANSIColor(1)).Render(truncate(dialog.Error, max(width-2, 1))))
	}
	addFocus, doneFocus, cancelFocus := 6, 7, 8
	if dialog.Mode == conditionalColorScale {
		addFocus, doneFocus, cancelFocus = 5, 6, 7
	}
	lines = append(lines, selected("+Rule", dialog.Focus == addFocus)+" "+selected("Done", dialog.Focus == doneFocus)+" "+selected("Cancel", dialog.Focus == cancelFocus))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ANSIColor(6)).Padding(0, 1).Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (dialog *conditionalDialogState) Preview() string {
	if dialog.Mode == conditionalColorScale {
		if dialog.ThreeColor {
			return colorName(dialog.Scale.MinRGB) + " → " + colorName(dialog.Scale.MidRGB) + " → " + colorName(dialog.Scale.MaxRGB)
		}
		return colorName(dialog.Scale.MinRGB) + " → " + colorName(dialog.Scale.MaxRGB)
	}
	if dialog.Operator < 0 || dialog.Operator >= len(conditionalOperators) {
		return "Choose a condition"
	}
	value := strings.TrimSpace(dialog.ValueInput.Value())
	if value == "" {
		value = "…"
	}
	return conditionalOperators[dialog.Operator].label + " " + value + " · " + colorName(dialog.FillRGB)
}

func colorName(rgb string) string {
	if rgb == "" {
		return "choose color"
	}
	return "#" + strings.TrimPrefix(strings.ToUpper(rgb), "#")
}
func compactColor(rgb string) string {
	if rgb == "" {
		return "—"
	}
	return "#" + strings.TrimPrefix(strings.ToUpper(rgb), "#")
}
