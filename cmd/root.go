package cmd

import (
	"flag"
	"fmt"
	"os"
	"sambo/pkg/tui"
)

// Execute is the main entry point for the CLI
func Execute() error {
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
		fmt.Println("sambo v1.5.0 - Linux Share Management CLI")
		return nil
	}

	// Check if running as root for all other commands
	if os.Geteuid() != 0 {
		return fmt.Errorf("this tool must be run as root (use sudo)")
	}

	switch command {
	case "tui":
		return tui.Start()
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
	fmt.Println(`sambo - Linux Share Management CLI

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

func parseFlags(args []string) *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	return fs
}
