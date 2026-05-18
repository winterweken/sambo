package tui

import (
	"fmt"
	"sambo/pkg/samba"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	// Force TrueColor for consistent friendly palette
	lipgloss.SetColorProfile(termenv.TrueColor)
}

var (
	// Color palette - Friendly Refresh
	// Using a softer, more modern palette
	primaryColor   = lipgloss.AdaptiveColor{Light: "#6366F1", Dark: "#818CF8"} // Indigo
	secondaryColor = lipgloss.AdaptiveColor{Light: "#EC4899", Dark: "#F472B6"} // Pink
	successColor   = lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#34D399"} // Emerald
	errorColor     = lipgloss.AdaptiveColor{Light: "#EF4444", Dark: "#F87171"} // Red
	warningColor   = lipgloss.AdaptiveColor{Light: "#F59E0B", Dark: "#FBBF24"} // Amber
	infoColor      = lipgloss.AdaptiveColor{Light: "#3B82F6", Dark: "#60A5FA"} // Blue
	mutedColor     = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"} // Gray
	borderColor    = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"} // Gray-200/700

	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(1, 4). // More padding for "banner" look
			MarginTop(1).
			MarginBottom(1).
			Align(lipgloss.Center)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			MarginBottom(1).
			PaddingLeft(1)

	// Menu styles -- Natural Width with Alignment
	menuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#D1D5DB"}).
			PaddingLeft(2).
			PaddingRight(1).
			MarginLeft(0)

	selectedStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Background(lipgloss.AdaptiveColor{Light: "#EEF2FF", Dark: "#312E81"}).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1).
			MarginLeft(0).
			Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(primaryColor)

	menuBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 2). // Less aggressive padding
			MarginTop(0).
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
			BorderForeground(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}).
			Padding(0, 1).
			MarginTop(1).
			Italic(true)

	// Status message styles - Softer look
	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true).
			Padding(0, 1)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			Foreground(infoColor).
			Padding(0, 1)
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

	// Spinner for long-running operations
	spinner      spinner.Model
	isInstalling bool
	installMsg   string // What's being installed

	// Data
	selectedItem string

	// Managers
	sambaManager *samba.Manager
}

func newModel(sambaManager *samba.Manager) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	return model{
		currentScreen: screenMainMenu,
		cursor:        0,
		inForm:        false,
		spinner:       s,
		sambaManager:  sambaManager,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle spinner updates when installing
	if m.isInstalling {
		switch msg := msg.(type) {
		case spinner.TickMsg:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		case installMsg:
			m.isInstalling = false
			m.installMsg = ""
			return m.handleInstallMsg(msg)
		}
		// Ignore other input while installing
		return m, nil
	}

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
		m.isInstalling = false
		m.installMsg = ""
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
	return m.renderMenuView(" Sambo - Linux Share Management ", []string{
		"📁 Manage Samba Shares",
		"🌐 Manage NFS Exports",
		"💾 Manage Network Mounts",
		"👤 Manage Users",
		"🔧 Check & Install Dependencies",
		"🚪 Exit",
	}, false)
}

func (m model) viewSambaMenu() string {
	return m.renderMenuView(" Samba Share Management ", []string{
		"📋 List Shares",
		"➕ Create Share",
		"✏️  Modify Share",
		"🗑️  Remove Share",
		"⬅️  Back to Main Menu",
	}, true)
}

func (m model) viewNFSMenu() string {
	return m.renderMenuView(" NFS Export Management ", []string{
		"📋 List Exports",
		"➕ Create Export",
		"✏️  Modify Export",
		"🗑️  Remove Export",
		"⬅️  Back to Main Menu",
	}, true)
}

func (m model) viewMountMenu() string {
	return m.renderMenuView(" Network Mount Management ", []string{
		"📋 List Mounts",
		"💾 Mount CIFS/SMB Share",
		"🌐 Mount NFS Share",
		"✏️  Edit Mount",
		"⏏️  Unmount Share",
		"🔍 Discover NFS Servers",
		"⬅️  Back to Main Menu",
	}, true)
}

func (m model) viewUserMenu() string {
	return m.renderMenuView(" User Management ", []string{
		"📋 List Users",
		"➕ Add User",
		"🔑 Change Password",
		"🗑️  Remove User",
		"⬅️  Back to Main Menu",
	}, true)
}

// renderMenuView renders a standard menu with title, items, cursor, and optional back-nav help text
func (m model) renderMenuView(title string, items []string, hasBack bool) string {
	s := titleStyle.Render(title) + "\n\n"

	menuContent := ""
	for i, item := range items {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			menuContent += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			menuContent += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s += menuBoxStyle.Render(menuContent)

	if hasBack {
		s += "\n" + helpBoxStyle.Render("↑/↓ or j/k: Navigate • Enter: Select • ESC: Back • Q: Main Menu")
	} else {
		s += "\n" + helpBoxStyle.Render("↑/↓ or j/k: Navigate • Enter: Select • Q: Quit")
	}
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
			selectModel, err := newSambaModifySelect(m.sambaManager)
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
func Start(sambaManager *samba.Manager) error {
	p := tea.NewProgram(newModel(sambaManager), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
