package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"sambo/pkg/mount"
	"sambo/pkg/nfs"
	"sambo/pkg/samba"
	"sambo/pkg/user"
)

var (
	focusedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9D7EF7"}).
			Bold(true)

	blurredStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#666666"})

	cursorStyle = focusedStyle.Copy()
	noStyle     = lipgloss.NewStyle()

	focusedButton = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"}).
			Background(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9D7EF7"}).
			Padding(0, 3).
			Bold(true).
			Render("Submit")

	blurredButton = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#666666"}).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}).
			Padding(0, 3).
			Render("Submit")

	checkboxChecked = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#00AA00", Dark: "#00FF00"}).
			Bold(true).
			Render("[✓]")

	checkboxUnchecked = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}).
				Render("[ ]")

	formBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9D7EF7"}).
			Padding(1, 2).
			MarginTop(1)

	fieldLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#5A4FCF", Dark: "#7D6FD9"}).
			Bold(true)

	fieldDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#666666"}).
			Italic(true)
)

type formField struct {
	label       string
	input       textinput.Model
	checkbox    bool
	checkValue  bool
	description string
	displayOnly bool // For read-only display fields
}

type formModel struct {
	fields       []formField
	focusIndex   int
	submitIndex  int
	formType     string // "samba-create", "nfs-create", "user-add", etc.
	timeMachine  bool
	readOnly     bool
	browseable   bool
	returnScreen screen
	originalName string // For modify operations
}

func newSambaCreateForm() formModel {
	inputs := make([]formField, 6)

	inputs[0] = formField{
		label:       "Share Name",
		input:       makeInput("myshare", "Share name (required)"),
		description: "Name for the share",
	}
	inputs[1] = formField{
		label:       "Path",
		input:       makeInput("/mnt/data", "Path to directory (required)"),
		description: "Directory path to share",
	}
	inputs[2] = formField{
		label:       "Comment",
		input:       makeInput("", "Share description"),
		description: "Optional description",
	}
	inputs[3] = formField{
		label:       "Valid Users",
		input:       makeInput("", "Comma-separated usernames"),
		description: "Users who can access (leave empty for all)",
	}
	inputs[4] = formField{
		label:       "Time Machine",
		checkbox:    true,
		checkValue:  false,
		description: "Enable Apple Time Machine support",
	}
	inputs[5] = formField{
		label:       "TM Max Size",
		input:       makeInput("0", "e.g., 500G, 1T, 0=unlimited"),
		description: "Max backup size (only used if Time Machine enabled)",
	}

	inputs[0].input.Focus()
	inputs[0].input.PromptStyle = focusedStyle
	inputs[0].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   0,
		submitIndex:  len(inputs),
		formType:     "samba-create",
		returnScreen: screenSambaMenu,
	}
}

func newNFSCreateForm() formModel {
	inputs := make([]formField, 5)

	inputs[0] = formField{
		label:       "Export Path",
		input:       makeInput("/mnt/data", "Path to export (required)"),
		description: "Directory path to export",
	}
	inputs[1] = formField{
		label:       "Clients",
		input:       makeInput("*", "IP, CIDR, or * for all"),
		description: "Client access specification (e.g., 192.168.1.0/24)",
	}
	inputs[2] = formField{
		label:       "Read Only",
		checkbox:    true,
		checkValue:  false,
		description: "Mount as read-only (recommended for media servers)",
	}
	inputs[3] = formField{
		label:       "No Root Squash",
		checkbox:    true,
		checkValue:  false,
		description: "Allow root access (needed for backups/Time Machine)",
	}
	inputs[4] = formField{
		label:       "Async Mode",
		checkbox:    true,
		checkValue:  false,
		description: "Faster but less safe (good for non-critical data)",
	}

	inputs[0].input.Focus()
	inputs[0].input.PromptStyle = focusedStyle
	inputs[0].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   0,
		submitIndex:  len(inputs),
		formType:     "nfs-create",
		returnScreen: screenNFSMenu,
	}
}

func newUserAddForm() formModel {
	inputs := make([]formField, 2)

	inputs[0] = formField{
		label:       "Username",
		input:       makeInput("", "Username (required)"),
		description: "System username",
	}
	inputs[1] = formField{
		label:       "Password",
		input:       makePasswordInput("", "Password (required)"),
		description: "User password",
	}

	inputs[0].input.Focus()
	inputs[0].input.PromptStyle = focusedStyle
	inputs[0].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   0,
		submitIndex:  len(inputs),
		formType:     "user-add",
		returnScreen: screenUserMenu,
	}
}

func newUserPasswordForm() formModel {
	inputs := make([]formField, 2)

	inputs[0] = formField{
		label:       "Username",
		input:       makeInput("", "Username (required)"),
		description: "Existing username",
	}
	inputs[1] = formField{
		label:       "New Password",
		input:       makePasswordInput("", "New password (required)"),
		description: "New password for user",
	}

	inputs[0].input.Focus()
	inputs[0].input.PromptStyle = focusedStyle
	inputs[0].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   0,
		submitIndex:  len(inputs),
		formType:     "user-password",
		returnScreen: screenUserMenu,
	}
}

func newSambaRemoveForm() formModel {
	inputs := make([]formField, 1)

	inputs[0] = formField{
		label:       "Share Name",
		input:       makeInput("", "Name of share to remove"),
		description: "Existing share name",
	}

	inputs[0].input.Focus()
	inputs[0].input.PromptStyle = focusedStyle
	inputs[0].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   0,
		submitIndex:  len(inputs),
		formType:     "samba-remove",
		returnScreen: screenSambaMenu,
	}
}

func newNFSRemoveForm() formModel {
	inputs := make([]formField, 1)

	inputs[0] = formField{
		label:       "Export Path",
		input:       makeInput("", "Path to remove"),
		description: "Existing export path",
	}

	inputs[0].input.Focus()
	inputs[0].input.PromptStyle = focusedStyle
	inputs[0].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   0,
		submitIndex:  len(inputs),
		formType:     "nfs-remove",
		returnScreen: screenNFSMenu,
	}
}

func newUserRemoveForm() formModel {
	inputs := make([]formField, 1)

	inputs[0] = formField{
		label:       "Username",
		input:       makeInput("", "Username to remove"),
		description: "Existing username",
	}

	inputs[0].input.Focus()
	inputs[0].input.PromptStyle = focusedStyle
	inputs[0].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   0,
		submitIndex:  len(inputs),
		formType:     "user-remove",
		returnScreen: screenUserMenu,
	}
}

func newSambaModifyForm(shareName string) (formModel, error) {
	// Load existing share
	share, err := samba.Get(shareName)
	if err != nil {
		return formModel{}, err
	}

	inputs := make([]formField, 8)

	// Share name (read-only display)
	nameInput := makeInput(share.Name, "Share name (read-only)")
	nameInput.CharLimit = 0
	nameInput.Width = 50
	inputs[0] = formField{
		label:       "Share Name",
		input:       nameInput,
		description: "Cannot be changed",
	}

	inputs[1] = formField{
		label:       "Comment",
		input:       makeInput(share.Comment, "Share description"),
		description: "Optional description",
	}
	inputs[1].input.SetValue(share.Comment)

	// Valid users
	usersStr := ""
	if len(share.ValidUsers) > 0 {
		usersStr = strings.Join(share.ValidUsers, ", ")
	}
	inputs[2] = formField{
		label:       "Valid Users",
		input:       makeInput(usersStr, "Comma-separated usernames"),
		description: "Users who can access (leave empty for all)",
	}
	inputs[2].input.SetValue(usersStr)

	// Read-only checkbox
	inputs[3] = formField{
		label:       "Read Only",
		checkbox:    true,
		checkValue:  share.ReadOnly,
		description: "Make share read-only",
	}

	// Browseable checkbox
	inputs[4] = formField{
		label:       "Browseable",
		checkbox:    true,
		checkValue:  share.Browseable,
		description: "Make share browseable",
	}

	// Time Machine checkbox
	inputs[5] = formField{
		label:       "Time Machine",
		checkbox:    true,
		checkValue:  share.TimeMachine,
		description: "Enable Apple Time Machine support",
	}

	// Time Machine max size
	tmMaxSize := share.TimeMachineMaxSize
	if tmMaxSize == "" {
		tmMaxSize = "0"
	}
	inputs[6] = formField{
		label:       "TM Max Size",
		input:       makeInput(tmMaxSize, "e.g., 500G, 1T, 0=unlimited"),
		description: "Max backup size (only used if Time Machine enabled)",
	}
	inputs[6].input.SetValue(tmMaxSize)

	// Note about path
	inputs[7] = formField{
		label:       fmt.Sprintf("Path: %s (cannot be changed)", share.Path),
		checkbox:    false,
		description: "",
		displayOnly: true,
	}

	inputs[1].input.Focus()
	inputs[1].input.PromptStyle = focusedStyle
	inputs[1].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   1, // Start at comment field (skip name)
		submitIndex:  len(inputs),
		formType:     "samba-modify",
		timeMachine:  share.TimeMachine,
		readOnly:     share.ReadOnly,
		browseable:   share.Browseable,
		returnScreen: screenSambaMenu,
		originalName: share.Name,
	}, nil
}

func newNFSModifyForm(exportPath string) (formModel, error) {
	// Load existing export
	export, err := nfs.Get(exportPath)
	if err != nil {
		return formModel{}, err
	}

	// Parse existing options
	readOnly := strings.Contains(export.Options, "ro")
	noRootSquash := strings.Contains(export.Options, "no_root_squash")
	asyncMode := strings.Contains(export.Options, "async")

	inputs := make([]formField, 5)

	// Path (read-only display)
	inputs[0] = formField{
		label:       fmt.Sprintf("Path: %s (cannot be changed)", export.Path),
		checkbox:    false,
		description: "",
		displayOnly: true,
	}

	clientsInput := makeInput("", "IP, CIDR, or * for all")
	clientsInput.SetValue(export.Clients)
	inputs[1] = formField{
		label:       "Clients",
		input:       clientsInput,
		description: "Client access specification (e.g., 192.168.1.0/24)",
	}

	inputs[2] = formField{
		label:       "Read Only",
		checkbox:    true,
		checkValue:  readOnly,
		description: "Mount as read-only (recommended for media servers)",
	}

	inputs[3] = formField{
		label:       "No Root Squash",
		checkbox:    true,
		checkValue:  noRootSquash,
		description: "Allow root access (needed for backups/Time Machine)",
	}

	inputs[4] = formField{
		label:       "Async Mode",
		checkbox:    true,
		checkValue:  asyncMode,
		description: "Faster but less safe (good for non-critical data)",
	}

	inputs[1].input.Focus()
	inputs[1].input.PromptStyle = focusedStyle
	inputs[1].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   1, // Start at clients field
		submitIndex:  len(inputs),
		formType:     "nfs-modify",
		returnScreen: screenNFSMenu,
		originalName: export.Path,
	}, nil
}

func makeInput(placeholder, prompt string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = prompt + ": "
	ti.CharLimit = 200
	ti.Width = 50
	return ti
}

func makePasswordInput(placeholder, prompt string) textinput.Model {
	ti := makeInput(placeholder, prompt)
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	return ti
}

func (fm formModel) Init() tea.Cmd {
	return textinput.Blink
}

func (fm formModel) Update(msg tea.Msg, parent *model) (formModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			parent.currentScreen = fm.returnScreen
			parent.message = ""
			parent.inForm = false
			return fm, nil

		case "tab", "shift+tab", "up", "down":
			s := msg.String()

			// Move focus
			if s == "up" || s == "shift+tab" {
				fm.focusIndex--
			} else {
				fm.focusIndex++
			}

			if fm.focusIndex > fm.submitIndex {
				fm.focusIndex = 0
			} else if fm.focusIndex < 0 {
				fm.focusIndex = fm.submitIndex
			}

			// Update input focus
			for i := range fm.fields {
				if i == fm.focusIndex && !fm.fields[i].checkbox && !fm.fields[i].displayOnly {
					fm.fields[i].input.Focus()
					fm.fields[i].input.PromptStyle = focusedStyle
					fm.fields[i].input.TextStyle = focusedStyle
				} else if !fm.fields[i].checkbox && !fm.fields[i].displayOnly {
					fm.fields[i].input.Blur()
					fm.fields[i].input.PromptStyle = noStyle
					fm.fields[i].input.TextStyle = noStyle
				}
			}

			return fm, nil

		case "enter":
			// Handle checkbox toggle
			if fm.focusIndex < len(fm.fields) && fm.fields[fm.focusIndex].checkbox {
				fm.fields[fm.focusIndex].checkValue = !fm.fields[fm.focusIndex].checkValue
				if fm.formType == "samba-create" && fm.focusIndex == 4 {
					fm.timeMachine = fm.fields[fm.focusIndex].checkValue
				}
				return fm, nil
			}

			// Submit form
			if fm.focusIndex == fm.submitIndex {
				return fm.submitForm(parent)
			}

		case " ":
			// Toggle checkbox with space
			if fm.focusIndex < len(fm.fields) && fm.fields[fm.focusIndex].checkbox {
				fm.fields[fm.focusIndex].checkValue = !fm.fields[fm.focusIndex].checkValue
				if fm.formType == "samba-create" && fm.focusIndex == 4 {
					fm.timeMachine = fm.fields[fm.focusIndex].checkValue
				}
				return fm, nil
			}
		}
	}

	// Update the focused input
	if fm.focusIndex < len(fm.fields) && !fm.fields[fm.focusIndex].checkbox && !fm.fields[fm.focusIndex].displayOnly {
		var cmd tea.Cmd
		fm.fields[fm.focusIndex].input, cmd = fm.fields[fm.focusIndex].input.Update(msg)
		return fm, cmd
	}

	return fm, nil
}

func (fm formModel) submitForm(parent *model) (formModel, tea.Cmd) {
	switch fm.formType {
	case "samba-create":
		return fm.submitSambaCreate(parent)
	case "samba-modify":
		return fm.submitSambaModify(parent)
	case "samba-remove":
		return fm.submitSambaRemove(parent)
	case "nfs-create":
		return fm.submitNFSCreate(parent)
	case "nfs-modify":
		return fm.submitNFSModify(parent)
	case "nfs-remove":
		return fm.submitNFSRemove(parent)
	case "mount-cifs":
		return fm.submitMountCIFS(parent)
	case "mount-nfs":
		return fm.submitMountNFS(parent)
	case "mount-unmount":
		return fm.submitMountUnmount(parent)
	case "mount-edit":
		return fm.submitMountEdit(parent)
	case "user-add":
		return fm.submitUserAdd(parent)
	case "user-password":
		return fm.submitUserPassword(parent)
	case "user-remove":
		return fm.submitUserRemove(parent)
	}
	return fm, nil
}

func (fm formModel) submitSambaCreate(parent *model) (formModel, tea.Cmd) {
	name := fm.fields[0].input.Value()
	path := fm.fields[1].input.Value()
	comment := fm.fields[2].input.Value()
	users := fm.fields[3].input.Value()
	tmMaxSize := fm.fields[5].input.Value()

	if name == "" || path == "" {
		parent.message = "Name and path are required"
		parent.messageType = "error"
		return fm, nil
	}

	// Check if Samba is installed (non-interactive for TUI)
	if err := samba.CheckInstalledInteractive(false); err != nil {
		parent.message = fmt.Sprintf("%v", err)
		parent.messageType = "error"
		return fm, nil
	}

	share := samba.Share{
		Name:              name,
		Path:              path,
		Comment:           comment,
		ReadOnly:          false,
		Browseable:        true,
		TimeMachine:       fm.timeMachine,
		TimeMachineMaxSize: tmMaxSize,
	}

	if users != "" {
		share.ValidUsers = strings.Split(users, ",")
		for i := range share.ValidUsers {
			share.ValidUsers[i] = strings.TrimSpace(share.ValidUsers[i])
		}
	}

	if err := samba.Create(share); err != nil {
		parent.message = fmt.Sprintf("Failed to create share: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("Share '%s' created successfully. Samba service reloaded.", name)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitSambaRemove(parent *model) (formModel, tea.Cmd) {
	name := fm.fields[0].input.Value()

	if name == "" {
		parent.message = "Share name is required"
		parent.messageType = "error"
		return fm, nil
	}

	if err := samba.Remove(name); err != nil {
		parent.message = fmt.Sprintf("Failed to remove share: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("Share '%s' removed successfully. Samba service reloaded.", name)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitSambaModify(parent *model) (formModel, tea.Cmd) {
	comment := fm.fields[1].input.Value()
	users := fm.fields[2].input.Value()
	readOnly := fm.fields[3].checkValue
	browseable := fm.fields[4].checkValue
	timeMachine := fm.fields[5].checkValue
	tmMaxSize := fm.fields[6].input.Value()

	// Get the original share to keep path
	share, err := samba.Get(fm.originalName)
	if err != nil {
		parent.message = fmt.Sprintf("Failed to load share: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	// Update share with new values
	share.Comment = comment
	share.ReadOnly = readOnly
	share.Browseable = browseable
	share.TimeMachine = timeMachine
	share.TimeMachineMaxSize = tmMaxSize

	if users != "" {
		share.ValidUsers = strings.Split(users, ",")
		for i := range share.ValidUsers {
			share.ValidUsers[i] = strings.TrimSpace(share.ValidUsers[i])
		}
	} else {
		share.ValidUsers = nil
	}

	// Remove and re-create the share
	if err := samba.Remove(fm.originalName); err != nil {
		parent.message = fmt.Sprintf("Failed to modify share: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	if err := samba.Create(*share); err != nil {
		parent.message = fmt.Sprintf("Failed to update share: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("Share '%s' modified successfully. Samba service reloaded.", fm.originalName)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitNFSCreate(parent *model) (formModel, tea.Cmd) {
	path := fm.fields[0].input.Value()
	clients := fm.fields[1].input.Value()
	readOnly := fm.fields[2].checkValue
	noRootSquash := fm.fields[3].checkValue
	asyncMode := fm.fields[4].checkValue

	if path == "" {
		parent.message = "Path is required"
		parent.messageType = "error"
		return fm, nil
	}

	// Check if NFS is installed (non-interactive for TUI)
	if err := nfs.CheckInstalledInteractive(false); err != nil {
		parent.message = fmt.Sprintf("%v", err)
		parent.messageType = "error"
		return fm, nil
	}

	if clients == "" {
		clients = "*"
	}

	// Build options string based on selections
	var options []string
	if readOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}

	if asyncMode {
		options = append(options, "async")
	} else {
		options = append(options, "sync")
	}

	if noRootSquash {
		options = append(options, "no_root_squash")
	} else {
		options = append(options, "root_squash")
	}

	options = append(options, "no_subtree_check")

	export := nfs.Export{
		Path:    path,
		Clients: clients,
		Options: strings.Join(options, ","),
	}

	if err := nfs.Create(export); err != nil {
		parent.message = fmt.Sprintf("Failed to create export: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("Export '%s' created successfully. NFS exports reloaded.", path)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitNFSRemove(parent *model) (formModel, tea.Cmd) {
	path := fm.fields[0].input.Value()

	if path == "" {
		parent.message = "Path is required"
		parent.messageType = "error"
		return fm, nil
	}

	if err := nfs.Remove(path); err != nil {
		parent.message = fmt.Sprintf("Failed to remove export: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("Export '%s' removed successfully. NFS exports reloaded.", path)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitNFSModify(parent *model) (formModel, tea.Cmd) {
	clients := fm.fields[1].input.Value()
	readOnly := fm.fields[2].checkValue
	noRootSquash := fm.fields[3].checkValue
	asyncMode := fm.fields[4].checkValue

	if clients == "" {
		clients = "*"
	}

	// Build options string based on selections
	var options []string
	if readOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}

	if asyncMode {
		options = append(options, "async")
	} else {
		options = append(options, "sync")
	}

	if noRootSquash {
		options = append(options, "no_root_squash")
	} else {
		options = append(options, "root_squash")
	}

	options = append(options, "no_subtree_check")

	// Remove and re-create the export
	if err := nfs.Remove(fm.originalName); err != nil {
		parent.message = fmt.Sprintf("Failed to modify export: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	export := nfs.Export{
		Path:    fm.originalName,
		Clients: clients,
		Options: strings.Join(options, ","),
	}

	if err := nfs.Create(export); err != nil {
		parent.message = fmt.Sprintf("Failed to update export: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("Export '%s' modified successfully. NFS exports reloaded.", fm.originalName)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitUserAdd(parent *model) (formModel, tea.Cmd) {
	username := fm.fields[0].input.Value()
	password := fm.fields[1].input.Value()

	if username == "" || password == "" {
		parent.message = "Username and password are required"
		parent.messageType = "error"
		return fm, nil
	}

	if err := user.Add(username, password, true); err != nil {
		parent.message = fmt.Sprintf("Failed to add user: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("User '%s' added successfully", username)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitUserPassword(parent *model) (formModel, tea.Cmd) {
	username := fm.fields[0].input.Value()
	password := fm.fields[1].input.Value()

	if username == "" || password == "" {
		parent.message = "Username and password are required"
		parent.messageType = "error"
		return fm, nil
	}

	if err := user.SetPassword(username, password); err != nil {
		parent.message = fmt.Sprintf("Failed to set password: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("Password for '%s' updated successfully", username)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitUserRemove(parent *model) (formModel, tea.Cmd) {
	username := fm.fields[0].input.Value()

	if username == "" {
		parent.message = "Username is required"
		parent.messageType = "error"
		return fm, nil
	}

	if err := user.Remove(username, false); err != nil {
		parent.message = fmt.Sprintf("Failed to remove user: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("User '%s' removed successfully", username)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

// Mount form functions

func newMountCIFSForm() formModel {
	inputs := make([]formField, 5)

	inputs[0] = formField{
		label:       "Source Share",
		input:       makeInput("//server/share", "Source (//server/share)"),
		description: "Remote CIFS/SMB share path",
	}
	inputs[1] = formField{
		label:       "Mount Point",
		input:       makeInput("/mnt/share", "Local mount point"),
		description: "Local directory to mount at",
	}
	inputs[2] = formField{
		label:       "Username",
		input:       makeInput("", "Username (optional)"),
		description: "Leave empty to use credentials file",
	}
	inputs[3] = formField{
		label:       "Password",
		input:       makePasswordInput("", "Password (optional)"),
		description: "Leave empty to use credentials file",
	}
	inputs[4] = formField{
		label:       "Persistent",
		checkbox:    true,
		checkValue:  false,
		description: "Add to /etc/fstab for automatic mounting",
	}

	inputs[0].input.Focus()
	inputs[0].input.PromptStyle = focusedStyle
	inputs[0].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   0,
		submitIndex:  len(inputs),
		formType:     "mount-cifs",
		returnScreen: screenMountMenu,
	}
}

func newMountNFSForm() formModel {
	inputs := make([]formField, 3)

	inputs[0] = formField{
		label:       "Source Share",
		input:       makeInput("server:/path", "Source (server:/path)"),
		description: "Remote NFS share path",
	}
	inputs[1] = formField{
		label:       "Mount Point",
		input:       makeInput("/mnt/share", "Local mount point"),
		description: "Local directory to mount at",
	}
	inputs[2] = formField{
		label:       "Persistent",
		checkbox:    true,
		checkValue:  false,
		description: "Add to /etc/fstab for automatic mounting",
	}

	inputs[0].input.Focus()
	inputs[0].input.PromptStyle = focusedStyle
	inputs[0].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   0,
		submitIndex:  len(inputs),
		formType:     "mount-nfs",
		returnScreen: screenMountMenu,
	}
}

func newMountUnmountForm() formModel {
	return newMountUnmountFormWithPath("")
}

func newMountUnmountFormWithPath(mountPoint string) formModel {
	inputs := make([]formField, 2)

	inputs[0] = formField{
		label:       "Mount Point",
		input:       makeInput(mountPoint, "Path to unmount"),
		description: "Path to the mounted directory",
	}
	inputs[1] = formField{
		label:       "Remove Persistent",
		checkbox:    true,
		checkValue:  false,
		description: "Also remove from /etc/fstab",
	}

	// Set the mount point value if provided
	if mountPoint != "" {
		inputs[0].input.SetValue(mountPoint)
	}

	inputs[0].input.Focus()
	inputs[0].input.PromptStyle = focusedStyle
	inputs[0].input.TextStyle = focusedStyle

	return formModel{
		fields:       inputs,
		focusIndex:   0,
		submitIndex:  len(inputs),
		formType:     "mount-unmount",
		returnScreen: screenMountMenu,
	}
}

func newMountEditForm(mountPoint string) (formModel, error) {
	mnt, err := mount.Get(mountPoint)
	if err != nil {
		return formModel{}, err
	}

	inputs := make([]formField, 4)

	// Mount point (read-only display)
	inputs[0] = formField{
		label:       "Mount Point",
		input:       makeInput(mnt.MountPoint, ""),
		description: "Path to the mounted directory",
		displayOnly: true,
	}
	inputs[0].input.SetValue(mnt.MountPoint)

	// Source (read-only display)
	inputs[1] = formField{
		label:       "Source",
		input:       makeInput(mnt.Source, ""),
		description: "Remote share location",
		displayOnly: true,
	}
	inputs[1].input.SetValue(mnt.Source)

	// Type (read-only display)
	inputs[2] = formField{
		label:       "Type",
		input:       makeInput(string(mnt.Type), ""),
		description: "Mount type (NFS or CIFS)",
		displayOnly: true,
	}
	inputs[2].input.SetValue(strings.ToUpper(string(mnt.Type)))

	// Persistent toggle
	inputs[3] = formField{
		label:       "Persistent",
		checkbox:    true,
		checkValue:  mnt.Persistent,
		description: "Mount automatically on boot",
	}

	return formModel{
		fields:       inputs,
		focusIndex:   3, // Start on the checkbox
		submitIndex:  len(inputs),
		formType:     "mount-edit",
		returnScreen: screenMountMenu,
	}, nil
}

func (fm formModel) submitMountCIFS(parent *model) (formModel, tea.Cmd) {
	source := fm.fields[0].input.Value()
	mountPoint := fm.fields[1].input.Value()
	username := fm.fields[2].input.Value()
	password := fm.fields[3].input.Value()
	persistent := fm.fields[4].checkValue

	if source == "" || mountPoint == "" {
		parent.message = "Source and mount point are required"
		parent.messageType = "error"
		return fm, nil
	}

	var err error
	if username != "" && password != "" {
		// Use secure credentials handling - credentials stored in file, not command line
		err = mount.MountCIFSWithCredentials(source, mountPoint, username, password, persistent)
	} else {
		// Use default credentials file
		opts := "credentials=/root/.smbcredentials,uid=1000,gid=1000"
		err = mount.MountCIFS(source, mountPoint, opts, persistent)
	}

	if err != nil {
		parent.message = fmt.Sprintf("Failed to mount: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("CIFS share mounted at '%s'", mountPoint)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitMountNFS(parent *model) (formModel, tea.Cmd) {
	source := fm.fields[0].input.Value()
	mountPoint := fm.fields[1].input.Value()
	persistent := fm.fields[2].checkValue

	if source == "" || mountPoint == "" {
		parent.message = "Source and mount point are required"
		parent.messageType = "error"
		return fm, nil
	}

	if err := mount.MountNFS(source, mountPoint, "defaults", persistent); err != nil {
		parent.message = fmt.Sprintf("Failed to mount: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("NFS share mounted at '%s'", mountPoint)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitMountUnmount(parent *model) (formModel, tea.Cmd) {
	mountPoint := fm.fields[0].input.Value()
	removePersistent := fm.fields[1].checkValue

	if mountPoint == "" {
		parent.message = "Mount point is required"
		parent.messageType = "error"
		return fm, nil
	}

	if err := mount.Unmount(mountPoint, removePersistent); err != nil {
		parent.message = fmt.Sprintf("Failed to unmount: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("Successfully unmounted '%s'", mountPoint)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitMountEdit(parent *model) (formModel, tea.Cmd) {
	mountPoint := fm.fields[0].input.Value()
	persistent := fm.fields[3].checkValue

	if mountPoint == "" {
		parent.message = "Mount point is required"
		parent.messageType = "error"
		return fm, nil
	}

	if err := mount.SetPersistent(mountPoint, persistent); err != nil {
		parent.message = fmt.Sprintf("Failed to update mount: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	status := "temporary"
	if persistent {
		status = "persistent"
	}
	parent.message = fmt.Sprintf("Mount '%s' is now %s", mountPoint, status)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) View() string {
	return fm.ViewWithMessage("", "")
}

func (fm formModel) ViewWithMessage(message, messageType string) string {
	var b strings.Builder

	// Title
	title := "Create Form"
	switch fm.formType {
	case "samba-create":
		title = "Create Samba Share"
	case "samba-modify":
		title = "Modify Samba Share"
	case "samba-remove":
		title = "Remove Samba Share"
	case "nfs-create":
		title = "Create NFS Export"
	case "nfs-modify":
		title = "Modify NFS Export"
	case "nfs-remove":
		title = "Remove NFS Export"
	case "mount-cifs":
		title = "Mount CIFS/SMB Share"
	case "mount-nfs":
		title = "Mount NFS Share"
	case "mount-unmount":
		title = "Unmount Network Share"
	case "mount-edit":
		title = "Edit Mount Settings"
	case "user-add":
		title = "Add User"
	case "user-password":
		title = "Change Password"
	case "user-remove":
		title = "Remove User"
	}

	b.WriteString(titleStyle.Render(title) + "\n\n")

	// Show message if present
	if message != "" {
		var messageStyle lipgloss.Style
		switch messageType {
		case "error":
			messageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF4444"}).
				Background(lipgloss.AdaptiveColor{Light: "#FFE6E6", Dark: "#3A1A1A"}).
				Bold(true).
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF4444"})
		case "success":
			messageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#00AA00", Dark: "#00FF00"}).
				Background(lipgloss.AdaptiveColor{Light: "#E6FFE6", Dark: "#1A3A1A"}).
				Bold(true).
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#00AA00", Dark: "#00FF00"})
		default:
			messageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#0088CC", Dark: "#00AAFF"}).
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#0088CC", Dark: "#00AAFF"})
		}
		b.WriteString(messageStyle.Render(" "+message+" ") + "\n\n")
	}

	// Form fields
	for i, field := range fm.fields {
		if field.displayOnly {
			// Display-only field (no input)
			b.WriteString(fmt.Sprintf("  %s\n", blurredStyle.Render(field.label)))
		} else if field.checkbox {
			// Checkbox field
			checkbox := checkboxUnchecked
			if field.checkValue {
				checkbox = checkboxChecked
			}

			cursor := " "
			if i == fm.focusIndex {
				cursor = "▶"
			}

			b.WriteString(fmt.Sprintf("%s %s %s\n", cursor, checkbox, field.label))
			if field.description != "" {
				b.WriteString(fmt.Sprintf("   %s\n", blurredStyle.Render(field.description)))
			}
		} else {
			// Text input field
			cursor := " "
			if i == fm.focusIndex {
				cursor = "▶"
			}

			b.WriteString(fmt.Sprintf("%s %s\n", cursor, field.label))
			if field.description != "" {
				b.WriteString(fmt.Sprintf("   %s\n", blurredStyle.Render(field.description)))
			}
			b.WriteString(fmt.Sprintf("   %s\n", field.input.View()))
		}
		b.WriteString("\n")
	}

	// Submit button
	button := blurredButton
	if fm.focusIndex == fm.submitIndex {
		button = focusedButton
	}
	b.WriteString(fmt.Sprintf("\n%s\n", button))

	b.WriteString("\n" + helpStyle.Render("Tab: Next field • Enter: Submit/Toggle • ESC: Cancel"))

	return b.String()
}
