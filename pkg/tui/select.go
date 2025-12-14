package tui

import (
	"fmt"
	"strings"
	"sambo/pkg/mount"
	"sambo/pkg/nfs"
	"sambo/pkg/samba"

	tea "github.com/charmbracelet/bubbletea"
)

type selectModel struct {
	items        []string
	cursor       int
	selectType   string // "samba-modify", "nfs-modify", "mount-unmount"
	returnScreen screen
	nextScreen   screen
}

func newSambaModifySelect() (selectModel, error) {
	shares, err := samba.List()
	if err != nil {
		return selectModel{}, err
	}

	items := make([]string, len(shares))
	for i, share := range shares {
		items[i] = share.Name
	}

	return selectModel{
		items:        items,
		cursor:       0,
		selectType:   "samba-modify",
		returnScreen: screenSambaMenu,
		nextScreen:   screenSambaModify,
	}, nil
}

func newNFSModifySelect() (selectModel, error) {
	exports, err := nfs.List()
	if err != nil {
		return selectModel{}, err
	}

	items := make([]string, len(exports))
	for i, export := range exports {
		items[i] = export.Path
	}

	return selectModel{
		items:        items,
		cursor:       0,
		selectType:   "nfs-modify",
		returnScreen: screenNFSMenu,
		nextScreen:   screenNFSModify,
	}, nil
}

func newMountUnmountSelect() (selectModel, error) {
	mounts, err := mount.List()
	if err != nil {
		return selectModel{}, err
	}

	items := make([]string, len(mounts))
	for i, mnt := range mounts {
		items[i] = fmt.Sprintf("%s (from %s)", mnt.MountPoint, mnt.Source)
	}

	return selectModel{
		items:        items,
		cursor:       0,
		selectType:   "mount-unmount",
		returnScreen: screenMountMenu,
		nextScreen:   screenMountUnmount,
	}, nil
}

func newMountEditSelect() (selectModel, error) {
	mounts, err := mount.List()
	if err != nil {
		return selectModel{}, err
	}

	items := make([]string, len(mounts))
	for i, mnt := range mounts {
		persistent := "temporary"
		if mnt.Persistent {
			persistent = "persistent"
		}
		items[i] = fmt.Sprintf("%s [%s] (from %s)", mnt.MountPoint, persistent, mnt.Source)
	}

	return selectModel{
		items:        items,
		cursor:       0,
		selectType:   "mount-edit",
		returnScreen: screenMountMenu,
		nextScreen:   screenMountEdit,
	}, nil
}

func (sm selectModel) Init() tea.Cmd {
	return nil
}

func (sm selectModel) Update(msg tea.Msg, parent *model) (selectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			parent.currentScreen = sm.returnScreen
			parent.inSelect = false
			parent.message = ""
			return sm, nil

		case "up", "k":
			if sm.cursor > 0 {
				sm.cursor--
			}

		case "down", "j":
			if sm.cursor < len(sm.items)-1 {
				sm.cursor++
			}

		case "enter":
			if len(sm.items) == 0 {
				return sm, nil
			}

			// User selected an item, create modify form
			selectedItem := sm.items[sm.cursor]

			switch sm.selectType {
			case "samba-modify":
				form, err := newSambaModifyForm(selectedItem)
				if err != nil {
					parent.message = fmt.Sprintf("Failed to load share: %v", err)
					parent.messageType = "error"
					parent.currentScreen = sm.returnScreen
					parent.inSelect = false
					return sm, nil
				}
				parent.form = &form
				parent.inForm = true
				parent.inSelect = false
				parent.currentScreen = sm.nextScreen
				return sm, parent.form.Init()

			case "nfs-modify":
				form, err := newNFSModifyForm(selectedItem)
				if err != nil {
					parent.message = fmt.Sprintf("Failed to load export: %v", err)
					parent.messageType = "error"
					parent.currentScreen = sm.returnScreen
					parent.inSelect = false
					return sm, nil
				}
				parent.form = &form
				parent.inForm = true
				parent.inSelect = false
				parent.currentScreen = sm.nextScreen
				return sm, parent.form.Init()

			case "mount-unmount":
				// Extract mount point from the display string (format: "mountpoint (from source)")
				mountPoint := selectedItem
				if idx := strings.Index(selectedItem, " (from "); idx != -1 {
					mountPoint = selectedItem[:idx]
				}
				form := newMountUnmountFormWithPath(mountPoint)
				parent.form = &form
				parent.inForm = true
				parent.inSelect = false
				parent.currentScreen = sm.nextScreen
				return sm, parent.form.Init()

			case "mount-edit":
				// Extract mount point from the display string (format: "mountpoint [persistent/temporary] (from source)")
				mountPoint := selectedItem
				if idx := strings.Index(selectedItem, " ["); idx != -1 {
					mountPoint = selectedItem[:idx]
				}
				form, err := newMountEditForm(mountPoint)
				if err != nil {
					parent.message = fmt.Sprintf("Failed to load mount: %v", err)
					parent.messageType = "error"
					parent.currentScreen = sm.returnScreen
					parent.inSelect = false
					return sm, nil
				}
				parent.form = &form
				parent.inForm = true
				parent.inSelect = false
				parent.currentScreen = sm.nextScreen
				return sm, parent.form.Init()
			}
		}
	}

	return sm, nil
}

func (sm selectModel) View() string {
	var s string

	// Title
	title := "Select Item to Modify"
	switch sm.selectType {
	case "samba-modify":
		title = "Select Samba Share to Modify"
	case "nfs-modify":
		title = "Select NFS Export to Modify"
	case "mount-unmount":
		title = "Select Mount to Unmount"
	case "mount-edit":
		title = "Select Mount to Edit"
	}

	s += titleStyle.Render(title) + "\n\n"

	if len(sm.items) == 0 {
		s += "No items available.\n\n"
		s += helpStyle.Render("Press ESC to go back")
		return s
	}

	// List items
	for i, item := range sm.items {
		cursor := " "
		if i == sm.cursor {
			cursor = "▶"
			s += selectedStyle.Render(fmt.Sprintf("%s %s", cursor, item)) + "\n"
		} else {
			s += menuStyle.Render(fmt.Sprintf("%s %s", cursor, item)) + "\n"
		}
	}

	s += "\n" + helpStyle.Render("↑/↓: Navigate • Enter: Select • ESC: Back")

	return s + "\n"
}
