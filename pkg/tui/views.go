package tui

import (
	"fmt"
	"sambo/pkg/mount"
	"sambo/pkg/nfs"
	"sambo/pkg/user"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Table styles - Friendly Refresh & Spaced Out
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.AdaptiveColor{Light: "#818CF8", Dark: "#6366F1"}). // Indigo
				Padding(1, 2).                                                         // Taller header
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#C7D2FE", Dark: "#4F46E5"})

	tableRowStyle = lipgloss.NewStyle().
			Padding(0, 2). // More horizontal padding
			Foreground(lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#D1D5DB"})

	tableAltRowStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Background(lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#1F2937"}). // Gray-100/800
				Foreground(lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#D1D5DB"})

	tableBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}).
			Padding(1, 1). // Add padding inside the box
			MarginTop(0)

	emptyStateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}).
			Italic(true).
			Padding(2, 4).
			Align(lipgloss.Center).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"})
)

func (m model) viewSambaList() string {
	s := titleStyle.Render(" 📁 Samba Shares ") + "\n\n"

	shares, err := m.sambaManager.List()
	if err != nil {
		s += errorStyle.Render(fmt.Sprintf(" ✗ Error loading shares: %v ", err)) + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
		return s
	}

	if len(shares) == 0 {
		s += emptyStateStyle.Render("📭 No Samba shares configured yet.\n\nUse 'Create Share' to add your first share.") + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
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

	s += "\n" + helpBoxStyle.Render("Press ESC to go back")
	s += m.renderMessage()

	return s + "\n"
}

func (m model) viewNFSList() string {
	s := titleStyle.Render(" 🌐 NFS Exports ") + "\n\n"

	exports, err := nfs.List()
	if err != nil {
		s += errorStyle.Render(fmt.Sprintf(" ✗ Error loading exports: %v ", err)) + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
		return s
	}

	if len(exports) == 0 {
		s += emptyStateStyle.Render("📭 No NFS exports configured yet.\n\nUse 'Create Export' to add your first export.") + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
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

	s += "\n" + helpBoxStyle.Render("Press ESC to go back")
	s += m.renderMessage()

	return s + "\n"
}

func (m model) viewUserList() string {
	s := titleStyle.Render(" 👤 Samba Users ") + "\n\n"

	users, err := user.List()
	if err != nil {
		s += errorStyle.Render(fmt.Sprintf(" ✗ Error loading users: %v ", err)) + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
		return s
	}

	if len(users) == 0 {
		s += emptyStateStyle.Render("📭 No Samba users configured yet.\n\nUse 'Add User' to create your first user.") + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
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

	s += "\n" + helpBoxStyle.Render("Press ESC to go back")
	s += m.renderMessage()

	return s + "\n"
}

func (m model) viewMountList() string {
	s := titleStyle.Render(" 💾 Network Mounts ") + "\n\n"

	mounts, err := mount.List()
	if err != nil {
		s += errorStyle.Render(fmt.Sprintf(" ✗ Error loading mounts: %v ", err)) + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
		return s
	}

	if len(mounts) == 0 {
		s += emptyStateStyle.Render("📭 No network mounts found.\n\nUse 'Mount CIFS/SMB' or 'Mount NFS' to add mounts.") + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
		return s
	}

	// Table header
	s += tableHeaderStyle.Render(
		fmt.Sprintf("%-8s %-28s %-22s %-10s %-10s",
			"Type", "Source", "Mount Point", "Active", "Persistent"),
	) + "\n"

	// Table rows
	for i, mnt := range mounts {
		active := "No"
		if mnt.Active {
			active = "Yes"
		}
		persistent := "No"
		if mnt.Persistent {
			persistent = "Yes"
		}

		row := fmt.Sprintf("%-8s %-28s %-22s %-10s %-10s",
			strings.ToUpper(string(mnt.Type)),
			truncate(mnt.Source, 28),
			truncate(mnt.MountPoint, 22),
			active,
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

	s += "\n" + helpBoxStyle.Render("Press ESC to go back")
	s += m.renderMessage()

	return s + "\n"
}

func (m model) viewMountDiscover() string {
	s := titleStyle.Render(" 🔍 NFS Server Discovery ") + "\n\n"

	// Get local subnet
	subnet, err := mount.GetLocalSubnet()
	if err != nil {
		s += errorStyle.Render(fmt.Sprintf(" ✗ Failed to detect subnet: %v ", err)) + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
		return s
	}

	s += infoStyle.Render(fmt.Sprintf(" Scanning %s for NFS servers... ", subnet)) + "\n\n"

	// Perform discovery
	servers, err := mount.DiscoverNFSServers(subnet, 300*time.Millisecond)
	if err != nil {
		s += errorStyle.Render(fmt.Sprintf(" ✗ Scan failed: %v ", err)) + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
		return s
	}

	if len(servers) == 0 {
		s += emptyStateStyle.Render("📭 No NFS servers found on the network.\n\nMake sure NFS servers are running and accessible.") + "\n\n"
		s += helpBoxStyle.Render("Press ESC to go back")
		return s
	}

	s += successStyle.Render(fmt.Sprintf(" ✓ Found %d NFS server(s) ", len(servers))) + "\n\n"

	for _, server := range servers {
		// Server header
		s += tableHeaderStyle.Render(fmt.Sprintf(" Server: %s ", server.IP)) + "\n"

		// Exports
		for i, export := range server.Exports {
			clients := "*"
			if len(export.Clients) > 0 {
				clients = strings.Join(export.Clients, " ")
			}

			row := fmt.Sprintf("  %-35s → %s", truncate(export.Path, 35), clients)

			if i%2 == 0 {
				s += tableRowStyle.Render(row) + "\n"
			} else {
				s += tableAltRowStyle.Render(row) + "\n"
			}
		}
		s += "\n"
	}

	s += helpBoxStyle.Render("Press ESC to go back")

	return s + "\n"
}

// truncate shortens a string to maxLen runes, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string([]rune(s)[:maxLen])
	}
	return string([]rune(s)[:maxLen-3]) + "..."
}
