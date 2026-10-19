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
	Open              bool
	RangeInput        textinput.Model
	ValueInput        textinput.Model
	Mode              int
	Focus             int
	Operator          int
	FillRGB           string
	FontRGB           string
	Scale             workbook.ColorScale
	ThreeColor        bool
	ScaleDraftActive  bool
	Rules             []workbook.ConditionalRule
	ColorField        string
	ConditionMenuOpen bool
	Error             string
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
		Open:             true,
		RangeInput:       rangeInput,
		ValueInput:       valueInput,
		Scale:            workbook.ColorScale{MinRGB: "CD0000", MidRGB: "FFFF00", MaxRGB: "00CD00"},
		ThreeColor:       true,
		ScaleDraftActive: true,
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
	case tea.KeyShiftTab:
		dialog.Focus = (dialog.Focus - 1 + dialog.focusCount()) % dialog.focusCount()
		dialog.ConditionMenuOpen = false
		dialog.Error = ""
		dialog.syncInputFocus()
		return ""
	case tea.KeyTab:
		dialog.Focus = (dialog.Focus + 1) % dialog.focusCount()
		dialog.ConditionMenuOpen = false
		dialog.Error = ""
		dialog.syncInputFocus()
		return ""
	case tea.KeyEnter:
		if dialog.ConditionMenuOpen {
			dialog.ConditionMenuOpen = false
			return ""
		}
		action := dialog.enter()
		dialog.syncInputFocus()
		return action
	case tea.KeyLeft:
		if dialog.Mode == conditionalColorScale && dialog.Focus == 2 {
			dialog.setThreeColor(false)
		} else if dialog.Focus == 1 {
			dialog.Mode = conditionalSingleColor
			dialog.ConditionMenuOpen = false
			dialog.Error = ""
		}
	case tea.KeyRight:
		if dialog.Mode == conditionalColorScale && dialog.Focus == 2 {
			dialog.setThreeColor(true)
		} else if dialog.Focus == 1 {
			dialog.Mode = conditionalColorScale
			dialog.ConditionMenuOpen = false
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
		if dialog.Mode == conditionalColorScale && dialog.Focus == 2 {
			dialog.setThreeColor(message.Type == tea.KeyDown)
			return ""
		}
	case tea.KeyRunes:
		if dialog.Mode == conditionalColorScale && dialog.Focus == 2 {
			switch string(message.Runes) {
			case "2":
				dialog.setThreeColor(false)
			case "3":
				dialog.setThreeColor(true)
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
	case dialog.addFocus():
		return dialog.addRule()
	case dialog.removeFocus():
		dialog.removeLastRule()
		return ""
	case dialog.doneFocus():
		return "done"
	case dialog.cancelFocus():
		return "cancel"
	}
	switch dialog.Focus {
	case 0, 1, 2:
		dialog.Focus++
	case 3:
		if dialog.Mode == conditionalSingleColor {
			dialog.Focus++
		} else {
			dialog.ColorField = "min"
			return "color"
		}
	case 4:
		if dialog.Mode == conditionalSingleColor {
			dialog.ColorField = "fill"
		} else if dialog.ThreeColor {
			dialog.ColorField = "mid"
		} else {
			dialog.ColorField = "max"
		}
		return "color"
	case 5:
		if dialog.Mode == conditionalSingleColor {
			dialog.ColorField = "font"
		} else {
			dialog.ColorField = "max"
		}
		return "color"
	}
	return ""
}

func (dialog *conditionalDialogState) focusCount() int {
	return dialog.cancelFocus() + 1
}

func (dialog *conditionalDialogState) addFocus() int {
	if dialog.Mode == conditionalColorScale && !dialog.ThreeColor {
		return 5
	}
	return 6
}

func (dialog *conditionalDialogState) removeFocus() int {
	if len(dialog.Rules) == 0 {
		return -1
	}
	return dialog.addFocus() + 1
}

func (dialog *conditionalDialogState) doneFocus() int {
	if len(dialog.Rules) > 0 {
		return dialog.addFocus() + 2
	}
	return dialog.addFocus() + 1
}

func (dialog *conditionalDialogState) cancelFocus() int {
	return dialog.doneFocus() + 1
}

func (dialog *conditionalDialogState) setThreeColor(enabled bool) {
	if dialog.Mode == conditionalColorScale {
		if enabled && !dialog.ThreeColor && dialog.Focus == 4 {
			dialog.Focus = 5
		} else if !enabled && dialog.ThreeColor && dialog.Focus == 5 {
			dialog.Focus = 4
		}
	}
	dialog.ThreeColor = enabled
	dialog.ScaleDraftActive = true
	dialog.Error = ""
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

func (dialog *conditionalDialogState) removeLastRule() {
	if len(dialog.Rules) == 0 {
		return
	}
	dialog.Rules = dialog.Rules[:len(dialog.Rules)-1]
	dialog.Focus = dialog.addFocus()
	dialog.Error = ""
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
	dialog.ConditionMenuOpen = false
	dialog.Error = ""
	if dialog.Mode == conditionalColorScale {
		dialog.ScaleDraftActive = false
		dialog.Focus = 3
	} else {
		dialog.Focus = 2
	}
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
		if !dialog.ScaleDraftActive {
			return nil, nil
		}
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
		dialog.ScaleDraftActive = true
	case "mid":
		dialog.Scale.MidRGB = rgb
		dialog.ScaleDraftActive = true
	case "max":
		dialog.Scale.MaxRGB = rgb
		dialog.ScaleDraftActive = true
	}
	dialog.Error = ""
}

type conditionalDialogLayout struct {
	View     string
	Bounds   overlayRect
	Controls []conditionalDialogControl
}

type conditionalDialogControl struct {
	XStart int
	XEnd   int
	Y      int
	Action string
	Value  int
	ClickX int
}

func (dialog *conditionalDialogState) View(width, height int) string {
	return dialog.layout(width, height).View
}

func (dialog *conditionalDialogState) inputView(input *textinput.Model) string {
	value := []rune(input.Value())
	if !input.Focused() {
		return input.PromptStyle.Render(input.Prompt) + input.TextStyle.Render(string(value))
	}
	position := clamp(input.Position(), 0, len(value))
	return input.PromptStyle.Render(input.Prompt) +
		input.TextStyle.Render(string(value[:position])) +
		input.Cursor.View() +
		input.TextStyle.Render(string(value[position:]))
}

func (dialog *conditionalDialogState) layout(width, height int) conditionalDialogLayout {
	width = max(width, 1)
	height = max(height, 1)
	frameWidth := min(76, max(width-4, 1))
	innerWidth := max(frameWidth-4, 1)
	compact := width <= 42 || height <= 12
	dialog.RangeInput.Prompt = "Range: "
	dialog.RangeInput.Width = max(innerWidth-lipgloss.Width(dialog.RangeInput.Prompt), 1)
	dialog.ValueInput.Prompt = "Value: "
	dialog.ValueInput.Width = max(innerWidth-lipgloss.Width(dialog.ValueInput.Prompt), 1)

	accent := lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6))
	dim := lipgloss.NewStyle().Foreground(ANSIColor(8))
	selected := func(value string, focused bool) string {
		style := lipgloss.NewStyle()
		if focused {
			style = style.Reverse(true).Bold(true)
		}
		return style.Render(value)
	}
	scaleChoice := func(value string, active, focused bool) string {
		style := lipgloss.NewStyle()
		if active {
			style = style.Foreground(ANSIColor(6)).Bold(true)
		}
		if focused {
			style = style.Reverse(true).Bold(true)
		}
		return style.Render(value)
	}
	lines := make([]string, 0, 24)
	controls := make([]conditionalDialogControl, 0, 16)
	addLine := func(value string) int {
		row := len(lines)
		lines = append(lines, truncate(value, innerWidth))
		return row
	}
	addFullControl := func(row int, action string, value int) {
		controls = append(controls, conditionalDialogControl{
			XStart: 2, XEnd: 2 + innerWidth, Y: row + 1, Action: action, Value: value,
		})
	}
	addTextControl := func(row int, text, action string, value int) {
		index := strings.Index(lines[row], text)
		if index < 0 {
			return
		}
		x := 2 + lipgloss.Width(lines[row][:index])
		controls = append(controls, conditionalDialogControl{
			XStart: x, XEnd: x + lipgloss.Width(text), Y: row + 1, Action: action, Value: value,
		})
	}
	addButtonRow := func() {
		row := addLine(selected("Cancel", dialog.Focus == dialog.cancelFocus()) + "    " + selected("Done", dialog.Focus == dialog.doneFocus()))
		addTextControl(row, "Cancel", "cancel", 0)
		addTextControl(row, "Done", "done", 0)
	}
	addDrafts := func() {
		if compact {
			if len(dialog.Rules) == 0 {
				addLine(dim.Render("Drafts: none"))
				return
			}
			summaries := make([]string, 0, len(dialog.Rules))
			for index, rule := range dialog.Rules {
				summaries = append(summaries, conditionalRuleSummary(index, rule))
			}
			addLine(truncate("Drafts: "+strings.Join(summaries, " · "), innerWidth))
			return
		}
		addLine(accent.Render("Current draft rules"))
		if len(dialog.Rules) == 0 {
			addLine(dim.Render("No rules added yet"))
			return
		}
		for index, rule := range dialog.Rules {
			addLine(truncate(conditionalRuleSummary(index, rule), innerWidth))
		}
	}
	addSectionGap := func() {
		if height >= 24 && !dialog.ConditionMenuOpen {
			addLine("")
		}
	}

	if compact {
		modeRow := addLine(selected("Single", dialog.Mode == conditionalSingleColor && dialog.Focus == 1) + "   " + selected("Scale", dialog.Mode == conditionalColorScale && dialog.Focus == 1))
		addTextControl(modeRow, "Single", "single", 0)
		addTextControl(modeRow, "Scale", "scale", 0)
		rangeRow := addLine(dialog.inputView(&dialog.RangeInput))
		addFullControl(rangeRow, "range", 0)
		if dialog.Mode == conditionalSingleColor {
			operator := "If: " + selected(conditionalOperators[dialog.Operator].label+"  ▾", dialog.Focus == 2)
			conditionRow := addLine(operator)
			addFullControl(conditionRow, "condition", 0)
			if dialog.ConditionMenuOpen {
				for start := 0; start < len(conditionalOperators); start += 3 {
					end := min(start+3, len(conditionalOperators))
					labels := make([]string, 0, end-start)
					for index := start; index < end; index++ {
						label := conditionalOperators[index].label
						if conditionalOperators[index].operator == workbook.GreaterThan {
							label = "Greater"
						} else if conditionalOperators[index].operator == workbook.LessThan {
							label = "Less"
						} else if conditionalOperators[index].operator == workbook.Equal {
							label = "Equal"
						} else if conditionalOperators[index].operator == workbook.Blank {
							label = "Blank"
						} else {
							label = "Text"
						}
						labels = append(labels, label)
					}
					row := addLine(strings.Join(labels, "   "))
					for offset, label := range labels {
						addTextControl(row, label, "operator", start+offset)
					}
				}
			}
			valueRow := addLine(dialog.inputView(&dialog.ValueInput))
			addFullControl(valueRow, "value", 0)
			styleRow := addLine(selected("F:"+compactColor(dialog.FillRGB), dialog.Focus == 4) + "   " + selected("T:"+compactColor(dialog.FontRGB), dialog.Focus == 5))
			addTextControl(styleRow, "F:", "fill", 0)
			addTextControl(styleRow, "T:", "font", 0)
			if !dialog.ConditionMenuOpen {
				if dialog.Error != "" {
					addLine(lipgloss.NewStyle().Foreground(ANSIColor(1)).Render(dialog.Error))
				}
				addDrafts()
			}
		} else {
			toggleRow := addLine(scaleChoice("2-color", !dialog.ThreeColor, dialog.Focus == 2) + "   " + scaleChoice("3-color", dialog.ThreeColor, dialog.Focus == 2))
			addTextControl(toggleRow, "2-color", "toggle-scale", 2)
			addTextControl(toggleRow, "3-color", "toggle-scale", 3)
			minimumRow := addLine(selected("Min: "+compactColor(dialog.Scale.MinRGB), dialog.Focus == 3))
			addFullControl(minimumRow, "min", 0)
			if dialog.ThreeColor {
				midpointRow := addLine(selected("Mid: "+compactColor(dialog.Scale.MidRGB), dialog.Focus == 4))
				addFullControl(midpointRow, "mid", 0)
			}
			maximumRow := addLine(selected("Max: "+compactColor(dialog.Scale.MaxRGB), dialog.Focus == 5))
			addFullControl(maximumRow, "max", 0)
			if dialog.Error != "" {
				addLine(lipgloss.NewStyle().Foreground(ANSIColor(1)).Render(dialog.Error))
			} else {
				addDrafts()
			}
		}
		if !dialog.ConditionMenuOpen {
			actionText := selected("Add", dialog.Focus == dialog.addFocus())
			if len(dialog.Rules) > 0 {
				actionText += " " + selected("Remove", dialog.Focus == dialog.removeFocus())
			}
			actionText += " " + selected("Done", dialog.Focus == dialog.doneFocus()) + " " + selected("Cancel", dialog.Focus == dialog.cancelFocus())
			actionRow := addLine(actionText)
			addTextControl(actionRow, "Add", "add", 0)
			if len(dialog.Rules) > 0 {
				addTextControl(actionRow, "Remove", "remove-last", 0)
			}
			addTextControl(actionRow, "Done", "done", 0)
			addTextControl(actionRow, "Cancel", "cancel", 0)
		} else {
			actionRow := addLine(selected("Done", dialog.Focus == dialog.doneFocus()) + "   " + selected("Cancel", dialog.Focus == dialog.cancelFocus()))
			addTextControl(actionRow, "Done", "done", 0)
			addTextControl(actionRow, "Cancel", "cancel", 0)
		}
	} else {
		addLine(accent.Render("Conditional formatting"))
		tabRow := addLine(selected("Single color", dialog.Mode == conditionalSingleColor && dialog.Focus == 1) + "    " + selected("Color scale", dialog.Mode == conditionalColorScale && dialog.Focus == 1))
		addTextControl(tabRow, "Single color", "single", 0)
		addTextControl(tabRow, "Color scale", "scale", 0)
		addSectionGap()
		addLine(accent.Render("Apply to range"))
		rangeRow := addLine(dialog.inputView(&dialog.RangeInput))
		addFullControl(rangeRow, "range", 0)
		addSectionGap()
		addLine(accent.Render("Rule type and criteria"))
		if dialog.Mode == conditionalSingleColor {
			conditionRow := addLine("Condition: " + selected(conditionalOperators[dialog.Operator].label+"  ▾", dialog.Focus == 2))
			addFullControl(conditionRow, "condition", 0)
			if dialog.ConditionMenuOpen {
				for index, operator := range conditionalOperators {
					row := addLine("  " + selected(operator.label, dialog.Focus == 2 && dialog.Operator == index))
					addFullControl(row, "operator", index)
				}
			}
			valueRow := addLine(dialog.inputView(&dialog.ValueInput))
			addFullControl(valueRow, "value", 0)
			addSectionGap()
			addLine(accent.Render("Formatting style"))
			fillText := "Fill color: " + colorName(dialog.FillRGB) + "   click to choose"
			fillRow := addLine(selected(fillText, dialog.Focus == 4))
			addFullControl(fillRow, "fill", 0)
			fontText := "Font color: " + colorName(dialog.FontRGB) + "   click to choose"
			fontRow := addLine(selected(fontText, dialog.Focus == 5))
			addFullControl(fontRow, "font", 0)
			addLine(dialog.previewView())
			addSectionGap()
		} else {
			addSectionGap()
			addLine(accent.Render("Formatting style"))
			toggleRow := addLine(scaleChoice("2-color scale", !dialog.ThreeColor, dialog.Focus == 2) + "    " + scaleChoice("3-color scale", dialog.ThreeColor, dialog.Focus == 2))
			addTextControl(toggleRow, "2-color scale", "toggle-scale", 2)
			addTextControl(toggleRow, "3-color scale", "toggle-scale", 3)
			minimumRow := addLine(selected("Minimum: "+colorName(dialog.Scale.MinRGB)+"   click to choose", dialog.Focus == 3))
			addFullControl(minimumRow, "min", 0)
			if dialog.ThreeColor {
				midpointRow := addLine(selected("Midpoint: "+colorName(dialog.Scale.MidRGB)+"   click to choose", dialog.Focus == 4))
				addFullControl(midpointRow, "mid", 0)
			}
			maximumRow := addLine(selected("Maximum: "+colorName(dialog.Scale.MaxRGB)+"   click to choose", dialog.Focus == 5))
			addFullControl(maximumRow, "max", 0)
			addLine(dialog.previewView())
			addSectionGap()
		}
		addLine(accent.Render("Current draft rules"))
		if len(dialog.Rules) == 0 {
			addLine(dim.Render("No rules added yet"))
		} else {
			for index, rule := range dialog.Rules {
				addLine(truncate(conditionalRuleSummary(index, rule), innerWidth))
			}
		}
		addRow := addLine(selected("+ Add another rule", dialog.Focus == dialog.addFocus()) + func() string {
			if len(dialog.Rules) > 0 {
				return "   " + selected("− Remove last rule", dialog.Focus == dialog.removeFocus())
			}
			return ""
		}())
		addTextControl(addRow, "+ Add another rule", "add", 0)
		if len(dialog.Rules) > 0 {
			addTextControl(addRow, "− Remove last rule", "remove-last", 0)
		}
		if dialog.Error != "" {
			addLine(lipgloss.NewStyle().Foreground(ANSIColor(1)).Render(dialog.Error))
		}
		addButtonRow()
	}

	popup := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ANSIColor(6)).Padding(0, 1).Width(innerWidth).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return conditionalDialogLayout{View: popup, Bounds: overlayBounds(popup, width, height), Controls: controls}
}

func (dialog *conditionalDialogState) HandleMouse(message tea.MouseMsg, width, height int) string {
	event := tea.MouseEvent(message)
	if !dialog.Open || event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return ""
	}
	layout := dialog.layout(width, height)
	if event.X < layout.Bounds.X || event.X >= layout.Bounds.X+layout.Bounds.Width || event.Y < layout.Bounds.Y || event.Y >= layout.Bounds.Y+layout.Bounds.Height {
		return ""
	}
	x, y := event.X-layout.Bounds.X, event.Y-layout.Bounds.Y
	for _, control := range layout.Controls {
		if x >= control.XStart && x < control.XEnd && y == control.Y {
			control.ClickX = x
			return dialog.activateMouseControl(control)
		}
	}
	return ""
}

func (dialog *conditionalDialogState) activateMouseControl(control conditionalDialogControl) string {
	switch control.Action {
	case "single":
		dialog.Mode = conditionalSingleColor
		dialog.ConditionMenuOpen = false
		dialog.Focus = 1
		dialog.Error = ""
		dialog.syncInputFocus()
	case "scale":
		dialog.Mode = conditionalColorScale
		dialog.ConditionMenuOpen = false
		dialog.Focus = 1
		dialog.Error = ""
		dialog.syncInputFocus()
	case "range":
		dialog.Focus = 0
		dialog.RangeInput.SetCursor(clamp(control.ClickX-control.XStart-lipgloss.Width(dialog.RangeInput.Prompt), 0, len([]rune(dialog.RangeInput.Value()))))
		dialog.syncInputFocus()
	case "value":
		dialog.Focus = 3
		dialog.ValueInput.SetCursor(clamp(control.ClickX-control.XStart-lipgloss.Width(dialog.ValueInput.Prompt), 0, len([]rune(dialog.ValueInput.Value()))))
		dialog.syncInputFocus()
	case "condition":
		dialog.ConditionMenuOpen = !dialog.ConditionMenuOpen
		dialog.Focus = 2
		dialog.Error = ""
		dialog.syncInputFocus()
	case "operator":
		dialog.Operator = control.Value
		dialog.ConditionMenuOpen = false
		dialog.Focus = 2
		dialog.Error = ""
		dialog.syncInputFocus()
	case "fill":
		dialog.Focus = 4
		dialog.ColorField = "fill"
		return dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	case "font":
		dialog.Focus = 5
		dialog.ColorField = "font"
		return dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	case "min":
		dialog.Focus = 3
		dialog.ColorField = "min"
		return dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	case "mid":
		dialog.Focus = 4
		dialog.ColorField = "mid"
		return dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	case "max":
		dialog.Focus = 5
		if dialog.Mode == conditionalColorScale && !dialog.ThreeColor {
			dialog.Focus = 4
		}
		dialog.ColorField = "max"
		return dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	case "toggle-scale":
		dialog.setThreeColor(control.Value == 3)
		dialog.Focus = 2
		dialog.syncInputFocus()
	case "remove-last":
		dialog.Focus = dialog.removeFocus()
		return dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	case "add":
		dialog.Focus = dialog.addFocus()
		return dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	case "done":
		dialog.Focus = dialog.doneFocus()
		return dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	case "cancel":
		dialog.Focus = dialog.cancelFocus()
		return dialog.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	}
	return ""
}

func conditionalRuleSummary(index int, rule workbook.ConditionalRule) string {
	if rule.ColorScale != nil {
		scale := rule.ColorScale
		if scale.MidRGB == "" {
			return fmt.Sprintf("%d. Color scale %s → %s", index+1, colorName(scale.MinRGB), colorName(scale.MaxRGB))
		}
		return fmt.Sprintf("%d. Color scale %s → %s → %s", index+1, colorName(scale.MinRGB), colorName(scale.MidRGB), colorName(scale.MaxRGB))
	}
	label := "Condition"
	for _, operator := range conditionalOperators {
		if operator.operator == rule.Operator {
			label = operator.label
			break
		}
	}
	style := colorName(rule.FillRGB)
	if rule.FillRGB == "" && rule.FontRGB != "" {
		style = "font " + colorName(rule.FontRGB)
	}
	return fmt.Sprintf("%d. %s %s · %s", index+1, label, rule.Value, style)
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

func (dialog *conditionalDialogState) previewView() string {
	if dialog.Mode == conditionalColorScale {
		colors := []string{dialog.Scale.MinRGB}
		if dialog.ThreeColor {
			colors = append(colors, dialog.Scale.MidRGB)
		}
		colors = append(colors, dialog.Scale.MaxRGB)
		var preview strings.Builder
		for index, rgb := range colors {
			if index > 0 {
				preview.WriteByte(' ')
			}
			if color, err := ParseRGBHex("#" + strings.TrimPrefix(rgb, "#")); err == nil {
				preview.WriteString(lipgloss.NewStyle().Background(TerminalColor(color, supportsTrueColor())).Render("   "))
			} else {
				preview.WriteString("   ")
			}
		}
		return "Preview: " + preview.String() + " " + dialog.Preview()
	}
	style := lipgloss.NewStyle().Padding(0, 1)
	if dialog.FillRGB != "" {
		if color, err := ParseRGBHex("#" + strings.TrimPrefix(dialog.FillRGB, "#")); err == nil {
			style = style.Background(TerminalColor(color, supportsTrueColor()))
		}
	}
	if dialog.FontRGB != "" {
		if color, err := ParseRGBHex("#" + strings.TrimPrefix(dialog.FontRGB, "#")); err == nil {
			style = style.Foreground(TerminalColor(color, supportsTrueColor()))
		}
	}
	return "Preview: " + style.Render(dialog.Preview())
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
