package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type homeActionItem struct {
	action string
	label  string
}

func (item homeActionItem) FilterValue() string { return item.label }
func (item homeActionItem) Title() string       { return item.label }
func (item homeActionItem) Description() string { return "" }

type homeScreen struct {
	list   list.Model
	width  int
	height int
}

func newHomeScreen() homeScreen {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Foreground(ANSIColor(6)).Bold(true)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().Foreground(ANSIColor(7))
	items := []list.Item{
		homeActionItem{action: "open-file", label: "Open workbook"},
		homeActionItem{action: "new-csv", label: "New CSV workbook"},
		homeActionItem{action: "new-xlsx", label: "New XLSX workbook"},
	}
	menu := list.New(items, delegate, 48, 5)
	menu.Title = "Start a workbook"
	menu.SetShowStatusBar(false)
	menu.SetShowPagination(false)
	menu.SetShowHelp(false)
	menu.SetShowTitle(true)
	menu.SetShowFilter(false)
	menu.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(ANSIColor(6))
	menu.DisableQuitKeybindings()
	return homeScreen{list: menu, width: 80, height: 24}
}

func (screen *homeScreen) View(width, height int) string {
	if screen == nil || width <= 0 || height <= 0 {
		return ""
	}
	return screen.layout(width, height).view
}

func (screen *homeScreen) HandleKey(message tea.KeyMsg) string {
	if screen == nil {
		return ""
	}
	if message.Type == tea.KeyEnter {
		return screen.selectedAction()
	}
	updated, _ := screen.list.Update(message)
	screen.list = updated
	return ""
}

func (screen *homeScreen) HandleMouse(message tea.MouseMsg) string {
	if screen == nil {
		return ""
	}
	event := tea.MouseEvent(message)
	if event.Button == tea.MouseButtonWheelUp || event.Button == tea.MouseButtonWheelDown {
		updated, _ := screen.list.Update(message)
		screen.list = updated
		return ""
	}
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return ""
	}
	layout := screen.layout(screen.width, screen.height)
	if event.X < layout.actionLeft || event.X >= layout.actionRight {
		return ""
	}
	for index, row := range layout.actionRows {
		if event.Y == row {
			screen.list.Select(index)
			return screen.selectedAction()
		}
	}
	return ""
}

type homeScreenLayout struct {
	view        string
	actionLeft  int
	actionRight int
	actionRows  []int
}

func (screen *homeScreen) layout(width, height int) homeScreenLayout {
	if screen == nil || width <= 0 || height <= 0 {
		return homeScreenLayout{}
	}
	screen.width, screen.height = width, height
	compact := height < 19 || width < 64
	panelWidth := min(max(width-4, 1), 52)
	listWidth := max(panelWidth-4, 1)
	screen.list.SetSize(listWidth, 5)
	listView := screen.list.View()
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ANSIColor(8)).
		Padding(0, 1).
		Width(listWidth).
		Render(listView)
	wordmark := SHTUIWordmarkText()
	if !compact {
		wordmark = SHTUIWordmark()
	}
	tagline := lipgloss.NewStyle().Foreground(ANSIColor(8)).Render("A calmer way to work with spreadsheets")
	wideHints := lipgloss.NewStyle().Foreground(ANSIColor(8)).Render("↑/↓ select  ·  Enter choose  ·  Ctrl+P commands  ·  Ctrl+C quit")
	compactHints := lipgloss.NewStyle().Foreground(ANSIColor(8)).Render("↑/↓ select · Enter choose\nCtrl+P commands · Ctrl+C quit")
	sections := []string{wordmark}
	prefixHeight := lipgloss.Height(wordmark)
	if !compact {
		sections = append(sections, tagline, "")
		prefixHeight += lipgloss.Height(tagline) + 1
	}
	sections = append(sections, panel)
	if !compact {
		sections = append(sections, "", wideHints)
	} else if height >= 10 {
		sections = append(sections, compactHints)
	} else if height == 9 {
		sections = append(sections, lipgloss.NewStyle().Foreground(ANSIColor(8)).Render("Enter choose · Ctrl+P"))
	}
	content := lipgloss.JoinVertical(lipgloss.Center, sections...)
	contentWidth := lipgloss.Width(content)
	contentHeight := lipgloss.Height(content)
	top := max((height-contentHeight)/2, 0)
	panelWithinContent := max((contentWidth-panelWidth)/2, 0)
	contentLeft := max((width-contentWidth)/2, 0)
	panelTop := top + prefixHeight
	listLines := strings.Split(ansi.Strip(listView), "\n")
	actionRows := make([]int, len(screen.list.Items()))
	for itemIndex, rawItem := range screen.list.Items() {
		item, ok := rawItem.(homeActionItem)
		if !ok {
			continue
		}
		for lineIndex, line := range listLines {
			if strings.Contains(line, item.label) {
				actionRows[itemIndex] = panelTop + 1 + lineIndex
				break
			}
		}
	}
	return homeScreenLayout{
		view:        lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content),
		actionLeft:  contentLeft + panelWithinContent + 2,
		actionRight: contentLeft + panelWithinContent + panelWidth - 2,
		actionRows:  actionRows,
	}
}

func (screen *homeScreen) selectedAction() string {
	item, ok := screen.list.SelectedItem().(homeActionItem)
	if !ok {
		return ""
	}
	return item.action
}

