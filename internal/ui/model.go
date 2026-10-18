package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"omasheet/internal/workbook"
)

type colorTarget string

const (
	fillColorTarget colorTarget = "fill"
	fontColorTarget colorTarget = "font"
)

type colorPickerState struct {
	Open         bool
	Target       colorTarget
	Selected     int
	EditingInput bool
	Input        string
}

type AppModel struct {
	session        *workbook.Session
	Grid           GridViewport
	Palette        CommandPalette
	Theme          ThemePalette
	ColorPicker    colorPickerState
	toolbarVisible bool
	width          int
	height         int
	editing        bool
	editor         string
	findMode       bool
	findQuery      string
	themeQuery     func(uint8, time.Duration) (RGB, bool)
	status         string
}

func NewModel(session *workbook.Session) AppModel {
	return AppModel{
		session:        session,
		Grid:           NewGridViewport(),
		Theme:          NewThemePalette(),
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
		content := "OmaSheet\nOpen or create a CSV or XLSX workbook.\n\nUse: omasheet <path>\nCtrl+P  Command palette"
		if model.toolbarVisible {
			content = model.toolbarView() + "\n" + content
		}
		if model.Palette.Open {
			return model.Palette.View(model.width)
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
	output.WriteString(statusStyle.Render(status))
	if model.editing || model.findMode || model.ColorPicker.Open {
		output.WriteByte('\n')
		output.WriteString(model.inputOverlay())
	}
	if model.Palette.Open {
		return model.Palette.View(model.width)
	}
	return output.String()
}

func (model *AppModel) handleKey(message tea.KeyMsg) {
	if message.Type == tea.KeyCtrlP {
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
	if model.findMode {
		model.handleFindKey(message)
		return
	}
	if model.editing {
		model.handleEditKey(message)
		return
	}
	if model.session == nil || model.session.Workbook == nil {
		return
	}
	switch message.Type {
	case tea.KeyCtrlZ:
		model.setStatus(workbook.Undo(model.session))
	case tea.KeyCtrlY:
		model.setStatus(workbook.Redo(model.session))
	case tea.KeyEnter:
		cell := model.activeSheet().Cells[model.Grid.Selected]
		model.editor = cell.Input
		model.editing = true
	case tea.KeyUp:
		model.moveSelection(-1, 0)
	case tea.KeyDown:
		model.moveSelection(1, 0)
	case tea.KeyLeft:
		model.moveSelection(0, -1)
	case tea.KeyRight:
		model.moveSelection(0, 1)
	case tea.KeyRunes:
		if len(message.Runes) > 0 {
			model.editor = string(message.Runes)
			model.editing = true
		}
	}
}

func (model *AppModel) handleEditKey(message tea.KeyMsg) {
	switch message.Type {
	case tea.KeyEsc:
		model.editing = false
		model.editor = ""
	case tea.KeyEnter:
		if model.session != nil {
			address := workbook.CellAddress{Sheet: model.activeSheet().Name, Row: model.Grid.Selected.Row, Col: model.Grid.Selected.Col}
			model.setStatus(workbook.SetCell(model.session, address, model.editor))
		}
		model.editing = false
		model.editor = ""
	case tea.KeyBackspace:
		runes := []rune(model.editor)
		if len(runes) > 0 {
			model.editor = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		model.editor += string(message.Runes)
	}
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

func (model *AppModel) handleMouse(message tea.MouseMsg) {
	event := tea.MouseEvent(message)
	if model.ColorPicker.Open || model.Palette.Open {
		return
	}
	if model.toolbarVisible && event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft && event.Y == 0 {
		model.runCommand(toolbarActionAt(event.X))
		return
	}
	if model.session == nil || model.session.Workbook == nil {
		return
	}
	if event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft && event.Y == model.height-2 {
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
	case "fill-color":
		model.openColorPicker(fillColorTarget)
	case "font-color":
		model.openColorPicker(fontColorTarget)
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

func (model *AppModel) openColorPicker(target colorTarget) {
	model.ColorPicker = colorPickerState{Open: true, Target: target}
}

func (model *AppModel) applyColor(color RGB) {
	if model.session == nil || model.session.Workbook == nil {
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
	selected.Row = max(0, selected.Row+rowDelta)
	selected.Col = max(0, selected.Col+colDelta)
	model.selectCell(selected)
}

func (model *AppModel) selectCell(coord workbook.Coord) {
	model.Grid.Selected = coord
	model.Grid.Selection = workbook.CellRange{Start: coord, End: coord}
	model.ensureSelectedVisible()
}

func (model *AppModel) ensureSelectedVisible() {
	if model.Grid.Selected.Row < model.Grid.ScrollRow {
		model.Grid.ScrollRow = model.Grid.Selected.Row
	}
	visibleRows := max(model.Grid.Height-model.Grid.HeaderHeight, 1)
	if model.Grid.Selected.Row >= model.Grid.ScrollRow+visibleRows {
		model.Grid.ScrollRow = model.Grid.Selected.Row - visibleRows + 1
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
	originY := model.topLines()
	gridHeight := max(model.height-originY-2, 1)
	model.Grid.SetBounds(0, originY, model.width, gridHeight)
	if model.session != nil && model.session.Workbook != nil && len(model.session.Workbook.Sheets) > 0 {
		model.ensureSelectedVisible()
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
		value = model.editor
	}
	return fmt.Sprintf("%s!%s  fx  %s", sheet.Name, columnName(model.Grid.Selected.Col)+itoa(model.Grid.Selected.Row+1), value)
}

func (model *AppModel) sheetTabs() string {
	if model.session == nil || model.session.Workbook == nil {
		return ""
	}
	var tabs []string
	for index, sheet := range model.session.Workbook.Sheets {
		label := " " + sheet.Name + " "
		if index == model.session.Workbook.ActiveSheet {
			label = lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6)).Render(label)
		}
		tabs = append(tabs, label)
	}
	return strings.Join(tabs, "  ")
}

func (model *AppModel) selectSheetAt(x int) {
	position := 0
	for index, sheet := range model.session.Workbook.Sheets {
		label := " " + sheet.Name + " "
		width := lipgloss.Width(label)
		if x >= position && x < position+width {
			model.session.Workbook.ActiveSheet = index
			model.Grid.ScrollRow = 0
			model.Grid.ScrollCol = 0
			model.Grid.Selected = workbook.Coord{}
			model.Grid.Selection = workbook.CellRange{}
			model.Grid.ColumnWidths = make(map[int]int)
			return
		}
		position += width + 2
	}
}

func (model *AppModel) inputOverlay() string {
	switch {
	case model.editing:
		return "Edit: " + model.editor
	case model.findMode:
		return "Find: " + model.findQuery + "_"
	case model.ColorPicker.Open:
		if model.ColorPicker.EditingInput {
			return "Custom RGB (#RRGGBB): " + model.ColorPicker.Input + "_"
		}
		var lines []string
		for index, option := range model.Theme.Options() {
			prefix := "  "
			if index == model.ColorPicker.Selected {
				prefix = "> "
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
		if model.ColorPicker.Selected == len(model.Theme.Options()) {
			prefix = "> "
		}
		lines = append(lines, prefix+"Add custom RGB…")
		return strings.Join(lines, "\n")
	default:
		return ""
	}
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
