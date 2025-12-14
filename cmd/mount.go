package cmd

import (
	"flag"
	"fmt"
	"sambo/pkg/mount"
	"sambo/pkg/platform"
	"strings"
	"time"
)

func handleMount(args []string) error {
	if len(args) < 1 {
		printMountUsage()
		return nil
	}

	subcommand := args[0]

	switch subcommand {
	case "list", "ls":
		return mountList()
	case "cifs", "smb", "samba":
		return mountCIFS(args[1:])
	case "nfs":
		return mountNFS(args[1:])
	case "unmount", "umount", "remove", "rm":
		return mountUnmount(args[1:])
	case "discover", "scan":
		return mountDiscover(args[1:])
	case "exports":
		return mountExports(args[1:])
	case "-h", "--help", "help":
		printMountUsage()
		return nil
	default:
		printMountUsage()
		return fmt.Errorf("unknown mount subcommand: %s", subcommand)
	}
}

func mountList() error {
	mounts, err := mount.List()
	if err != nil {
		return fmt.Errorf("failed to list mounts: %w", err)
	}

	if len(mounts) == 0 {
		fmt.Println("No network mounts found")
		return nil
	}

	fmt.Println("Network Mounts:")
	fmt.Println("──────────────────────────────────────────────────────────")
	for _, m := range mounts {
		fmt.Printf("Type:        %s\n", strings.ToUpper(string(m.Type)))
		fmt.Printf("Source:      %s\n", m.Source)
		fmt.Printf("Mount Point: %s\n", m.MountPoint)
		fmt.Printf("Options:     %s\n", m.Options)
		if m.Active {
			fmt.Printf("Active:      Yes (currently mounted)\n")
		} else {
			fmt.Printf("Active:      No (not mounted)\n")
		}
		if m.Persistent {
			configFile := "/etc/fstab"
			if platform.IsMacOS() {
				configFile = "/etc/auto_nfs"
			}
			fmt.Printf("Persistent:  Yes (in %s)\n", configFile)
		} else {
			fmt.Printf("Persistent:  No (temporary)\n")
		}
		fmt.Println("──────────────────────────────────────────────────────────")
	}

	return nil
}

func mountCIFS(args []string) error {
	fs := flag.NewFlagSet("mount cifs", flag.ExitOnError)
	source := fs.String("source", "", "Source share (//server/share)")
	mountPoint := fs.String("mountpoint", "", "Local mount point (e.g., /mnt/share)")
	username := fs.String("username", "", "Username for authentication")
	password := fs.String("password", "", "Password for authentication")
	options := fs.String("options", "", "Additional mount options")
	persistent := fs.Bool("persistent", false, "Add to /etc/fstab for automatic mounting")

	fs.Parse(args)

	if *source == "" || *mountPoint == "" {
		printMountUsage()
		return fmt.Errorf("source and mountpoint are required")
	}

	// Build options string
	var opts []string
	if *options != "" {
		opts = append(opts, *options)
	}

	// Add credentials if provided
	if *username != "" {
		opts = append(opts, fmt.Sprintf("username=%s", *username))
		if *password != "" {
			opts = append(opts, fmt.Sprintf("password=%s", *password))
		}
	} else {
		// Default to credentials file
		opts = append(opts, "credentials=/root/.smbcredentials")
	}

	// Add default options
	if !contains(opts, "uid") {
		opts = append(opts, "uid=1000")
	}
	if !contains(opts, "gid") {
		opts = append(opts, "gid=1000")
	}

	optionsStr := strings.Join(opts, ",")

	if err := mount.MountCIFS(*source, *mountPoint, optionsStr, *persistent); err != nil {
		return fmt.Errorf("failed to mount CIFS share: %w", err)
	}

	fmt.Printf("CIFS share '%s' mounted at '%s'\n", *source, *mountPoint)
	if *persistent {
		fmt.Println("Added to /etc/fstab for automatic mounting at boot")
	}
	return nil
}

func mountNFS(args []string) error {
	fs := flag.NewFlagSet("mount nfs", flag.ExitOnError)
	source := fs.String("source", "", "Source share (server:/path)")
	mountPoint := fs.String("mountpoint", "", "Local mount point (e.g., /mnt/share)")
	options := fs.String("options", "", "Mount options (default: defaults)")
	persistent := fs.Bool("persistent", false, "Add to /etc/fstab for automatic mounting")

	fs.Parse(args)

	if *source == "" || *mountPoint == "" {
		printMountUsage()
		return fmt.Errorf("source and mountpoint are required")
	}

	// Use defaults if no options specified
	optionsStr := *options
	if optionsStr == "" {
		optionsStr = "defaults"
	}

	if err := mount.MountNFS(*source, *mountPoint, optionsStr, *persistent); err != nil {
		return fmt.Errorf("failed to mount NFS share: %w", err)
	}

	fmt.Printf("NFS share '%s' mounted at '%s'\n", *source, *mountPoint)
	if *persistent {
		fmt.Println("Added to /etc/fstab for automatic mounting at boot")
	}
	return nil
}

func mountUnmount(args []string) error {
	fs := flag.NewFlagSet("mount unmount", flag.ExitOnError)
	mountPoint := fs.String("mountpoint", "", "Mount point to unmount")
	removePersistent := fs.Bool("remove-persistent", false, "Also remove from /etc/fstab")

	fs.Parse(args)

	if *mountPoint == "" {
		printMountUsage()
		return fmt.Errorf("mountpoint is required")
	}

	if err := mount.Unmount(*mountPoint, *removePersistent); err != nil {
		return fmt.Errorf("failed to unmount: %w", err)
	}

	fmt.Printf("Unmounted '%s'\n", *mountPoint)
	if *removePersistent {
		fmt.Println("Removed from /etc/fstab")
	}
	return nil
}

func printMountUsage() {
	fmt.Println(`Network Mount Management

USAGE:
    sambo mount <subcommand> [options]

SUBCOMMANDS:
    list, ls                    List all network mounts
    cifs, smb, samba            Mount a CIFS/SMB share
    nfs                         Mount an NFS share
    unmount, umount, remove     Unmount a share
    discover, scan              Scan subnet for NFS servers
    exports                     List exports from an NFS server

CIFS/SMB MOUNT OPTIONS:
    -source <share>             Source share (//server/share)
    -mountpoint <path>          Local mount point
    -username <user>            Username (optional)
    -password <pass>            Password (optional)
    -options <opts>             Additional mount options
    -persistent                 Add to /etc/fstab

NFS MOUNT OPTIONS:
    -source <share>             Source share (server:/path)
    -mountpoint <path>          Local mount point
    -options <opts>             Mount options (default: defaults)
    -persistent                 Add to /etc/fstab

UNMOUNT OPTIONS:
    -mountpoint <path>          Mount point to unmount
    -remove-persistent          Also remove from /etc/fstab

EXAMPLES:
    # List all network mounts
    sambo mount list

    # Mount a CIFS share
    sambo mount cifs \
      -source //server/share \
      -mountpoint /mnt/share \
      -username alice \
      -password secret123

    # Mount a CIFS share persistently (survives reboot)
    sambo mount cifs \
      -source //192.168.1.100/backup \
      -mountpoint /mnt/backup \
      -persistent

    # Mount an NFS share
    sambo mount nfs \
      -source server:/export/data \
      -mountpoint /mnt/data

    # Mount an NFS share persistently
    sambo mount nfs \
      -source 192.168.1.100:/backups \
      -mountpoint /mnt/backups \
      -persistent

    # Unmount a share
    sambo mount unmount -mountpoint /mnt/share

    # Unmount and remove from fstab
    sambo mount unmount \
      -mountpoint /mnt/share \
      -remove-persistent

DISCOVERY COMMANDS:
    # Scan local subnet for NFS servers
    sambo mount discover

    # Scan a specific subnet
    sambo mount discover -subnet 192.168.10.0/24

    # List exports from a specific server
    sambo mount exports 192.168.10.199
    sambo mount exports -server myserver.local`)
}

func mountDiscover(args []string) error {
	fs := flag.NewFlagSet("mount discover", flag.ExitOnError)
	subnet := fs.String("subnet", "", "Subnet to scan (e.g., 192.168.1.0/24). Auto-detects if not specified.")
	timeout := fs.Int("timeout", 500, "Timeout per host in milliseconds")

	fs.Parse(args)

	// Auto-detect subnet if not specified
	subnetStr := *subnet
	if subnetStr == "" {
		detected, err := mount.GetLocalSubnet()
		if err != nil {
			return fmt.Errorf("failed to detect subnet: %w. Please specify with -subnet", err)
		}
		subnetStr = detected
		fmt.Printf("Auto-detected subnet: %s\n", subnetStr)
	}

	fmt.Printf("Scanning for NFS servers on %s...\n", subnetStr)

	servers, err := mount.DiscoverNFSServers(subnetStr, time.Duration(*timeout)*time.Millisecond)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(servers) == 0 {
		fmt.Println("No NFS servers found")
		return nil
	}

	fmt.Printf("\nFound %d NFS server(s):\n", len(servers))
	fmt.Println("──────────────────────────────────────────────────────────")

	for _, server := range servers {
		fmt.Printf("Server: %s\n", server.IP)
		for _, export := range server.Exports {
			clients := "*"
			if len(export.Clients) > 0 {
				clients = strings.Join(export.Clients, " ")
			}
			fmt.Printf("  %s → %s\n", export.Path, clients)
		}
		fmt.Println("──────────────────────────────────────────────────────────")
	}

	return nil
}

func mountExports(args []string) error {
	fs := flag.NewFlagSet("mount exports", flag.ExitOnError)
	server := fs.String("server", "", "Server IP or hostname to query")

	fs.Parse(args)

	if *server == "" {
		// Check if positional arg was provided
		if fs.NArg() > 0 {
			*server = fs.Arg(0)
		} else {
			return fmt.Errorf("server is required. Usage: sambo mount exports -server <ip> OR sambo mount exports <ip>")
		}
	}

	fmt.Printf("Querying NFS exports on %s...\n\n", *server)

	exports, err := mount.GetNFSExports(*server)
	if err != nil {
		return fmt.Errorf("failed to get exports: %w", err)
	}

	if len(exports) == 0 {
		fmt.Println("No NFS exports found")
		return nil
	}

	fmt.Printf("NFS Exports on %s:\n", *server)
	fmt.Println("──────────────────────────────────────────────────────────")
	fmt.Printf("%-35s %s\n", "PATH", "ALLOWED CLIENTS")
	fmt.Println("──────────────────────────────────────────────────────────")

	for _, export := range exports {
		clients := "*"
		if len(export.Clients) > 0 {
			clients = strings.Join(export.Clients, " ")
		}
		fmt.Printf("%-35s %s\n", export.Path, clients)
	}

	return nil
}

func contains(slice []string, substr string) bool {
	for _, item := range slice {
		if strings.Contains(item, substr) {
			return true
		}
	}
	return false
}
