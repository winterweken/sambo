package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Color palette
	primaryColor   = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9D7EF7"}
	secondaryColor = lipgloss.AdaptiveColor{Light: "#5A4FCF", Dark: "#7D6FD9"}
	successColor   = lipgloss.AdaptiveColor{Light: "#00AA00", Dark: "#00FF00"}
	errorColor     = lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF4444"}
	warningColor   = lipgloss.AdaptiveColor{Light: "#DD8800", Dark: "#FFAA00"}
	infoColor      = lipgloss.AdaptiveColor{Light: "#0088CC", Dark: "#00AAFF"}
	mutedColor     = lipgloss.AdaptiveColor{Light: "#626262", Dark: "#8B8B8B"}
	borderColor    = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9D7EF7"}
	bgColor        = lipgloss.AdaptiveColor{Light: "#FAFAFA", Dark: "#1A1A1A"}

	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Background(bgColor).
			Padding(0, 2).
			MarginTop(1).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			MarginBottom(1)

	// Menu styles
	menuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#333", Dark: "#FFF"}).
			PaddingLeft(2).
			MarginLeft(2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Background(lipgloss.AdaptiveColor{Light: "#F0E6FF", Dark: "#2A1A3F"}).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1).
			MarginLeft(1)

	menuBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	// Help and info styles
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true).
			MarginTop(1).
			PaddingLeft(1)

	helpBoxStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Border(lipgloss.NormalBorder()).
			BorderForeground(mutedColor).
			Padding(0, 1).
			MarginTop(1)

	// Status message styles
	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Background(lipgloss.AdaptiveColor{Light: "#FFE6E6", Dark: "#3A1A1A"}).
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(errorColor)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Background(lipgloss.AdaptiveColor{Light: "#E6FFE6", Dark: "#1A3A1A"}).
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(successColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Background(lipgloss.AdaptiveColor{Light: "#FFF4E6", Dark: "#3A2A1A"}).
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(warningColor)

	infoStyle = lipgloss.NewStyle().
			Foreground(infoColor).
			Background(lipgloss.AdaptiveColor{Light: "#E6F4FF", Dark: "#1A2A3A"}).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(infoColor)
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
	screenMountMenu
	screenMountList
	screenMountCIFS
	screenMountNFS
	screenMountEdit
	screenMountUnmount
	screenMountDiscover
	screenUserMenu
	screenUserList
	screenUserAdd
	screenUserPassword
	screenUserRemove
	screenDependencies
)

type model struct {
	currentScreen screen
	cursor        int
	message       string
	messageType   string // "error", "success", ""

	// Form state
	form   *formModel
	inForm bool

	// Select state
	selectModel *selectModel
	inSelect    bool

	// Data
	selectedItem string
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
	// Delegate to select if we're in select mode
	if m.inSelect && m.selectModel != nil {
		var cmd tea.Cmd
		*m.selectModel, cmd = m.selectModel.Update(msg, &m)
		// Check if select closed
		if !m.inSelect {
			m.selectModel = nil
		}
		return m, cmd
	}

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
	case installMsg:
		return m.handleInstallMsg(msg)

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
			case screenSambaMenu, screenNFSMenu, screenMountMenu, screenUserMenu:
				m.currentScreen = screenMainMenu
			case screenSambaList, screenSambaCreate, screenSambaModify, screenSambaRemove:
				m.currentScreen = screenSambaMenu
			case screenNFSList, screenNFSCreate, screenNFSModify, screenNFSRemove:
				m.currentScreen = screenNFSMenu
			case screenMountList, screenMountCIFS, screenMountNFS, screenMountEdit, screenMountUnmount, screenMountDiscover:
				m.currentScreen = screenMountMenu
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
	// Show select if we're in select mode
	if m.inSelect && m.selectModel != nil {
		return m.selectModel.View()
	}

	// Show form if we're in a form
	if m.inForm && m.form != nil {
		return m.form.ViewWithMessage(m.message, m.messageType)
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
	case screenMountMenu:
		return m.viewMountMenu()
	case screenMountList:
		return m.viewMountList()
	case screenMountDiscover:
		return m.viewMountDiscover()
	case screenUserMenu:
		return m.viewUserMenu()
	case screenUserList:
		return m.viewUserList()
	case screenDependencies:
		return m.viewDependencies()
	default:
		return "Under construction...\n\nPress ESC to go back"
	}
}

// Helper function to render messages
func (m model) renderMessage() string {
	if m.message == "" {
		return ""
	}

	prefix := "\n\n"
	switch m.messageType {
	case "error":
		return prefix + errorStyle.Render(" ✗ "+m.message+" ")
	case "success":
		return prefix + successStyle.Render(" ✓ "+m.message+" ")
	case "warning":
		return prefix + warningStyle.Render(" ⚠ "+m.message+" ")
	default:
		return prefix + infoStyle.Render(" ℹ "+m.message+" ")
	}
}

func (m model) viewMainMenu() string {
	s := titleStyle.Render(" Sambo - Linux Share Management ") + "\n\n"

	menuItems := []string{
		"📁 Manage Samba Shares",
		"🌐 Manage NFS Exports",
		"💾 Manage Network Mounts",
		"👤 Manage Users",
		"🔧 Check & Install Dependencies",
		"🚪 Exit",
	}

	menuContent := ""
	for i, item := range menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			menuContent += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			menuContent += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s += menuBoxStyle.Render(menuContent)
	s += "\n" + helpBoxStyle.Render("↑/↓ or j/k: Navigate • Enter: Select • Q: Quit")
	s += m.renderMessage()

	return s + "\n"
}

func (m model) viewSambaMenu() string {
	s := titleStyle.Render(" Samba Share Management ") + "\n\n"

	menuItems := []string{
		"📋 List Shares",
		"➕ Create Share",
		"✏️  Modify Share",
		"🗑️  Remove Share",
		"⬅️  Back to Main Menu",
	}

	menuContent := ""
	for i, item := range menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			menuContent += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			menuContent += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s += menuBoxStyle.Render(menuContent)
	s += "\n" + helpBoxStyle.Render("↑/↓ or j/k: Navigate • Enter: Select • ESC: Back • Q: Main Menu")
	s += m.renderMessage()

	return s + "\n"
}

func (m model) viewNFSMenu() string {
	s := titleStyle.Render(" NFS Export Management ") + "\n\n"

	menuItems := []string{
		"📋 List Exports",
		"➕ Create Export",
		"✏️  Modify Export",
		"🗑️  Remove Export",
		"⬅️  Back to Main Menu",
	}

	menuContent := ""
	for i, item := range menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			menuContent += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			menuContent += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s += menuBoxStyle.Render(menuContent)
	s += "\n" + helpBoxStyle.Render("↑/↓ or j/k: Navigate • Enter: Select • ESC: Back • Q: Main Menu")
	s += m.renderMessage()

	return s + "\n"
}

func (m model) viewMountMenu() string {
	s := titleStyle.Render(" Network Mount Management ") + "\n\n"

	menuItems := []string{
		"📋 List Mounts",
		"💾 Mount CIFS/SMB Share",
		"🌐 Mount NFS Share",
		"✏️  Edit Mount",
		"⏏️  Unmount Share",
		"🔍 Discover NFS Servers",
		"⬅️  Back to Main Menu",
	}

	menuContent := ""
	for i, item := range menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			menuContent += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			menuContent += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s += menuBoxStyle.Render(menuContent)
	s += "\n" + helpBoxStyle.Render("↑/↓ or j/k: Navigate • Enter: Select • ESC: Back • Q: Main Menu")
	s += m.renderMessage()

	return s + "\n"
}

func (m model) viewUserMenu() string {
	s := titleStyle.Render(" User Management ") + "\n\n"

	menuItems := []string{
		"📋 List Users",
		"➕ Add User",
		"🔑 Change Password",
		"🗑️  Remove User",
		"⬅️  Back to Main Menu",
	}

	menuContent := ""
	for i, item := range menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			menuContent += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			menuContent += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s += menuBoxStyle.Render(menuContent)
	s += "\n" + helpBoxStyle.Render("↑/↓ or j/k: Navigate • Enter: Select • ESC: Back • Q: Main Menu")
	s += m.renderMessage()

	return s + "\n"
}

func (m model) getMaxCursor() int {
	switch m.currentScreen {
	case screenMainMenu:
		return 5 // 6 items including Dependencies (0-5)
	case screenSambaMenu, screenNFSMenu, screenUserMenu:
		return 4
	case screenMountMenu:
		return 6 // 7 items including Edit and Discover (0-6)
	case screenDependencies:
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
			m.currentScreen = screenMountMenu
			m.cursor = 0
		case 3:
			m.currentScreen = screenUserMenu
			m.cursor = 0
		case 4:
			m.currentScreen = screenDependencies
			m.cursor = 0
		case 5:
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
			selectModel, err := newSambaModifySelect()
			if err != nil {
				m.message = fmt.Sprintf("Error: %v", err)
				m.messageType = "error"
				m.currentScreen = screenSambaMenu
				return m, nil
			}
			m.selectModel = &selectModel
			m.inSelect = true
			return m, m.selectModel.Init()
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
			selectModel, err := newNFSModifySelect()
			if err != nil {
				m.message = fmt.Sprintf("Error: %v", err)
				m.messageType = "error"
				m.currentScreen = screenNFSMenu
				return m, nil
			}
			m.selectModel = &selectModel
			m.inSelect = true
			return m, m.selectModel.Init()
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

	case screenMountMenu:
		switch m.cursor {
		case 0:
			m.currentScreen = screenMountList
		case 1:
			m.currentScreen = screenMountCIFS
			form := newMountCIFSForm()
			m.form = &form
			m.inForm = true
			return m, m.form.Init()
		case 2:
			m.currentScreen = screenMountNFS
			form := newMountNFSForm()
			m.form = &form
			m.inForm = true
			return m, m.form.Init()
		case 3:
			// Show selection list for editing mount
			m.currentScreen = screenMountEdit
			selectModel, err := newMountEditSelect()
			if err != nil {
				m.message = fmt.Sprintf("Failed to list mounts: %v", err)
				m.messageType = "error"
				return m, nil
			}
			m.selectModel = &selectModel
			m.inSelect = true
			return m, nil
		case 4:
			// Show selection list of mounted shares
			selectModel, err := newMountUnmountSelect()
			if err != nil {
				m.message = fmt.Sprintf("Failed to list mounts: %v", err)
				m.messageType = "error"
				return m, nil
			}
			m.selectModel = &selectModel
			m.inSelect = true
			return m, nil
		case 5:
			m.currentScreen = screenMountDiscover
		case 6:
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

	case screenDependencies:
		return m.handleDependenciesEnter()
	}

	return m, nil
}

// Start launches the TUI
func Start() error {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
