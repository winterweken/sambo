package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"sambo/pkg/system"
)

type installMsg struct {
	component string
	success   bool
	err       error
}

func installComponent(component string) tea.Cmd {
	return func() tea.Msg {
		var err error
		pm := system.DetectPackageManager()

		switch component {
		case "samba":
			packages := system.GetSambaPackages(pm)
			err = system.InstallPackages(packages)
			if err == nil {
				// Enable and start the service
				serviceName := system.GetSambaServiceName()
				err = system.EnableService(serviceName)
			}

		case "nfs":
			packages := system.GetNFSPackages(pm)
			err = system.InstallPackages(packages)
			if err == nil {
				serviceName := system.GetNFSServiceName()
				err = system.EnableService(serviceName)
			}

		case "cifs":
			packages := system.GetCIFSPackages(pm)
			err = system.InstallPackages(packages)

		case "nfs-client":
			packages := system.GetNFSClientPackages(pm)
			err = system.InstallPackages(packages)
		}

		return installMsg{
			component: component,
			success:   err == nil,
			err:       err,
		}
	}
}

func (m model) viewDependencies() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render(" System Dependencies ") + "\n\n")

	// Show spinner if installing
	if m.isInstalling {
		s.WriteString("\n")
		s.WriteString(infoStyle.Render(fmt.Sprintf(" %s %s ", m.spinner.View(), m.installMsg)))
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("Please wait while packages are being installed..."))
		return s.String() + "\n"
	}

	// Detect package manager
	pm := system.DetectPackageManager()
	pmName := string(pm)
	if pm == system.UNKNOWN {
		pmName = "Unknown"
	}

	s.WriteString(infoStyle.Render(fmt.Sprintf(" Package Manager: %s ", pmName)) + "\n\n")

	// Check status of all components
	components := []struct {
		name        string
		displayName string
		installed   bool
		packages    []string
	}{
		{
			name:        "samba",
			displayName: "Samba Server (for sharing files via SMB/CIFS)",
			installed:   system.IsSambaInstalled(),
			packages:    system.GetSambaPackages(pm),
		},
		{
			name:        "nfs",
			displayName: "NFS Server (for sharing files via NFS)",
			installed:   system.IsNFSInstalled(),
			packages:    system.GetNFSPackages(pm),
		},
		{
			name:        "cifs",
			displayName: "CIFS Client (for mounting SMB/CIFS shares)",
			installed:   system.IsCIFSInstalled(),
			packages:    system.GetCIFSPackages(pm),
		},
		{
			name:        "nfs-client",
			displayName: "NFS Client (for mounting NFS shares)",
			installed:   system.IsNFSClientInstalled(),
			packages:    system.GetNFSClientPackages(pm),
		},
	}

	// Menu items
	menuItems := make([]string, 0)
	for _, comp := range components {
		status := "❌ Not Installed"
		if comp.installed {
			status = "✅ Installed"
		}
		pkgList := strings.Join(comp.packages, ", ")
		menuItems = append(menuItems, fmt.Sprintf("%s - %s (%s)", status, comp.displayName, pkgList))
	}
	menuItems = append(menuItems, "⬅️  Back to Main Menu")

	// Render menu
	boxContent := ""
	for i, item := range menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
			boxContent += selectedStyle.Render(cursor+" "+item) + "\n"
		} else {
			boxContent += menuStyle.Render(cursor+" "+item) + "\n"
		}
	}

	s.WriteString(menuBoxStyle.Render(boxContent))
	s.WriteString("\n")

	// Instructions
	if m.cursor < len(components) && !components[m.cursor].installed {
		s.WriteString(helpBoxStyle.Render("↑/↓ or j/k: Navigate • Enter: Install Selected • ESC: Back • Q: Main Menu"))
	} else if m.cursor < len(components) && components[m.cursor].installed {
		s.WriteString(helpBoxStyle.Render("↑/↓ or j/k: Navigate • ESC: Back • Q: Main Menu (Already Installed)"))
	} else {
		s.WriteString(helpBoxStyle.Render("↑/↓ or j/k: Navigate • Enter: Go Back • ESC: Back • Q: Main Menu"))
	}

	// Warning for unknown package manager
	if pm == system.UNKNOWN {
		s.WriteString("\n\n")
		s.WriteString(warningStyle.Render(" ⚠ Unable to detect package manager. Manual installation required. "))
	}

	s.WriteString(m.renderMessage())

	return s.String() + "\n"
}

func (m model) handleDependenciesEnter() (tea.Model, tea.Cmd) {
	pm := system.DetectPackageManager()

	components := []struct {
		name      string
		installed bool
	}{
		{name: "samba", installed: system.IsSambaInstalled()},
		{name: "nfs", installed: system.IsNFSInstalled()},
		{name: "cifs", installed: system.IsCIFSInstalled()},
		{name: "nfs-client", installed: system.IsNFSClientInstalled()},
	}

	// Back option
	if m.cursor == len(components) {
		m.currentScreen = screenMainMenu
		m.cursor = 0
		m.message = ""
		return m, nil
	}

	// Can't install if package manager is unknown
	if pm == system.UNKNOWN {
		m.message = "Cannot install: Unknown package manager"
		m.messageType = "error"
		return m, nil
	}

	// Already installed
	if components[m.cursor].installed {
		m.message = "This component is already installed"
		m.messageType = "info"
		return m, nil
	}

	// Install the component
	component := components[m.cursor].name
	m.isInstalling = true
	m.installMsg = fmt.Sprintf("Installing %s...", component)
	m.message = ""

	return m, tea.Batch(m.spinner.Tick, installComponent(component))
}

func (m model) handleInstallMsg(msg installMsg) (tea.Model, tea.Cmd) {
	if msg.success {
		m.message = fmt.Sprintf("✅ Successfully installed %s", msg.component)
		m.messageType = "success"
	} else {
		m.message = fmt.Sprintf("Failed to install %s: %v", msg.component, msg.err)
		m.messageType = "error"
	}
	return m, nil
}
