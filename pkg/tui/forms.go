package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"sambo/pkg/nfs"
	"sambo/pkg/samba"
	"sambo/pkg/user"
)

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	cursorStyle  = focusedStyle.Copy()
	noStyle      = lipgloss.NewStyle()

	focusedButton = focusedStyle.Copy().Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))

	checkboxChecked   = focusedStyle.Render("[✓]")
	checkboxUnchecked = blurredStyle.Render("[ ]")
)

type formField struct {
	label       string
	input       textinput.Model
	checkbox    bool
	checkValue  bool
	description string
}

type formModel struct {
	fields        []formField
	focusIndex    int
	submitIndex   int
	formType      string // "samba-create", "nfs-create", "user-add", etc.
	timeMachine   bool
	returnScreen  screen
}

func newSambaCreateForm() formModel {
	inputs := make([]formField, 5)

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
	inputs := make([]formField, 2)

	inputs[0] = formField{
		label:       "Export Path",
		input:       makeInput("/mnt/data", "Path to export (required)"),
		description: "Directory path to export",
	}
	inputs[1] = formField{
		label:       "Clients",
		input:       makeInput("*", "IP, CIDR, or * for all"),
		description: "Client access specification",
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
				if i == fm.focusIndex && !fm.fields[i].checkbox {
					fm.fields[i].input.Focus()
					fm.fields[i].input.PromptStyle = focusedStyle
					fm.fields[i].input.TextStyle = focusedStyle
				} else if !fm.fields[i].checkbox {
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
	if fm.focusIndex < len(fm.fields) && !fm.fields[fm.focusIndex].checkbox {
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
	case "samba-remove":
		return fm.submitSambaRemove(parent)
	case "nfs-create":
		return fm.submitNFSCreate(parent)
	case "nfs-remove":
		return fm.submitNFSRemove(parent)
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

	if name == "" || path == "" {
		parent.message = "Name and path are required"
		parent.messageType = "error"
		return fm, nil
	}

	share := samba.Share{
		Name:         name,
		Path:         path,
		Comment:      comment,
		ReadOnly:     false,
		Browseable:   true,
		TimeMachine:  fm.timeMachine,
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

	parent.message = fmt.Sprintf("Share '%s' created successfully", name)
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

	parent.message = fmt.Sprintf("Share '%s' removed successfully", name)
	parent.messageType = "success"
	parent.currentScreen = fm.returnScreen
	parent.inForm = false
	return fm, nil
}

func (fm formModel) submitNFSCreate(parent *model) (formModel, tea.Cmd) {
	path := fm.fields[0].input.Value()
	clients := fm.fields[1].input.Value()

	if path == "" {
		parent.message = "Path is required"
		parent.messageType = "error"
		return fm, nil
	}

	export := nfs.Export{
		Path:    path,
		Clients: clients,
		Options: "rw,sync,no_subtree_check",
	}

	if err := nfs.Create(export); err != nil {
		parent.message = fmt.Sprintf("Failed to create export: %v", err)
		parent.messageType = "error"
		return fm, nil
	}

	parent.message = fmt.Sprintf("Export '%s' created successfully", path)
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

	parent.message = fmt.Sprintf("Export '%s' removed successfully", path)
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

func (fm formModel) View() string {
	var b strings.Builder

	// Title
	title := "Create Form"
	switch fm.formType {
	case "samba-create":
		title = "Create Samba Share"
	case "samba-remove":
		title = "Remove Samba Share"
	case "nfs-create":
		title = "Create NFS Export"
	case "nfs-remove":
		title = "Remove NFS Export"
	case "user-add":
		title = "Add User"
	case "user-password":
		title = "Change Password"
	case "user-remove":
		title = "Remove User"
	}

	b.WriteString(titleStyle.Render(title) + "\n\n")

	// Form fields
	for i, field := range fm.fields {
		if field.checkbox {
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
