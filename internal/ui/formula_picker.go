package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xuri/excelize/v2"
)

type functionDefinition struct {
	name        string
	description string
	arguments   string
}

type functionItem struct {
	definition functionDefinition
}

func (item functionItem) FilterValue() string {
	return item.definition.name + " " + item.definition.description
}
func (item functionItem) Title() string       { return item.definition.name }
func (item functionItem) Description() string { return item.definition.description }

type formulaPickerState struct {
	Open        bool
	Functions   list.Model
	RangeInput  textinput.Model
	TargetInput textinput.Model
	Focus       int
}

var supportedFunctions = []functionDefinition{
	{name: "SUM", description: "Add numbers in a range", arguments: "range"},
	{name: "AVERAGE", description: "Average numbers in a range", arguments: "range"},
	{name: "COUNT", description: "Count numeric cells", arguments: "range"},
	{name: "MIN", description: "Smallest number in a range", arguments: "range"},
	{name: "MAX", description: "Largest number in a range", arguments: "range"},
	{name: "IF", description: "Choose a value when a condition is true", arguments: "condition, true value, false value"},
}

func newFormulaPicker(rangeName, resultCell string) formulaPickerState {
	items := make([]list.Item, 0, len(supportedFunctions))
	for _, function := range supportedFunctions {
		items = append(items, functionItem{definition: function})
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(ANSIColor(6)).Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(ANSIColor(7))
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(ANSIColor(7))
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(ANSIColor(8))
	functions := list.New(items, delegate, 30, 9)
	functions.Title = "Insert function"
	functions.SetShowStatusBar(false)
	functions.SetShowPagination(false)
	functions.SetShowHelp(false)
	functions.SetShowTitle(true)
	functions.Styles.Title = lipgloss.NewStyle().Foreground(ANSIColor(6)).Bold(true)

	rangeInput := newFormulaInput("Argument / range: ", rangeName)
	targetInput := newFormulaInput("Result: ", resultCell)
	return formulaPickerState{Open: true, Functions: functions, RangeInput: rangeInput, TargetInput: targetInput}
}

func newFormulaInput(prompt, value string) textinput.Model {
	input := textinput.New()
	input.Prompt = prompt
	input.SetValue(value)
	input.CursorEnd()
	input.Width = 32
	input.PromptStyle = lipgloss.NewStyle().Foreground(ANSIColor(8))
	input.TextStyle = lipgloss.NewStyle().Foreground(ANSIColor(7))
	input.Cursor.Style = lipgloss.NewStyle().Foreground(ANSIColor(6))
	return input
}

func (picker *formulaPickerState) HandleKey(message tea.KeyMsg) (submitted, cancelled bool) {
	if !picker.Open {
		return false, false
	}
	switch message.Type {
	case tea.KeyEsc:
		picker.Open = false
		return false, true
	case tea.KeyTab:
		picker.focusNext()
		return false, false
	case tea.KeyEnter:
		if picker.Focus == 2 {
			return true, false
		}
		picker.focusNext()
		return false, false
	}
	switch picker.Focus {
	case 0:
		previous := picker.selectedFunctionName()
		picker.Functions, _ = picker.Functions.Update(message)
		current := picker.selectedFunctionName()
		if current != previous {
			if current == "IF" {
				picker.RangeInput.SetValue("")
			}
			if current == "IF" {
				picker.RangeInput.Prompt = "Arguments: "
			} else {
				picker.RangeInput.Prompt = "Range: "
			}
		}
	case 1:
		picker.RangeInput, _ = picker.RangeInput.Update(message)
	case 2:
		picker.TargetInput, _ = picker.TargetInput.Update(message)
	}
	return false, false
}

func (picker *formulaPickerState) selectedFunctionName() string {
	item, ok := picker.Functions.SelectedItem().(functionItem)
	if !ok {
		return ""
	}
	return item.definition.name
}

func (picker *formulaPickerState) focusNext() {
	picker.RangeInput.Blur()
	picker.TargetInput.Blur()
	picker.Focus = min(picker.Focus+1, 2)
	switch picker.Focus {
	case 1:
		picker.RangeInput.Focus()
	case 2:
		picker.TargetInput.Focus()
	}
}

func (picker *formulaPickerState) Formula() (string, error) {
	item, ok := picker.Functions.SelectedItem().(functionItem)
	if !ok {
		return "", fmt.Errorf("choose a function")
	}
	argument := strings.TrimSpace(picker.RangeInput.Value())
	if argument == "" {
		return "", fmt.Errorf("enter a range or function argument")
	}
	switch item.definition.name {
	case "SUM", "AVERAGE", "COUNT", "MIN", "MAX":
		if _, _, _, _, err := parseA1Range(argument); err != nil {
			return "", fmt.Errorf("invalid range %q: %w", argument, err)
		}
		return "=" + item.definition.name + "(" + argument + ")", nil
	case "IF":
		return "=IF(" + argument + ")", nil
	default:
		return "", fmt.Errorf("unsupported function %q", item.definition.name)
	}
}

func (picker *formulaPickerState) Preview() string {
	formula, err := picker.Formula()
	if err != nil {
		return err.Error()
	}
	return formula
}

func (picker *formulaPickerState) View(width int) string {
	frameWidth := min(78, max(width-4, 1))
	innerWidth := max(frameWidth-4, 1)
	listWidth := min(31, max(innerWidth/2, 1))
	formWidth := max(innerWidth-listWidth-2, 1)
	picker.Functions.SetSize(listWidth, 10)
	picker.RangeInput.Width = max(formWidth-10, 1)
	picker.TargetInput.Width = max(formWidth-10, 1)

	functionView := picker.Functions.View()
	label := lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6))
	dim := lipgloss.NewStyle().Foreground(ANSIColor(8))
	preview := picker.Preview()
	fields := lipgloss.JoinVertical(lipgloss.Left,
		label.Render("Syntax / example"),
		dim.Render(picker.Syntax()),
		"",
		label.Render("Arguments"),
		picker.RangeInput.View(),
		picker.TargetInput.View(),
		"",
		label.Render("Formula preview"),
		lipgloss.NewStyle().Foreground(ANSIColor(7)).Render(preview),
		"",
		dim.Render("Enter / Tab: next field   Esc: cancel"),
	)
	content := lipgloss.JoinHorizontal(lipgloss.Top, functionView, "  ", lipgloss.NewStyle().Width(formWidth).Render(fields))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ANSIColor(6)).
		Padding(0, 1).
		Width(innerWidth).
		Render(content)
}

func (picker *formulaPickerState) Syntax() string {
	item, ok := picker.Functions.SelectedItem().(functionItem)
	if !ok {
		return ""
	}
	return item.definition.name + "(" + item.definition.arguments + ")"
}

func parseA1Range(value string) (int, int, int, int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) < 1 || len(parts) > 2 {
		return 0, 0, 0, 0, fmt.Errorf("expected A1 or A1:B2")
	}
	startCol, startRow, err := excelize.CellNameToCoordinates(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, 0, 0, err
	}
	endCol, endRow := startCol, startRow
	if len(parts) == 2 {
		endCol, endRow, err = excelize.CellNameToCoordinates(strings.TrimSpace(parts[1]))
		if err != nil {
			return 0, 0, 0, 0, err
		}
	}
	return min(startCol, endCol) - 1, min(startRow, endRow) - 1, max(startCol, endCol) - 1, max(startRow, endRow) - 1, nil
}

func parseA1Cell(value string) (int, int, error) {
	column, row, err := excelize.CellNameToCoordinates(strings.TrimSpace(value))
	if err != nil {
		return 0, 0, err
	}
	return column - 1, row - 1, nil
}
