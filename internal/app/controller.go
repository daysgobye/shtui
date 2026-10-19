package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"omasheet/internal/ui"
	"omasheet/internal/workbook"
)

type loadCompleteMsg struct {
	path    string
	session *workbook.Session
	err     error
}

type saveCompleteMsg struct {
	path string
	err  error
}

type Controller struct {
	initialPath     string
	path            string
	session         *workbook.Session
	view            ui.AppModel
	loading         bool
	loadingPath     string
	saving          bool
	picker          filepicker.Model
	pickerOpen      bool
	pickerMode      string
	pickerFocused   bool
	saveName        textinput.Model
	keys            ui.KeyMap
	confirmCSVLoss  bool
	pendingSavePath string
	status          string
	width           int
	height          int
}

func NewController(path string) *Controller {
	picker := filepicker.New()
	if directory, err := os.Getwd(); err == nil {
		picker.CurrentDirectory = directory
	}
	picker.AllowedTypes = []string{".csv", ".xlsx"}
	picker.DirAllowed = true
	picker.FileAllowed = true
	picker.ShowPermissions = false
	picker.ShowSize = false
	picker.AutoHeight = false
	picker.SetHeight(12)
	configurePickerTheme(&picker)

	filename := textinput.New()
	filename.Prompt = "Filename "
	filename.Placeholder = "workbook.csv"
	filename.Width = 52
	filename.PromptStyle = lipgloss.NewStyle().Foreground(ui.ANSIColor(6)).Bold(true)
	filename.TextStyle = lipgloss.NewStyle().Foreground(ui.ANSIColor(7))

	return &Controller{
		initialPath: path,
		view:        ui.NewModel(nil),
		picker:      picker,
		saveName:    filename,
		keys:        ui.DefaultKeyMap(),
		width:       80,
		height:      24,
	}
}
func configurePickerTheme(picker *filepicker.Model) {
	styles := filepicker.DefaultStyles()
	styles.Cursor = lipgloss.NewStyle().Foreground(ui.ANSIColor(6)).Bold(true)
	styles.Directory = lipgloss.NewStyle().Foreground(ui.ANSIColor(6)).Bold(true)
	styles.File = lipgloss.NewStyle().Foreground(ui.ANSIColor(7))
	styles.DisabledFile = lipgloss.NewStyle().Foreground(ui.ANSIColor(8))
	styles.Symlink = lipgloss.NewStyle().Foreground(ui.ANSIColor(5))
	styles.Selected = lipgloss.NewStyle().Foreground(ui.ANSIColor(7)).Reverse(true)
	styles.DisabledSelected = lipgloss.NewStyle().Foreground(ui.ANSIColor(8)).Reverse(true)
	styles.Permission = lipgloss.NewStyle().Foreground(ui.ANSIColor(8))
	styles.FileSize = lipgloss.NewStyle().Foreground(ui.ANSIColor(8))
	styles.EmptyDirectory = lipgloss.NewStyle().Foreground(ui.ANSIColor(8))
	picker.Styles = styles
}

func (controller *Controller) Init() tea.Cmd {
	if controller == nil || controller.initialPath == "" {
		return nil
	}
	controller.loading = true
	controller.loadingPath = controller.initialPath
	return controller.loadCmd(controller.initialPath)
}

func (controller *Controller) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if controller == nil {
		return controller, nil
	}
	if keyMessage, ok := message.(tea.KeyMsg); ok && keyMessage.Type == tea.KeyCtrlC {
		return controller, tea.Quit
	}
	if size, ok := message.(tea.WindowSizeMsg); ok {
		controller.width = size.Width
		controller.height = size.Height
		_, viewCommand := controller.view.Update(size)
		controller.picker.SetHeight(max(size.Height-8, 1))
		if controller.pickerOpen {
			var pickerCommand tea.Cmd
			controller.picker, pickerCommand = controller.picker.Update(size)
			return controller, tea.Batch(viewCommand, pickerCommand)
		}
		return controller, viewCommand
	}
	if controller.pickerOpen {
		return controller, controller.updatePicker(message)
	}

	switch message := message.(type) {
	case loadCompleteMsg:
		controller.loading = false
		controller.loadingPath = ""
		if message.err != nil {
			controller.status = fmt.Sprintf("Open %q failed: %v", message.path, message.err)
			return controller, nil
		}
		old := controller.session
		controller.session = message.session
		controller.path = message.path
		controller.view = controller.newView(message.session)
		controller.status = ""
		if old != nil && old != message.session {
			_ = old.Close()
		}
		return controller, nil
	case saveCompleteMsg:
		controller.saving = false
		if message.err != nil {
			controller.status = fmt.Sprintf("Save %q failed: %v", message.path, message.err)
		} else {
			controller.path = message.path
			controller.status = fmt.Sprintf("Saved %s", message.path)
		}
		return controller, nil
	case tea.KeyMsg:
		if controller.loading || controller.saving {
			return controller, nil
		}
		if controller.confirmCSVLoss {
			return controller, controller.handleCSVConfirmation(message)
		}
		if controller.keys.Matches(message, controller.keys.OpenFile) {
			return controller, controller.beginPicker("open")
		}
		if controller.keys.Matches(message, controller.keys.Save) {
			return controller, controller.requestSave()
		}
		if controller.session == nil && !controller.view.Palette.Open && message.Type == tea.KeyRunes && len(message.Runes) == 1 {
			switch strings.ToLower(string(message.Runes)) {
			case "o":
				return controller, controller.beginPicker("open")
			case "n":
				controller.newWorkbook(workbook.CSV)
				return controller, nil
			case "x":
				controller.newWorkbook(workbook.XLSX)
				return controller, nil
			}
		}
		_, command := controller.view.Update(message)
		action := controller.view.TakeCommand()
		return controller, controller.handleAction(action, command)
	default:
		_, command := controller.view.Update(message)
		return controller, command
	}
}

func (controller *Controller) View() string {
	if controller == nil {
		return ""
	}
	if controller.loading && controller.session == nil {
		return "OmaSheet\nLoading " + controller.loadingPath + "…"
	}

	var content string
	if controller.session == nil {
		content = "OmaSheet\nOpen or create a workbook\n\nO  Open file\nN  New CSV\nX  New XLSX\nCtrl+P  Command palette\nCtrl+C  Quit"
	} else {
		content = controller.view.View()
		switch {
		case controller.confirmCSVLoss:
			content += "\nCSV warning: " + strings.Join(controller.csvLossWarnings(), "; ") + ". Press Enter or Y to continue, Esc to cancel."
		case controller.loading:
			content += "\nLoading " + controller.loadingPath + "…"
		case controller.saving:
			content += "\nSaving " + controller.pendingSavePath + "…"
		case controller.status != "":
			content += "\n" + controller.status
		}
	}
	if controller.status != "" && controller.session == nil {
		content += "\n" + controller.status
	}
	if controller.pickerOpen {
		return ui.ComposeOverlay(content, controller.pickerView(), controller.width, controller.height)
	}
	if controller.session == nil && controller.view.Palette.Open {
		return ui.ComposeOverlay(content, controller.view.Palette.View(controller.width), controller.width, controller.height)
	}
	return content
}

func (controller *Controller) loadCmd(path string) tea.Cmd {
	return func() tea.Msg {
		session, err := workbook.OpenFile(path)
		return loadCompleteMsg{path: path, session: session, err: err}
	}
}

func (controller *Controller) saveCmd(path string) tea.Cmd {
	session := controller.session
	return func() tea.Msg {
		err := workbook.SaveFile(session, path)
		return saveCompleteMsg{path: path, err: err}
	}
}

func (controller *Controller) handleAction(action string, command tea.Cmd) tea.Cmd {
	switch action {
	case "open-file":
		return controller.beginPicker("open")
	case "new-csv":
		controller.newWorkbook(workbook.CSV)
	case "new-xlsx":
		controller.newWorkbook(workbook.XLSX)
	case "save":
		return controller.requestSave()
	case "save-as":
		return controller.beginPicker("save")
	}
	return command
}

func (controller *Controller) requestSave() tea.Cmd {
	if controller.session == nil {
		controller.status = "Open or create a workbook before saving"
		return nil
	}
	if warnings := controller.csvLossWarnings(); len(warnings) > 0 {
		controller.pendingSavePath = controller.session.Path
		controller.confirmCSVLoss = true
		controller.status = ""
		return nil
	}
	if controller.session.Path == "" {
		return controller.beginPicker("save")
	}
	return controller.startSave(controller.session.Path)
}

func (controller *Controller) beginPicker(mode string) tea.Cmd {
	controller.pickerOpen = true
	controller.pickerMode = mode
	controller.pickerFocused = false
	controller.status = ""
	controller.picker.Path = ""
	controller.picker.AllowedTypes = []string{".csv", ".xlsx"}
	controller.picker.DirAllowed = true
	controller.picker.FileAllowed = mode == "open"
	controller.picker.SetHeight(max(controller.height-8, 1))
	if mode == "save" && controller.path != "" {
		controller.picker.CurrentDirectory = filepath.Dir(controller.path)
	}
	if mode == "save" {
		controller.saveName.Reset()
		focusCommand := controller.saveName.Focus()
		return tea.Batch(controller.picker.Init(), focusCommand)
	}
	controller.saveName.Blur()
	return controller.picker.Init()
}

func (controller *Controller) updatePicker(message tea.Msg) tea.Cmd {
	if keyMessage, ok := message.(tea.KeyMsg); ok {
		if keyMessage.Type == tea.KeyEsc {
			controller.pickerOpen = false
			controller.pickerFocused = false
			controller.saveName.Blur()
			controller.status = ""
			return nil
		}
		if controller.pickerMode == "save" && !controller.pickerFocused {
			switch keyMessage.Type {
			case tea.KeyTab:
				controller.pickerFocused = true
				controller.saveName.Blur()
				return nil
			case tea.KeyEnter:
				name := strings.TrimSpace(controller.saveName.Value())
				if name == "" {
					controller.status = "Enter a file name"
					return nil
				}
				path := filepath.Join(controller.picker.CurrentDirectory, name)
				controller.pickerOpen = false
				controller.saveName.Blur()
				return controller.prepareSave(path)
			default:
				var command tea.Cmd
				controller.saveName, command = controller.saveName.Update(keyMessage)
				return command
			}
		}
		if controller.pickerMode == "save" && controller.pickerFocused && keyMessage.Type == tea.KeyTab {
			controller.pickerFocused = false
			return controller.saveName.Focus()
		}
		currentDirectory := controller.picker.CurrentDirectory
		var command tea.Cmd
		controller.picker, command = controller.picker.Update(keyMessage)
		if controller.picker.CurrentDirectory != currentDirectory {
			return command
		}
		if selected, path := controller.picker.DidSelectFile(keyMessage); selected {
			controller.pickerOpen = false
			return controller.startLoad(path)
		}
		if disabled, _ := controller.picker.DidSelectDisabledFile(keyMessage); disabled {
			controller.status = "Unsupported file; choose a supported .csv or .xlsx workbook"
		}
		return command
	}
	var command tea.Cmd
	controller.picker, command = controller.picker.Update(message)
	return command
}

func (controller *Controller) pickerView() string {
	title := "Open workbook"
	controls := "↑/↓ navigate · Enter open/select · Esc close"
	rows := []string{title, "Esc close · " + controller.picker.CurrentDirectory, ""}
	if controller.pickerMode == "save" {
		title = "Save as"
		controls = "Tab browse folders · Enter save · Esc cancel"
		rows[0] = title
		rows = append(rows, controller.saveName.View(), "")
		if controller.pickerFocused {
			rows[2] = "Choose a folder, then Tab to enter a filename"
		}
	}
	rows = append(rows, strings.TrimRight(controller.picker.View(), "\\n"), controls)
	if controller.status != "" {
		rows = append(rows, controller.status)
	}
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ANSIColor(6)).
		Padding(0, 1).
		Width(max(controller.width-4, 1)).
		Render(content)
}

func (controller *Controller) startLoad(path string) tea.Cmd {
	if controller.session != nil && controller.session.Workbook.Dirty {
		controller.status = "Save the current workbook before opening another file"
		return nil
	}
	controller.loading = true
	controller.loadingPath = path
	controller.status = ""
	return controller.loadCmd(path)
}

func (controller *Controller) prepareSave(path string) tea.Cmd {
	if controller.session == nil {
		controller.status = "Open or create a workbook before saving"
		return nil
	}
	if warnings := controller.csvLossWarnings(); len(warnings) > 0 {
		controller.pendingSavePath = path
		controller.confirmCSVLoss = true
		controller.status = ""
		return nil
	}
	return controller.startSave(path)
}

func (controller *Controller) startSave(path string) tea.Cmd {
	controller.saving = true
	controller.pendingSavePath = path
	controller.status = ""
	return controller.saveCmd(path)
}

func (controller *Controller) handleCSVConfirmation(message tea.KeyMsg) tea.Cmd {
	switch message.Type {
	case tea.KeyEsc:
		controller.confirmCSVLoss = false
		controller.pendingSavePath = ""
		controller.status = "Save cancelled"
	case tea.KeyEnter:
		return controller.confirmCSVSave()
	case tea.KeyRunes:
		if strings.EqualFold(string(message.Runes), "y") {
			return controller.confirmCSVSave()
		}
		if strings.EqualFold(string(message.Runes), "n") {
			controller.confirmCSVLoss = false
			controller.pendingSavePath = ""
		}
	}
	return nil
}

func (controller *Controller) confirmCSVSave() tea.Cmd {
	controller.confirmCSVLoss = false
	path := controller.pendingSavePath
	controller.pendingSavePath = ""
	if path == "" {
		return controller.beginPicker("save")
	}
	return controller.startSave(path)
}

func (controller *Controller) csvLossWarnings() []string {
	if controller.session == nil || controller.session.Format != workbook.CSV || controller.session.Workbook == nil {
		return nil
	}
	var warnings []string
	if len(controller.session.Workbook.Sheets) > 1 {
		warnings = append(warnings, "other sheets would be lost")
	}
	formatting := false
	for _, sheet := range controller.session.Workbook.Sheets {
		if len(sheet.ColumnWidths) > 0 || len(sheet.Merges) > 0 || len(sheet.ConditionalRules) > 0 {
			formatting = true
			break
		}
		for _, cell := range sheet.Cells {
			if cell.Style != (workbook.CellStyle{}) {
				formatting = true
				break
			}
		}
		if formatting {
			break
		}
	}
	if formatting {
		warnings = append(warnings, "formatting would be lost")
	}
	return warnings
}

func (controller *Controller) newWorkbook(format workbook.Format) {
	if controller.session != nil && controller.session.Workbook.Dirty {
		controller.status = "Save the current workbook before creating another"
		return
	}
	session, err := workbook.NewWorkbook(format)
	if err != nil {
		controller.status = err.Error()
		return
	}
	old := controller.session
	controller.session = session
	controller.path = ""
	controller.view = controller.newView(session)
	controller.status = "New " + string(format) + " workbook"
	if old != nil {
		_ = old.Close()
	}
}
func (controller *Controller) newView(session *workbook.Session) ui.AppModel {
	view := ui.NewModel(session)
	view.Update(tea.WindowSizeMsg{Width: controller.width, Height: controller.height})
	return view
}

func Run(ctx context.Context, path string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	program := tea.NewProgram(NewController(path), tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithContext(ctx))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run terminal app: %w", err)
	}
	return nil
}
