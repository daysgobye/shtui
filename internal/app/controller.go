package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	pathPrompt      bool
	pathMode        string
	pathInput       string
	confirmCSVLoss  bool
	pendingSavePath string
	status          string
	width           int
	height          int
}

func NewController(path string) *Controller {
	return &Controller{initialPath: path, view: ui.NewModel(nil), width: 80, height: 24}
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
		if message.Type == tea.KeyCtrlC {
			return controller, tea.Quit
		}
		if controller.loading || controller.saving {
			return controller, nil
		}
		if controller.confirmCSVLoss {
			return controller, controller.handleCSVConfirmation(message)
		}
		if controller.pathPrompt {
			return controller, controller.handlePathPrompt(message)
		}
		if message.Type == tea.KeyCtrlO {
			controller.beginPathPrompt("open")
			return controller, nil
		}
		if message.Type == tea.KeyCtrlS {
			return controller, controller.requestSave()
		}
		if controller.session == nil && !controller.view.Palette.Open && message.Type == tea.KeyRunes && len(message.Runes) == 1 {
			switch strings.ToLower(string(message.Runes)) {
			case "o":
				controller.beginPathPrompt("open")
				return controller, nil
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
	case tea.WindowSizeMsg:
		controller.width = message.Width
		controller.height = message.Height
		_, command := controller.view.Update(message)
		return controller, command
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
	if controller.session == nil {
		if controller.view.Palette.Open {
			palette := controller.view.Palette.View(controller.width)
			if controller.status != "" {
				palette += "\n" + controller.status
			}
			return palette
		}
		home := "OmaSheet\nOpen or create a workbook\n\nO  Open file\nN  New CSV\nX  New XLSX\nCtrl+P  Command palette\nCtrl+C  Quit"
		if controller.pathPrompt {
			home += "\n" + controller.pathPromptView()
		}
		if controller.status != "" {
			home += "\n" + controller.status
		}
		return home
	}
	content := controller.view.View()
	switch {
	case controller.pathPrompt:
		prompt := controller.pathPromptView()
		content += "\n" + prompt
	case controller.confirmCSVLoss:
		content += "\nCSV warning: " + strings.Join(controller.csvLossWarnings(), "; ") + ". Press Enter or Y to continue, Esc to cancel."
	case controller.loading:
		content += "\nLoading " + controller.loadingPath + "…"
	case controller.saving:
		content += "\nSaving " + controller.pendingSavePath + "…"
	case controller.status != "":
		content += "\n" + controller.status
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
		controller.beginPathPrompt("open")
	case "new-csv":
		controller.newWorkbook(workbook.CSV)
	case "new-xlsx":
		controller.newWorkbook(workbook.XLSX)
	case "save":
		return controller.requestSave()
	case "save-as":
		controller.beginPathPrompt("save")
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
		controller.beginPathPrompt("save")
		return nil
	}
	return controller.startSave(controller.session.Path)
}

func (controller *Controller) beginPathPrompt(mode string) {
	controller.pathPrompt = true
	controller.pathMode = mode
	controller.pathInput = ""
	controller.status = ""
}

func (controller *Controller) pathPromptView() string {
	label := "Open file"
	if controller.pathMode == "save" {
		label = "Save as"
	}
	return label + " path: " + controller.pathInput + "_\nEnter to continue · Esc to cancel"
}

func (controller *Controller) handlePathPrompt(message tea.KeyMsg) tea.Cmd {
	switch message.Type {
	case tea.KeyEsc:
		controller.pathPrompt = false
		controller.pathInput = ""
	case tea.KeyEnter:
		path := strings.TrimSpace(controller.pathInput)
		if path == "" {
			controller.status = "Enter a file path"
			return nil
		}
		controller.pathPrompt = false
		controller.pathInput = ""
		if controller.pathMode == "open" {
			return controller.startLoad(path)
		}
		return controller.prepareSave(path)
	case tea.KeyBackspace:
		runes := []rune(controller.pathInput)
		if len(runes) > 0 {
			controller.pathInput = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		controller.pathInput += string(message.Runes)
	}
	return nil
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
		controller.beginPathPrompt("save")
		return nil
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
