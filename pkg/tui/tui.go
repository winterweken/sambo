package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginTop(1).
			MarginBottom(1)

	menuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			PaddingLeft(2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true).
			PaddingLeft(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)
)

type screen int

const (
	screenMainMenu screen = iota
	screenSambaMenu
	screenSambaList
	screenSambaCreate
	screenSambaModify
	screenSambaRemove
	screenNFSMenu
	screenNFSList
	screenNFSCreate
	screenNFSModify
	screenNFSRemove
	screenUserMenu
	screenUserList
	screenUserAdd
	screenUserPassword
	screenUserRemove
)

type model struct {
	currentScreen screen
	cursor        int
	message       string
	messageType   string // "error", "success", ""

	// Form state
	form          *formModel
	inForm        bool

	// Data
	selectedItem  string
}

func newModel() model {
	return model{
		currentScreen: screenMainMenu,
		cursor:        0,
		inForm:        false,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Delegate to form if we're in a form
	if m.inForm && m.form != nil {
		var cmd tea.Cmd
		*m.form, cmd = m.form.Update(msg, &m)
		// Check if form closed
		if !m.inForm {
			m.form = nil
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.currentScreen == screenMainMenu {
				return m, tea.Quit
			}
			// Go back to main menu from submenus
			m.currentScreen = screenMainMenu
			m.cursor = 0
			m.message = ""
			return m, nil

		case "esc":
			// Go back one level
			switch m.currentScreen {
			case screenSambaMenu, screenNFSMenu, screenUserMenu:
				m.currentScreen = screenMainMenu
			case screenSambaList, screenSambaCreate, screenSambaModify, screenSambaRemove:
				m.currentScreen = screenSambaMenu
			case screenNFSList, screenNFSCreate, screenNFSModify, screenNFSRemove:
				m.currentScreen = screenNFSMenu
			case screenUserList, screenUserAdd, screenUserPassword, screenUserRemove:
				m.currentScreen = screenUserMenu
			}
			m.cursor = 0
			m.message = ""
			return m, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			maxCursor := m.getMaxCursor()
			if m.cursor < maxCursor {
				m.cursor++
			}

		case "enter":
			return m.handleEnter()
		}
	}

	return m, nil
}

func (m model) View() string {
	// Show form if we're in a form
	if m.inForm && m.form != nil {
		return m.form.View()
	}

	switch m.currentScreen {
	case screenMainMenu:
		return m.viewMainMenu()
	case screenSambaMenu:
		return m.viewSambaMenu()
	case screenSambaList:
		return m.viewSambaList()
	case screenNFSMenu:
		return m.viewNFSMenu()
	case screenNFSList:
		return m.viewNFSList()
	case screenUserMenu:
		return m.viewUserMenu()
	case screenUserList:
		return m.viewUserList()
	default:
		return "Under construction...\n\nPress ESC to go back"
	}
}

func (m model) viewMainMenu() string {
	s := titleStyle.Render("Sambo - Linux Share Management") + "\n\n"

	menuItems := []string{
		"Manage Samba Shares",
		"Manage NFS Exports",
		"Manage Users",
		"Exit",
	}

	for i, item := range menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			s += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			s += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s += "\n" + helpStyle.Render("↑/↓: Navigate • Enter: Select • Q: Quit")

	if m.message != "" {
		s += "\n\n"
		if m.messageType == "error" {
			s += errorStyle.Render("✗ " + m.message)
		} else if m.messageType == "success" {
			s += successStyle.Render("✓ " + m.message)
		}
	}

	return s + "\n"
}

func (m model) viewSambaMenu() string {
	s := titleStyle.Render("Samba Share Management") + "\n\n"

	menuItems := []string{
		"List Shares",
		"Create Share",
		"Modify Share",
		"Remove Share",
		"Back to Main Menu",
	}

	for i, item := range menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			s += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			s += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s += "\n" + helpStyle.Render("↑/↓: Navigate • Enter: Select • ESC: Back • Q: Main Menu")

	return s + "\n"
}

func (m model) viewNFSMenu() string {
	s := titleStyle.Render("NFS Export Management") + "\n\n"

	menuItems := []string{
		"List Exports",
		"Create Export",
		"Modify Export",
		"Remove Export",
		"Back to Main Menu",
	}

	for i, item := range menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			s += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			s += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s += "\n" + helpStyle.Render("↑/↓: Navigate • Enter: Select • ESC: Back • Q: Main Menu")

	return s + "\n"
}

func (m model) viewUserMenu() string {
	s := titleStyle.Render("User Management") + "\n\n"

	menuItems := []string{
		"List Users",
		"Add User",
		"Change Password",
		"Remove User",
		"Back to Main Menu",
	}

	for i, item := range menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			s += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			s += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s += "\n" + helpStyle.Render("↑/↓: Navigate • Enter: Select • ESC: Back • Q: Main Menu")

	return s + "\n"
}

func (m model) getMaxCursor() int {
	switch m.currentScreen {
	case screenMainMenu:
		return 3
	case screenSambaMenu, screenNFSMenu, screenUserMenu:
		return 4
	default:
		return 0
	}
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.currentScreen {
	case screenMainMenu:
		switch m.cursor {
		case 0:
			m.currentScreen = screenSambaMenu
			m.cursor = 0
		case 1:
			m.currentScreen = screenNFSMenu
			m.cursor = 0
		case 2:
			m.currentScreen = screenUserMenu
			m.cursor = 0
		case 3:
			return m, tea.Quit
		}

	case screenSambaMenu:
		switch m.cursor {
		case 0:
			m.currentScreen = screenSambaList
		case 1:
			m.currentScreen = screenSambaCreate
			form := newSambaCreateForm()
			m.form = &form
			m.inForm = true
			return m, m.form.Init()
		case 2:
			m.currentScreen = screenSambaModify
		case 3:
			m.currentScreen = screenSambaRemove
			form := newSambaRemoveForm()
			m.form = &form
			m.inForm = true
			return m, m.form.Init()
		case 4:
			m.currentScreen = screenMainMenu
			m.cursor = 0
		}

	case screenNFSMenu:
		switch m.cursor {
		case 0:
			m.currentScreen = screenNFSList
		case 1:
			m.currentScreen = screenNFSCreate
			form := newNFSCreateForm()
			m.form = &form
			m.inForm = true
			return m, m.form.Init()
		case 2:
			m.currentScreen = screenNFSModify
		case 3:
			m.currentScreen = screenNFSRemove
			form := newNFSRemoveForm()
			m.form = &form
			m.inForm = true
			return m, m.form.Init()
		case 4:
			m.currentScreen = screenMainMenu
			m.cursor = 0
		}

	case screenUserMenu:
		switch m.cursor {
		case 0:
			m.currentScreen = screenUserList
		case 1:
			m.currentScreen = screenUserAdd
			form := newUserAddForm()
			m.form = &form
			m.inForm = true
			return m, m.form.Init()
		case 2:
			m.currentScreen = screenUserPassword
			form := newUserPasswordForm()
			m.form = &form
			m.inForm = true
			return m, m.form.Init()
		case 3:
			m.currentScreen = screenUserRemove
			form := newUserRemoveForm()
			m.form = &form
			m.inForm = true
			return m, m.form.Init()
		case 4:
			m.currentScreen = screenMainMenu
			m.cursor = 0
		}
	}

	return m, nil
}

// Start launches the TUI
func Start() error {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
