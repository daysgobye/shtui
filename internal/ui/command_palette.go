package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type paletteCommand struct {
	ID          string
	Name        string
	Description string
}

type paletteItem struct {
	command paletteCommand
}

func (item paletteItem) FilterValue() string {
	return item.command.Name + " " + item.command.Description + " " + item.command.ID
}
func (item paletteItem) Title() string {
	return item.command.Name
}

func (item paletteItem) Description() string {
	return item.command.Description
}

func newPaletteItems(keys KeyMap) []list.Item {
	commands := []paletteCommand{
		{ID: "copy", Name: "Copy"},
		{ID: "cut", Name: "Cut"},
		{ID: "paste", Name: "Paste"},
		{ID: "copy-row", Name: "Copy row"},
		{ID: "cut-row", Name: "Cut row"},
		{ID: "delete-row", Name: "Delete row"},
		{ID: "delete-column", Name: "Delete column"},
		{ID: "duplicate-sheet", Name: "Duplicate sheet"},
		{ID: "toggle-toolbar", Name: "Toggle toolbar"},
		{ID: "hide-toolbar", Name: "Hide toolbar"},
		{ID: "show-toolbar", Name: "Show toolbar"},
		{ID: "undo", Name: "Undo"},
		{ID: "redo", Name: "Redo"},
		{ID: "find", Name: "Find"},
		{ID: "fill-color", Name: "Fill color"},
		{ID: "font-color", Name: "Font color"},
		{ID: "border-color", Name: "Border color", Description: "Color all four edges of selected cells"},
		{ID: "border-none", Name: "Remove borders"},
		{ID: "insert-row", Name: "Insert row"},
		{ID: "insert-column", Name: "Insert column"},
		{ID: "open-file", Name: "Open file"},
		{ID: "new-csv", Name: "New CSV workbook"},
		{ID: "new-xlsx", Name: "New XLSX workbook"},
		{ID: "save", Name: "Save"},
		{ID: "save-as", Name: "Save as"},
		{ID: "sort", Name: "Sort by selected column"},
		{ID: "filter", Name: "Filter selected column"},
		{ID: "merge", Name: "Merge selected cells"},
		{ID: "unmerge", Name: "Unmerge selected cells"},
		{ID: "toggle-bold", Name: "Toggle bold"},
		{ID: "add-sheet", Name: "Add sheet"},
		{ID: "rename-sheet", Name: "Rename sheet", Description: "Rename the active sheet"},
		{ID: "conditional-format", Name: "Add conditional formatting"},
		{ID: "insert-function", Name: "Insert function"},
		{ID: "previous-sheet", Name: "Previous sheet"},
		{ID: "next-sheet", Name: "Next sheet"},
	}
	items := make([]list.Item, 0, len(commands))
	for _, command := range commands {
		shortcut := keys.Shortcut(command.ID)
		if shortcut != "" {
			command.Description = "Keyboard shortcut · " + shortcut
		}
		items = append(items, paletteItem{command: command})
	}
	return items
}

type CommandPalette struct {
	Open     bool
	Query    string
	Selected int
	list     list.Model
	help     help.Model
	keys     KeyMap
}

func NewCommandPalette(keys KeyMap) CommandPalette {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(ANSIColor(6)).Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(ANSIColor(7))
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(ANSIColor(7))
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(ANSIColor(8))
	items := newPaletteItems(keys)
	commands := list.New(items, delegate, 54, 12)
	commands.Title = "Commands · keyboard shortcuts"
	commands.SetShowStatusBar(false)
	commands.SetShowPagination(false)
	commands.SetShowHelp(false)
	commands.SetShowTitle(true)
	commands.SetShowFilter(true)
	commands.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6))
	commands.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(ANSIColor(6))
	commands.Styles.FilterCursor = lipgloss.NewStyle().Foreground(ANSIColor(6))
	return CommandPalette{list: commands, help: help.New(), keys: keys}
}

func (palette *CommandPalette) Show() {
	if palette == nil {
		return
	}
	palette.Open = true
	palette.Query = ""
	palette.Selected = 0
	palette.list.ResetFilter()
	palette.list.SetFilterText(palette.Query)
	palette.list.SetFilterState(list.Filtering)
	palette.list.Select(0)
}

func (palette *CommandPalette) HandleKey(message tea.KeyMsg) (string, bool) {
	if palette == nil || !palette.Open {
		return "", false
	}
	if message.Type == tea.KeyEsc {
		palette.Open = false
		return "", false
	}
	var commandID string
	if message.Type == tea.KeyEnter {
		if item, ok := palette.list.SelectedItem().(paletteItem); ok {
			commandID = item.command.ID
		}
	}
	previousQuery := palette.list.FilterValue()
	palette.list, _ = palette.list.Update(message)
	palette.Query = palette.list.FilterValue()
	palette.Selected = palette.list.Index()
	if palette.Query != previousQuery {
		palette.list.SetFilterText(palette.Query)
		palette.list.SetFilterState(list.Filtering)
	}
	if commandID != "" {
		palette.Open = false
		return commandID, true
	}
	return "", false
}

func (palette *CommandPalette) View(width int) string {
	if palette == nil || !palette.Open {
		return ""
	}
	frameWidth := min(64, max(width-4, 1))
	innerWidth := max(frameWidth-4, 1)
	frameHeight := min(14, max(5, width/3))
	palette.list.SetSize(innerWidth, max(frameHeight-4, 1))
	palette.help.Width = innerWidth
	content := lipgloss.JoinVertical(lipgloss.Left, palette.list.View(), palette.help.View(palette.keys))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ANSIColor(6)).
		Padding(0, 1).
		Width(innerWidth).
		Render(content)
}
