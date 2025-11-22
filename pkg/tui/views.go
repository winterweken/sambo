package tui

import (
	"fmt"
	"sambo/pkg/mount"
	"sambo/pkg/nfs"
	"sambo/pkg/samba"
	"sambo/pkg/user"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7D56F4")).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(lipgloss.Color("#7D56F4"))

	tableRowStyle = lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1)

	tableAltRowStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				PaddingRight(1).
				Foreground(lipgloss.Color("#CCCCCC"))
)

func (m model) viewSambaList() string {
	s := titleStyle.Render("Samba Shares") + "\n\n"

	shares, err := samba.List()
	if err != nil {
		s += errorStyle.Render(fmt.Sprintf("Error loading shares: %v", err)) + "\n\n"
		s += helpStyle.Render("Press ESC to go back")
		return s
	}

	if len(shares) == 0 {
		s += "No Samba shares configured.\n\n"
		s += helpStyle.Render("Press ESC to go back")
		return s
	}

	// Table header
	s += tableHeaderStyle.Render(
		fmt.Sprintf("%-20s %-30s %-10s %-12s",
			"Name", "Path", "Read Only", "Browseable"),
	) + "\n"

	// Table rows
	for i, share := range shares {
		readOnly := "No"
		if share.ReadOnly {
			readOnly = "Yes"
		}
		browseable := "Yes"
		if !share.Browseable {
			browseable = "No"
		}

		row := fmt.Sprintf("%-20s %-30s %-10s %-12s",
			truncate(share.Name, 20),
			truncate(share.Path, 30),
			readOnly,
			browseable,
		)

		if i%2 == 0 {
			s += tableRowStyle.Render(row) + "\n"
		} else {
			s += tableAltRowStyle.Render(row) + "\n"
		}

		// Show valid users if any
		if len(share.ValidUsers) > 0 {
			users := fmt.Sprintf("  Users: %v", share.ValidUsers)
			s += lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Italic(true).
				Render(users) + "\n"
		}
	}

	s += "\n" + helpStyle.Render("ESC: Back to menu")

	return s + "\n"
}

func (m model) viewNFSList() string {
	s := titleStyle.Render("NFS Exports") + "\n\n"

	exports, err := nfs.List()
	if err != nil {
		s += errorStyle.Render(fmt.Sprintf("Error loading exports: %v", err)) + "\n\n"
		s += helpStyle.Render("Press ESC to go back")
		return s
	}

	if len(exports) == 0 {
		s += "No NFS exports configured.\n\n"
		s += helpStyle.Render("Press ESC to go back")
		return s
	}

	// Table header
	s += tableHeaderStyle.Render(
		fmt.Sprintf("%-30s %-20s %-30s",
			"Path", "Clients", "Options"),
	) + "\n"

	// Table rows
	for i, export := range exports {
		row := fmt.Sprintf("%-30s %-20s %-30s",
			truncate(export.Path, 30),
			truncate(export.Clients, 20),
			truncate(export.Options, 30),
		)

		if i%2 == 0 {
			s += tableRowStyle.Render(row) + "\n"
		} else {
			s += tableAltRowStyle.Render(row) + "\n"
		}
	}

	s += "\n" + helpStyle.Render("ESC: Back to menu")

	return s + "\n"
}

func (m model) viewUserList() string {
	s := titleStyle.Render("Samba Users") + "\n\n"

	users, err := user.List()
	if err != nil {
		s += errorStyle.Render(fmt.Sprintf("Error loading users: %v", err)) + "\n\n"
		s += helpStyle.Render("Press ESC to go back")
		return s
	}

	if len(users) == 0 {
		s += "No Samba users configured.\n\n"
		s += helpStyle.Render("Press ESC to go back")
		return s
	}

	// Table header
	s += tableHeaderStyle.Render(
		fmt.Sprintf("%-20s %-10s %-10s",
			"Username", "UID", "Status"),
	) + "\n"

	// Table rows
	for i, u := range users {
		status := "Enabled"
		if !u.Enabled {
			status = "Disabled"
		}

		row := fmt.Sprintf("%-20s %-10d %-10s",
			truncate(u.Username, 20),
			u.UID,
			status,
		)

		if i%2 == 0 {
			s += tableRowStyle.Render(row) + "\n"
		} else {
			s += tableAltRowStyle.Render(row) + "\n"
		}
	}

	s += "\n" + helpStyle.Render("ESC: Back to menu")

	return s + "\n"
}

func (m model) viewMountList() string {
	s := titleStyle.Render("Network Mounts") + "\n\n"

	mounts, err := mount.List()
	if err != nil {
		s += errorStyle.Render(fmt.Sprintf("Error loading mounts: %v", err)) + "\n\n"
		s += helpStyle.Render("Press ESC to go back")
		return s
	}

	if len(mounts) == 0 {
		s += "No network mounts found.\n\n"
		s += helpStyle.Render("Press ESC to go back")
		return s
	}

	// Table header
	s += tableHeaderStyle.Render(
		fmt.Sprintf("%-8s %-30s %-25s %-12s",
			"Type", "Source", "Mount Point", "Persistent"),
	) + "\n"

	// Table rows
	for i, mnt := range mounts {
		persistent := "No"
		if mnt.Persistent {
			persistent = "Yes"
		}

		row := fmt.Sprintf("%-8s %-30s %-25s %-12s",
			strings.ToUpper(string(mnt.Type)),
			truncate(mnt.Source, 30),
			truncate(mnt.MountPoint, 25),
			persistent,
		)

		if i%2 == 0 {
			s += tableRowStyle.Render(row) + "\n"
		} else {
			s += tableAltRowStyle.Render(row) + "\n"
		}

		// Show options
		if mnt.Options != "" {
			opts := fmt.Sprintf("  Options: %s", truncate(mnt.Options, 70))
			s += lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Italic(true).
				Render(opts) + "\n"
		}
	}

	s += "\n" + helpStyle.Render("ESC: Back to menu")

	return s + "\n"
}

// Helper function to truncate strings
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
