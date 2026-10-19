package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"omasheet/internal/workbook"
)

type colorPickerState struct {
	Open         bool
	Target       colorTarget
	Selected     int
	EditingInput bool
	Input        string
}

type colorTarget string

const (
	fillColorTarget              colorTarget = "fill"
	fontColorTarget              colorTarget = "font"
	conditionalDialogColorTarget colorTarget = "conditional-dialog"
)

type pendingOperation struct {
	Mode  string
	Input string
}

type AppModel struct {
	session           *workbook.Session
	Grid              GridViewport
	Palette           CommandPalette
	FunctionPicker    formulaPickerState
	ConditionalDialog conditionalDialogState
	home              homeScreen
	keys              KeyMap
	Theme             ThemePalette
	ColorPicker       colorPickerState
	toolbarVisible    bool
	width             int
	height            int
	editing           bool
	editorInput       textinput.Model
	findMode          bool
	findQuery         string
	themeQuery        func(uint8, time.Duration) (RGB, bool)
	status            string
	lastCommand       string
	operation         pendingOperation
	visibleSheet      string
	filtersDirty      bool
}

func NewModel(session *workbook.Session) AppModel {
	keys := DefaultKeyMap()
	editor := textinput.New()
	editor.Prompt = "Edit: "
	editor.Width = 68
	editor.PromptStyle = lipgloss.NewStyle().Foreground(ANSIColor(6)).Bold(true)
	editor.TextStyle = lipgloss.NewStyle().Foreground(ANSIColor(7))
	editor.Cursor.Style = lipgloss.NewStyle().Foreground(ANSIColor(6))
	return AppModel{
		session:        session,
		Grid:           NewGridViewport(),
		Palette:        NewCommandPalette(keys),
		home:           newHomeScreen(),
		Theme:          NewThemePalette(),
		keys:           keys,
		editorInput:    editor,
		toolbarVisible: true,
		width:          80,
		height:         24,
	}
}

func (model *AppModel) Init() tea.Cmd {
	return nil
}

func (model *AppModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if model == nil {
		return model, nil
	}
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		model.layout()
	case tea.MouseMsg:
		model.handleMouse(message)
	case tea.KeyMsg:
		if message.Type == tea.KeyCtrlC {
			return model, tea.Quit
		}
		model.handleKey(message)
	}
	return model, nil
}

func (model *AppModel) View() string {
	if model == nil {
		return ""
	}
	model.layout()
	if model.session == nil || model.session.Workbook == nil || len(model.session.Workbook.Sheets) == 0 {
		content := model.home.View(model.width, model.height)
		if model.Palette.Open {
			return ComposeOverlay(content, model.Palette.View(model.width), model.width, model.height)
		}
		return content
	}
	sheet := model.activeSheet()
	var output strings.Builder
	if model.toolbarVisible {
		output.WriteString(model.toolbarView())
		output.WriteByte('\n')
	}
	output.WriteString(model.formulaBar())
	output.WriteByte('\n')
	gridHeight := max(model.height-model.topLines()-2, 1)
	output.WriteString(model.Grid.View(sheet, gridHeight))
	output.WriteByte('\n')
	output.WriteString(model.sheetTabs())
	output.WriteByte('\n')
	statusStyle := lipgloss.NewStyle().Foreground(ANSIColor(8))
	status := model.status
	if status == "" {
		status = fmt.Sprintf("%s%d  Ctrl+P commands  Ctrl+S save", columnName(model.Grid.Selected.Col), model.Grid.Selected.Row+1)
	}
	status = truncate(status, max(model.width, 1))
	output.WriteString(statusStyle.Render(status))
	if model.editing || model.findMode || model.operation.Mode != "" {
		output.WriteByte('\n')
		output.WriteString(model.inputOverlay())
	}
	if model.Palette.Open {
		return ComposeOverlay(output.String(), model.Palette.View(model.width), model.width, model.height)
	}
	if model.FunctionPicker.Open {
		return ComposeOverlay(output.String(), model.FunctionPicker.View(model.width), model.width, model.height)
	}
	if model.ConditionalDialog.Open {
		view := ComposeOverlay(output.String(), model.ConditionalDialog.View(model.width, model.height), model.width, model.height)
		if model.ColorPicker.Open {
			return ComposeOverlay(view, model.colorPickerView(), model.width, model.height)
		}
		return view
	}
	if model.ColorPicker.Open {
		return ComposeOverlay(output.String(), model.colorPickerView(), model.width, model.height)
	}
	return output.String()
}

func (model *AppModel) handleKey(message tea.KeyMsg) {
	if model.keys.Matches(message, model.keys.OpenPalette) {
		model.Palette.Show()
		model.findMode = false
		model.ColorPicker.Open = false
		return
	}
	if model.ColorPicker.Open {
		model.handleColorKey(message)
		return
	}
	if model.Palette.Open {
		command, invoked := model.Palette.HandleKey(message)
		if invoked {
			model.runCommand(command)
		}
		return
	}
	if model.ConditionalDialog.Open {
		model.handleConditionalDialogKey(message)
		return
	}
	if model.FunctionPicker.Open {
		model.handleFormulaPickerKey(message)
		return
	}
	if model.operation.Mode != "" {
		model.handleOperationKey(message)
		return
	}
	if model.findMode {
		model.handleFindKey(message)
		return
	}
	if model.editing {
		model.handleEditKey(message)
		return
	}
	if model.keys.Matches(message, model.keys.Find) {
		model.findMode = true
		model.findQuery = ""
		return
	}
	if model.keys.Matches(message, model.keys.ToggleToolbar) {
		model.ToggleToolbar()
		return
	}
	if model.session == nil || model.session.Workbook == nil || len(model.session.Workbook.Sheets) == 0 {
		if action := model.home.HandleKey(message); action != "" {
			model.runCommand(action)
		}
		return
	}
	switch {
	case model.keys.Matches(message, model.keys.Undo):
		model.setStatus(workbook.Undo(model.session))
		model.filtersDirty = true
	case model.keys.Matches(message, model.keys.Redo):
		model.setStatus(workbook.Redo(model.session))
		model.filtersDirty = true
	case model.keys.Matches(message, model.keys.PreviousSheet):
		model.changeSheet(-1)
	case model.keys.Matches(message, model.keys.NextSheet):
		model.changeSheet(1)
	case model.keys.Matches(message, model.keys.BeginEdit):
		model.startEditing(model.activeSheet().Cells[model.Grid.Selected].Input)
	case model.keys.Matches(message, model.keys.MoveUp):
		model.moveSelection(-1, 0)
	case model.keys.Matches(message, model.keys.MoveDown):
		model.moveSelection(1, 0)
	case model.keys.Matches(message, model.keys.MoveLeft):
		model.moveSelection(0, -1)
	case model.keys.Matches(message, model.keys.MoveRight):
		model.moveSelection(0, 1)
	case message.Type == tea.KeySpace:
		model.startEditing(" ")
	case message.Type == tea.KeyRunes && len(message.Runes) > 0:
		model.startEditing(string(message.Runes))
	}
}

func (model *AppModel) handleEditKey(message tea.KeyMsg) {
	switch {
	case model.keys.Matches(message, model.keys.CancelEdit):
		model.editing = false
		model.editorInput.Blur()
		model.editorInput.Reset()
	case model.keys.Matches(message, model.keys.CommitDown):
		model.commitEdit(1, 0)
	case model.keys.Matches(message, model.keys.CommitRight):
		model.commitEdit(0, 1)
	default:
		if message.Type == tea.KeySpace {
			message.Type = tea.KeyRunes
			message.Runes = []rune{' '}
		}
		model.editorInput, _ = model.editorInput.Update(message)
	}
}

func (model *AppModel) startEditing(value string) {
	model.editorInput.SetValue(value)
	model.editorInput.CursorEnd()
	model.editorInput.Focus()
	model.editing = true
}

func (model *AppModel) commitEdit(rowDelta, colDelta int) {
	if model.session == nil || model.activeSheet() == nil {
		return
	}
	address := workbook.CellAddress{Sheet: model.activeSheet().Name, Row: model.Grid.Selected.Row, Col: model.Grid.Selected.Col}
	err := workbook.SetCell(model.session, address, model.editorInput.Value())
	model.setStatus(err)
	if err != nil {
		return
	}
	model.filtersDirty = true
	model.editing = false
	model.editorInput.Blur()
	model.editorInput.Reset()
	model.moveSelection(rowDelta, colDelta)
}

func (model *AppModel) handleFindKey(message tea.KeyMsg) {
	switch message.Type {
	case tea.KeyEsc:
		model.findMode = false
		model.findQuery = ""
	case tea.KeyBackspace:
		runes := []rune(model.findQuery)
		if len(runes) > 0 {
			model.findQuery = string(runes[:len(runes)-1])
		}
	case tea.KeyEnter:
		matches := workbook.Find(model.session, model.activeSheet().Name, model.findQuery)
		if len(matches) > 0 {
			model.selectCell(workbook.Coord{Row: matches[0].Row, Col: matches[0].Col})
		}
		model.status = fmt.Sprintf("%d matches for %q", len(matches), model.findQuery)
		model.findMode = false
	case tea.KeyRunes:
		model.findQuery += string(message.Runes)
	}
}

func (model *AppModel) handleColorKey(message tea.KeyMsg) {
	if model.ColorPicker.EditingInput {
		switch message.Type {
		case tea.KeyEsc:
			model.ColorPicker.Open = false
			model.ColorPicker.EditingInput = false
		case tea.KeyEnter:
			color, err := ParseRGBHex(model.ColorPicker.Input)
			if err != nil {
				model.status = err.Error()
				return
			}
			if err := model.Theme.AddCustom(FormatRGBHex(color)); err != nil {
				model.status = err.Error()
				return
			}
			model.applyColor(color)
			model.ColorPicker.Open = false
			model.ColorPicker.EditingInput = false
		case tea.KeyBackspace:
			runes := []rune(model.ColorPicker.Input)
			if len(runes) > 0 {
				model.ColorPicker.Input = string(runes[:len(runes)-1])
			}
		case tea.KeyRunes:
			model.ColorPicker.Input += string(message.Runes)
		}
		return
	}
	options := model.Theme.Options()
	customIndex := len(options)
	switch message.Type {
	case tea.KeyEsc:
		model.ColorPicker.Open = false
	case tea.KeyUp:
		model.ColorPicker.Selected = max(0, model.ColorPicker.Selected-1)
	case tea.KeyDown:
		model.ColorPicker.Selected = min(customIndex, model.ColorPicker.Selected+1)
	case tea.KeyEnter:
		if model.ColorPicker.Selected == customIndex {
			model.ColorPicker.EditingInput = true
			model.ColorPicker.Input = ""
			return
		}
		if model.ColorPicker.Selected < len(options) {
			option := options[model.ColorPicker.Selected]
			if option.Default {
				model.applyDefaultColor()
			} else {
				color := option.Color
				if !option.Custom {
					color = ResolveThemeColor(option.ANSI, model.themeQuery)
				}
				model.applyColor(color)
			}
			model.ColorPicker.Open = false
		}
	}
}
func (model *AppModel) handleConditionalDialogKey(message tea.KeyMsg) {
	model.handleConditionalDialogAction(model.ConditionalDialog.HandleKey(message))
}

func (model *AppModel) handleConditionalDialogAction(action string) {
	switch action {
	case "color":
		model.openColorPicker(conditionalDialogColorTarget)
	case "cancel":
		model.ColorPicker = colorPickerState{}
		model.ConditionalDialog = conditionalDialogState{}
	case "done":
		_, rules, err := model.ConditionalDialog.BuildRules()
		if err != nil {
			model.ConditionalDialog.Error = err.Error()
			return
		}
		applied := 0
		for _, rule := range rules {
			if rule.ColorScale != nil {
				err = workbook.AddColorScaleRule(model.session, model.activeSheet().Name, rule.Range, *rule.ColorScale)
			} else {
				err = workbook.AddConditionalRule(model.session, model.activeSheet().Name, rule.Range, rule)
			}
			if err != nil {
				for applied > 0 {
					if undoErr := workbook.Undo(model.session); undoErr != nil {
						model.ConditionalDialog.Error = fmt.Sprintf("%v; rollback failed: %v", err, undoErr)
						return
					}
					applied--
				}
				model.ConditionalDialog.Error = err.Error()
				return
			}
			applied++
		}
		model.ColorPicker = colorPickerState{}
		model.ConditionalDialog = conditionalDialogState{}
		model.status = fmt.Sprintf("Applied %d conditional formatting rule(s)", len(rules))
	}
}

func (model *AppModel) handleMouse(message tea.MouseMsg) {
	event := tea.MouseEvent(message)
	if model.ColorPicker.Open {
		model.handleColorPickerMouse(message)
		return
	}
	if model.Palette.Open {
		return
	}
	if model.ConditionalDialog.Open {
		action := model.ConditionalDialog.HandleMouse(message, model.width, model.height)
		model.handleConditionalDialogAction(action)
		return
	}
	if model.session == nil || model.session.Workbook == nil || len(model.session.Workbook.Sheets) == 0 {
		if action := model.home.HandleMouse(message); action != "" {
			model.runCommand(action)
		}
		return
	}
	if model.toolbarVisible && event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft && event.Y == 0 {
		model.runCommand(toolbarActionAt(event.X))
		return
	}
	if event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft && event.Y == model.sheetTabsY() {
		model.selectSheetAt(event.X)
		return
	}
	col, width, resized := model.Grid.HandleMouse(message, model.activeSheet())
	if resized {
		if err := workbook.SetColumnWidth(model.session, model.activeSheet().Name, col, float64(width)); err != nil {
			model.status = err.Error()
		}
	}
}

func (model *AppModel) runCommand(command string) {
	model.lastCommand = command
	if model.activeSheet() == nil {
		switch command {
		case "open-file", "new-csv", "new-xlsx", "save", "save-as", "toggle-toolbar", "hide-toolbar", "show-toolbar", "view", "file", "edit", "data", "format", "undo", "redo":
		default:
			return
		}
	}
	switch command {
	case "toggle-toolbar":
		model.ToggleToolbar()
	case "hide-toolbar":
		model.toolbarVisible = false
	case "show-toolbar":
		model.toolbarVisible = true
	case "view":
		model.ToggleToolbar()
	case "file", "edit", "data", "format":
		model.Palette.Show()
	case "undo":
		if model.session != nil {
			model.setStatus(workbook.Undo(model.session))
		}
	case "redo":
		if model.session != nil {
			model.setStatus(workbook.Redo(model.session))
		}
	case "find":
		model.findMode = true
		model.findQuery = ""
	case "previous-sheet":
		model.changeSheet(-1)
	case "next-sheet":
		model.changeSheet(1)
	case "fill-color":
		model.openColorPicker(fillColorTarget)
	case "font-color":
		model.openColorPicker(fontColorTarget)
	case "sort":
		model.beginOperation("sort")
	case "filter":
		model.beginOperation("filter")
	case "conditional-format":
		model.ConditionalDialog = newConditionalDialog(model.selectedRangeName())
		model.operation = pendingOperation{}
	case "insert-function":
		model.FunctionPicker = newFormulaPicker(model.selectedRangeName(), model.defaultFormulaTargetName())
	case "border-menu":
		model.Palette.Show()
		model.Palette.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("border")})
	case "border-all", "border-outside", "border-top", "border-bottom", "border-left", "border-right", "border-none":
		model.setStatus(workbook.ApplyBorderPreset(model.session, model.activeSheet().Name, model.selectedRange(), strings.TrimPrefix(command, "border-")))
	case "toggle-bold":
		style := model.activeSheet().Cells[model.Grid.Selected].Style
		style.Bold = !style.Bold
		model.applyCellStyle(style)
	case "add-sheet":
		nameIndex := len(model.session.Workbook.Sheets) + 1
		name := "Sheet" + itoa(nameIndex)
		for {
			exists := false
			for _, sheet := range model.session.Workbook.Sheets {
				if sheet.Name == name {
					exists = true
					break
				}
			}
			if !exists {
				break
			}
			nameIndex++
			name = "Sheet" + itoa(nameIndex)
		}
		model.setStatus(workbook.AddSheet(model.session, name))
	case "merge":
		result, err := workbook.MergeRange(model.session, model.activeSheet().Name, model.selectedRange(), false)
		if result == workbook.ConfirmationRequired {
			model.beginOperation("merge-confirm")
			model.status = "Merging discards cell values. Confirm? (y/n)"
		} else {
			model.setStatus(err)
		}
	case "unmerge":
		model.setStatus(workbook.UnmergeRange(model.session, model.activeSheet().Name, model.selectedRange()))
	case "insert-row":
		if model.session != nil {
			model.setStatus(workbook.InsertRows(model.session, model.activeSheet().Name, model.Grid.Selected.Row, 1))
		}
	case "insert-column":
		if model.session != nil {
			model.setStatus(workbook.InsertColumns(model.session, model.activeSheet().Name, model.Grid.Selected.Col, 1))
		}
	}
}
func (model *AppModel) TakeCommand() string {
	if model == nil {
		return ""
	}
	command := model.lastCommand
	model.lastCommand = ""
	return command
}

func (model *AppModel) selectedRange() workbook.CellRange {
	selection := model.Grid.Selection
	if selection == (workbook.CellRange{}) {
		selection = workbook.CellRange{Start: model.Grid.Selected, End: model.Grid.Selected}
	}
	if selection.Start.Row > selection.End.Row {
		selection.Start.Row, selection.End.Row = selection.End.Row, selection.Start.Row
	}
	if selection.Start.Col > selection.End.Col {
		selection.Start.Col, selection.End.Col = selection.End.Col, selection.Start.Col
	}
	return selection
}

func (model *AppModel) beginOperation(mode string) {
	model.operation = pendingOperation{Mode: mode}
}

func (model *AppModel) handleOperationKey(message tea.KeyMsg) {
	switch message.Type {
	case tea.KeyEsc:
		model.operation = pendingOperation{}
		model.status = ""
	case tea.KeyEnter:
		mode, input := model.operation.Mode, strings.TrimSpace(model.operation.Input)
		model.operation = pendingOperation{}
		model.applyOperation(mode, input)
	case tea.KeyBackspace:
		runes := []rune(model.operation.Input)
		if len(runes) > 0 {
			model.operation.Input = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		model.operation.Input += string(message.Runes)
	}
}

func (model *AppModel) applyOperation(mode, input string) {
	sheet := model.activeSheet()
	if sheet == nil {
		return
	}
	switch mode {
	case "merge-confirm":
		if strings.EqualFold(input, "y") {
			_, err := workbook.MergeRange(model.session, sheet.Name, model.selectedRange(), true)
			model.setStatus(err)
		} else {
			model.status = "Merge cancelled"
		}
	case "sort":
		selection := model.inferredDataRange()
		ascending, hasHeader := true, true
		switch strings.ToLower(input) {
		case "a":
		case "d":
			ascending = false
		case "h":
			hasHeader = false
		default:
			model.status = "Choose a (ascending), d (descending), or h (no header)"
			return
		}
		model.setStatus(workbook.SortRange(model.session, sheet.Name, selection, model.Grid.Selected.Col-selection.Start.Col, ascending, hasHeader))
		model.filtersDirty = true
	case "filter":
		allowed := make(map[string]bool)
		for _, value := range strings.Split(input, ";") {
			value = strings.TrimSpace(value)
			if value != "" {
				allowed[value] = true
			}
		}
		selection := model.sheetRange()
		column := model.Grid.Selected.Col - selection.Start.Col
		if len(allowed) == 0 {
			model.setStatus(workbook.SetFilter(model.session, sheet.Name, selection, nil))
			model.filtersDirty = true
			return
		}
		model.setStatus(workbook.SetFilter(model.session, sheet.Name, selection, []workbook.ColumnFilter{{Column: column, AllowedValues: allowed}}))
		model.filtersDirty = true
	}
}

func (model *AppModel) sheetRange() workbook.CellRange {
	sheet := model.activeSheet()
	return workbook.CellRange{Start: workbook.Coord{}, End: workbook.Coord{Row: max(sheet.Rows-1, 0), Col: max(sheet.Cols-1, 0)}}
}

func (model *AppModel) inferredDataRange() workbook.CellRange {
	sheet := model.activeSheet()
	rangeValue := model.sheetRange()
	row := min(max(model.Grid.Selected.Row, 0), rangeValue.End.Row)
	if !model.rowHasData(sheet, row) {
		return rangeValue
	}
	for row > 0 && model.rowHasData(sheet, row-1) {
		row--
	}
	start := row
	for row+1 < sheet.Rows && model.rowHasData(sheet, row+1) {
		row++
	}
	rangeValue.Start.Row = start
	rangeValue.End.Row = row
	return rangeValue
}

func (model *AppModel) rowHasData(sheet *workbook.Sheet, row int) bool {
	for col := 0; col < sheet.Cols; col++ {
		cell := sheet.Cells[workbook.Coord{Row: row, Col: col}]
		if cell.Input != "" || cell.Style != (workbook.CellStyle{}) {
			return true
		}
	}
	return false
}

func (model *AppModel) openColorPicker(target colorTarget) {
	model.ColorPicker = colorPickerState{Open: true, Target: target}
}

func (model *AppModel) applyColor(color RGB) {
	if model.session == nil || model.session.Workbook == nil {
		return
	}
	if model.ColorPicker.Target == conditionalDialogColorTarget {
		model.ConditionalDialog.SetColor(strings.TrimPrefix(FormatRGBHex(color), "#"))
		return
	}
	style := model.activeSheet().Cells[model.Grid.Selected].Style
	if model.ColorPicker.Target == fillColorTarget {
		style.FillRGB = strings.TrimPrefix(FormatRGBHex(color), "#")
	} else {
		style.FontRGB = strings.TrimPrefix(FormatRGBHex(color), "#")
	}
	model.applyCellStyle(style)
}

func (model *AppModel) applyDefaultColor() {
	if model.session == nil || model.session.Workbook == nil {
		return
	}
	if model.ColorPicker.Target == conditionalDialogColorTarget {
		model.ConditionalDialog.Error = "Conditional formatting needs a theme or custom color"
		return
	}
	style := model.activeSheet().Cells[model.Grid.Selected].Style
	if model.ColorPicker.Target == fillColorTarget {
		style.FillRGB = ""
	} else {
		style.FontRGB = ""
	}
	model.applyCellStyle(style)
}

func (model *AppModel) applyCellStyle(style workbook.CellStyle) {
	sheet := model.activeSheet()
	selection := model.Grid.Selection
	if selection == (workbook.CellRange{}) && model.Grid.Selected != (workbook.Coord{}) {
		selection = workbook.CellRange{Start: model.Grid.Selected, End: model.Grid.Selected}
	}
	model.setStatus(workbook.ApplyStyle(model.session, sheet.Name, selection, style))
}

func (model *AppModel) moveSelection(rowDelta, colDelta int) {
	selected := model.Grid.Selected
	if rowDelta != 0 && len(model.Grid.VisibleRowIndexes) > 0 {
		visible := model.Grid.VisibleRowIndexes
		position := sort.SearchInts(visible, selected.Row)
		if position == len(visible) {
			position--
		} else if visible[position] != selected.Row {
			position = min(position, len(visible)-1)
		}
		position = min(max(position+rowDelta, 0), len(visible)-1)
		selected.Row = visible[position]
	} else {
		selected.Row = max(0, selected.Row+rowDelta)
	}
	selected.Col = max(0, selected.Col+colDelta)
	model.selectCell(selected)
}

func (model *AppModel) selectCell(coord workbook.Coord) {
	model.Grid.Selected = coord
	model.Grid.Selection = workbook.CellRange{Start: coord, End: coord}
	model.ensureSelectedVisible()
}

func (model *AppModel) ensureSelectedVisible() {
	rowPosition := model.Grid.Selected.Row
	if visible := model.Grid.VisibleRowIndexes; len(visible) > 0 {
		rowPosition = sort.SearchInts(visible, model.Grid.Selected.Row)
		if rowPosition == len(visible) || visible[rowPosition] != model.Grid.Selected.Row {
			rowPosition = min(rowPosition, len(visible)-1)
			model.Grid.Selected.Row = visible[rowPosition]
			model.Grid.Selection = workbook.CellRange{Start: model.Grid.Selected, End: model.Grid.Selected}
		}
	}
	if rowPosition < model.Grid.ScrollRow {
		model.Grid.ScrollRow = rowPosition
	}
	visibleRows := max((model.Grid.Height-model.Grid.HeaderHeight)/2, 1)
	if rowPosition >= model.Grid.ScrollRow+visibleRows {
		model.Grid.ScrollRow = rowPosition - visibleRows + 1
	}
	if model.Grid.Selected.Col < model.Grid.ScrollCol {
		model.Grid.ScrollCol = model.Grid.Selected.Col
	}
	sheet := model.activeSheet()
	if sheet == nil {
		return
	}
	availableWidth := max(model.Grid.Width-model.Grid.RowHeaderWidth, 1)
	usedWidth := 0
	for col := model.Grid.ScrollCol; col < model.Grid.Selected.Col; col++ {
		usedWidth += model.Grid.columnWidth(col, sheet)
	}
	if usedWidth+model.Grid.columnWidth(model.Grid.Selected.Col, sheet) > availableWidth {
		model.Grid.ScrollCol = model.Grid.Selected.Col
	}
}

func (model *AppModel) layout() {
	if model.width <= 0 {
		model.width = 80
	}
	if model.height <= 0 {
		model.height = 24
	}
	model.editorInput.Width = max(model.width-16, 1)
	model.home.width, model.home.height = model.width, model.height
	originY := model.topLines()
	gridHeight := max(model.height-originY-2, 1)
	model.Grid.SetBounds(0, originY, model.width, gridHeight)
	if model.session != nil && model.session.Workbook != nil && len(model.session.Workbook.Sheets) > 0 {
		sheet := model.activeSheet()
		if model.visibleSheet != sheet.Name || model.filtersDirty {
			filters := model.session.Filters[sheet.Name]
			if len(filters) > 0 {
				model.Grid.VisibleRowIndexes = workbook.VisibleRows(sheet, filters)
			} else {
				model.Grid.VisibleRowIndexes = nil
			}
			model.visibleSheet = sheet.Name
			model.filtersDirty = false
		}
		model.ensureSelectedVisible()
	} else {
		model.Grid.VisibleRowIndexes = nil
		model.visibleSheet = ""
	}
}

func (model *AppModel) topLines() int {
	lines := 1 // formula/name bar
	if model.toolbarVisible {
		lines++
	}
	return lines
}

func (model *AppModel) activeSheet() *workbook.Sheet {
	if model == nil || model.session == nil || model.session.Workbook == nil || len(model.session.Workbook.Sheets) == 0 {
		return nil
	}
	index := min(max(model.session.Workbook.ActiveSheet, 0), len(model.session.Workbook.Sheets)-1)
	return &model.session.Workbook.Sheets[index]
}

func (model *AppModel) formulaBar() string {
	sheet := model.activeSheet()
	if sheet == nil {
		return "Name: A1  fx  "
	}
	cell := sheet.Cells[model.Grid.Selected]
	value := cell.Input
	if model.editing {
		value = model.editorInput.Value()
	}
	return fmt.Sprintf("%s!%s  fx  %s", sheet.Name, columnName(model.Grid.Selected.Col)+itoa(model.Grid.Selected.Row+1), value)
}

type sheetTabHit struct {
	StartX     int
	EndX       int
	SheetIndex int
}

type sheetTabsLayout struct {
	View           string
	Tabs           []sheetTabHit
	AddSheetStartX int
	AddSheetEndX   int
}

func (model *AppModel) sheetTabsLayout() sheetTabsLayout {
	layout := sheetTabsLayout{}
	if model.session == nil || model.session.Workbook == nil {
		return layout
	}
	var output strings.Builder
	position := 0
	for index, sheet := range model.session.Workbook.Sheets {
		label := " " + sheet.Name + " "
		layout.Tabs = append(layout.Tabs, sheetTabHit{
			StartX: position, EndX: position + lipgloss.Width(label), SheetIndex: index,
		})
		if index == model.session.Workbook.ActiveSheet {
			output.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6)).Render(label))
		} else {
			output.WriteString(label)
		}
		position += lipgloss.Width(label)
		if index < len(model.session.Workbook.Sheets)-1 {
			output.WriteString("  ")
			position += 2
		}
	}
	if len(model.session.Workbook.Sheets) > 0 {
		output.WriteString("  ")
		position += 2
		addSheetLabel := " + "
		layout.AddSheetStartX = position
		layout.AddSheetEndX = position + lipgloss.Width(addSheetLabel)
		output.WriteString(lipgloss.NewStyle().Foreground(ANSIColor(8)).Render(addSheetLabel))
	}
	layout.View = output.String()
	return layout
}

func (model *AppModel) sheetTabs() string {
	return model.sheetTabsLayout().View
}

func (model *AppModel) sheetTabsY() int {
	originY := model.topLines()
	gridHeight := max(model.height-originY-2, 1)
	return originY + model.Grid.renderedHeight(gridHeight)
}

func (model *AppModel) selectSheetAt(x int) {
	tabLayout := model.sheetTabsLayout()
	for _, tab := range tabLayout.Tabs {
		if x >= tab.StartX && x < tab.EndX {
			model.activateSheet(tab.SheetIndex)
			return
		}
	}
	if x >= tabLayout.AddSheetStartX && x < tabLayout.AddSheetEndX {
		model.runCommand("add-sheet")
	}
}

func (model *AppModel) changeSheet(delta int) {
	if model.session == nil || model.session.Workbook == nil || len(model.session.Workbook.Sheets) == 0 {
		return
	}
	count := len(model.session.Workbook.Sheets)
	index := (model.session.Workbook.ActiveSheet + delta + count) % count
	model.activateSheet(index)
}

func (model *AppModel) activateSheet(index int) {
	if model.session == nil || model.session.Workbook == nil || len(model.session.Workbook.Sheets) == 0 {
		return
	}
	model.session.Workbook.ActiveSheet = min(max(index, 0), len(model.session.Workbook.Sheets)-1)
	model.Grid.ScrollRow = 0
	model.Grid.ScrollCol = 0
	model.Grid.Selected = workbook.Coord{}
	model.Grid.Selection = workbook.CellRange{}
	model.Grid.ColumnWidths = make(map[int]int)
	model.visibleSheet = ""
	model.filtersDirty = true
	model.layout()
}

func (model *AppModel) selectedRangeName() string {
	if model == nil {
		return "A1"
	}
	selection := model.Grid.Selection
	if selection == (workbook.CellRange{}) && model.Grid.Selected != (workbook.Coord{}) {
		selection = workbook.CellRange{Start: model.Grid.Selected, End: model.Grid.Selected}
	}
	startRow := min(selection.Start.Row, selection.End.Row)
	endRow := max(selection.Start.Row, selection.End.Row)
	startCol := min(selection.Start.Col, selection.End.Col)
	endCol := max(selection.Start.Col, selection.End.Col)
	start := columnName(startCol) + itoa(startRow+1)
	end := columnName(endCol) + itoa(endRow+1)
	if start == end {
		return start
	}
	return start + ":" + end
}

func (model *AppModel) defaultFormulaTargetName() string {
	target := model.Grid.Selected
	startCol, startRow, endCol, endRow, err := parseA1Range(model.selectedRangeName())
	if err == nil && target.Col >= startCol && target.Col <= endCol && target.Row >= startRow && target.Row <= endRow {
		target = workbook.Coord{Row: endRow + 1, Col: startCol}
	}
	return columnName(target.Col) + itoa(target.Row+1)
}

func (model *AppModel) handleFormulaPickerKey(message tea.KeyMsg) {
	submitted, cancelled := model.FunctionPicker.HandleKey(message)
	if cancelled {
		model.FunctionPicker = formulaPickerState{}
		return
	}
	if !submitted {
		return
	}
	formula, err := model.FunctionPicker.Formula()
	if err != nil {
		model.status = err.Error()
		return
	}
	targetCol, targetRow, err := parseA1Cell(model.FunctionPicker.TargetInput.Value())
	if err != nil {
		model.status = "Invalid result cell: " + err.Error()
		return
	}
	if startCol, startRow, endCol, endRow, rangeErr := parseA1Range(model.FunctionPicker.RangeInput.Value()); rangeErr == nil &&
		targetCol >= startCol && targetCol <= endCol && targetRow >= startRow && targetRow <= endRow {
		model.status = "Result cell must be outside the input range"
		return
	}
	sheet := model.activeSheet()
	if sheet == nil {
		model.FunctionPicker = formulaPickerState{}
		return
	}
	if err := workbook.SetCell(model.session, workbook.CellAddress{Sheet: sheet.Name, Row: targetRow, Col: targetCol}, formula); err != nil {
		model.status = err.Error()
		return
	}
	model.Grid.Selected = workbook.Coord{Row: targetRow, Col: targetCol}
	model.Grid.Selection = workbook.CellRange{}
	model.FunctionPicker = formulaPickerState{}
	model.status = ""
	model.layout()
}

func (model *AppModel) inputOverlay() string {
	switch {
	case model.editing:
		return model.editorInput.View()
	case model.findMode:
		return "Find: " + model.findQuery + "_"
	case model.operation.Mode != "":
		switch model.operation.Mode {
		case "sort":
			return "Sort (a ascending, d descending, h no header): " + model.operation.Input + "_"
		case "filter":
			return "Allowed values separated by semicolons (empty clears filter): " + model.operation.Input + "_"
		case "merge-confirm":
			return "Merging discards cell values. Confirm? (y/n)"
		default:
			return ""
		}
	default:
		return ""
	}
}
func (model *AppModel) colorPickerView() string {
	var lines []string
	if model.ColorPicker.EditingInput {
		lines = append(lines, "Custom RGB color", "#"+model.ColorPicker.Input+"_", "Enter apply · Esc cancel")
	} else {
		lines = append(lines, "Terminal theme")
		options := model.Theme.Options()
		for index, option := range options {
			prefix := "  "
			if index == model.ColorPicker.Selected {
				prefix = "› "
			}
			swatch := "□"
			if !option.Default {
				color := lipgloss.TerminalColor(ANSIColor(option.ANSI))
				if option.Custom {
					color = TerminalColor(option.Color, supportsTrueColor())
				}
				swatch = lipgloss.NewStyle().Foreground(color).Render("■")
			}
			lines = append(lines, prefix+swatch+" "+option.Name)
		}
		prefix := "  "
		if model.ColorPicker.Selected == len(options) {
			prefix = "› "
		}
		lines = append(lines, prefix+"＋ Add custom RGB…", "↑/↓ navigate · Enter select · Esc close")
	}
	popup := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ANSIColor(6)).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
	return popup
}

func (model *AppModel) setStatus(err error) {
	if err != nil {
		model.status = err.Error()
	} else {
		model.status = ""
	}
}

func supportsTrueColor() bool {
	profile := strings.ToLower(os.Getenv("COLORTERM"))
	if profile == "truecolor" || profile == "24bit" {
		return true
	}
	return strings.Contains(strings.ToLower(os.Getenv("TERM")), "direct")
}
