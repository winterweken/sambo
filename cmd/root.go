package cmd

import (
	"fmt"
	"os"
	"sambo/pkg/samba"
	"sambo/pkg/system"
	"sambo/pkg/tui"
)

// Version is set from main.go at startup (injected via ldflags)
var Version = "dev"

var (
	// Global managers
	sambaManager *samba.Manager
)

// Execute is the main entry point for the CLI
func Execute() error {
	// Initialize system interfaces
	fs := &system.RealFileSystem{}
	exec := &system.RealCommandExecutor{}
	plat := &system.RealPlatform{}
	avahi := &samba.RealAvahiManager{}

	// Initialize managers
	sambaManager = samba.NewManager(fs, exec, plat, avahi)

	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	command := os.Args[1]

	// Allow help and version without root
	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "version", "-v", "--version":
		fmt.Printf("sambo %s - Share Management CLI for Linux and macOS\n", Version)
		return nil
	}

	// Mount subcommands that don't require root
	if command == "mount" && len(os.Args) > 2 {
		subCmd := os.Args[2]
		if subCmd == "list" || subCmd == "ls" || subCmd == "discover" || subCmd == "scan" || subCmd == "exports" ||
			subCmd == "-h" || subCmd == "--help" || subCmd == "help" {
			return handleMount(os.Args[2:])
		}
	}

	// Check if running as root for all other commands
	if os.Geteuid() != 0 {
		return fmt.Errorf("this tool must be run as root (use sudo)")
	}

	switch command {
	case "tui":
		return tui.Start(sambaManager)
	case "samba":
		return handleSamba(os.Args[2:])
	case "nfs":
		return handleNFS(os.Args[2:])
	case "mount":
		return handleMount(os.Args[2:])
	case "user":
		return handleUser(os.Args[2:])
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Println(`sambo - Share Management CLI for Linux and macOS

USAGE:
    sambo <command> [options]

COMMANDS:
    tui         Launch interactive menu (recommended)
    samba       Manage Samba (SMB/CIFS) shares
    nfs         Manage NFS shares
    mount       Mount and manage network shares (client-side)
    user        Manage share users and permissions
    help        Show this help message
    version     Show version information

Run 'sambo <command> -h' for more information on a specific command.

EXAMPLES:
    sudo sambo tui                    # Launch interactive menu
    sambo samba list
    sambo samba create -name myshare -path /mnt/data
    sambo nfs create -name backup -path /mnt/backup -clients 192.168.1.0/24
    sambo mount cifs -source //server/share -mountpoint /mnt/share
    sambo user add -username john -password secret`)
}

