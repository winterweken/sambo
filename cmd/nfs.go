package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"sambo/pkg/nfs"
)

func handleNFS(args []string) error {
	if len(args) < 1 {
		printNFSUsage()
		return nil
	}

	subcommand := args[0]

	switch subcommand {
	case "list", "ls":
		return nfsList()
	case "create", "add":
		return nfsCreate(args[1:])
	case "remove", "rm", "delete":
		return nfsRemove(args[1:])
	case "modify", "update":
		return nfsModify(args[1:])
	case "show":
		return nfsShow(args[1:])
	case "-h", "--help", "help":
		printNFSUsage()
		return nil
	default:
		printNFSUsage()
		return fmt.Errorf("unknown nfs subcommand: %s", subcommand)
	}
}

func nfsList() error {
	exports, err := nfs.List()
	if err != nil {
		return fmt.Errorf("failed to list nfs exports: %w", err)
	}

	if len(exports) == 0 {
		fmt.Println("No NFS exports configured")
		return nil
	}

	fmt.Println("NFS Exports:")
	fmt.Println("──────────────────────────────────────────────────────────")
	for _, export := range exports {
		fmt.Printf("Path:        %s\n", export.Path)
		fmt.Printf("Clients:     %s\n", export.Clients)
		fmt.Printf("Options:     %s\n", export.Options)
		fmt.Println("──────────────────────────────────────────────────────────")
	}

	return nil
}

func nfsCreate(args []string) error {
	fs := flag.NewFlagSet("nfs create", flag.ExitOnError)
	path := fs.String("path", "", "Export path (required)")
	clients := fs.String("clients", "*", "Client specification (IP, CIDR, or hostname)")
	readOnly := fs.Bool("readonly", false, "Make export read-only")
	sync := fs.Bool("sync", true, "Use sync mode (safer)")
	noRootSquash := fs.Bool("no-root-squash", false, "Disable root squashing")

	fs.Parse(args)

	if *path == "" {
		printNFSUsage()
		return fmt.Errorf("path is required")
	}

	export := nfs.Export{
		Path:    *path,
		Clients: *clients,
	}

	// Build options
	options := []string{}
	if *readOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}
	if *sync {
		options = append(options, "sync")
	} else {
		options = append(options, "async")
	}
	if *noRootSquash {
		options = append(options, "no_root_squash")
	} else {
		options = append(options, "root_squash")
	}
	options = append(options, "no_subtree_check")

	export.Options = joinStrings(options, ",")

	if err := nfs.Create(export); err != nil {
		return fmt.Errorf("failed to create nfs export: %w", err)
	}

	fmt.Printf("NFS export '%s' created successfully\n\n", *path)

	// Get hostname for mount examples
	hostname, _ := getHostname()

	fmt.Println("Mount this export on a client with:")
	fmt.Println("──────────────────────────────────────────────────────────")
	fmt.Printf("# Temporary mount:\n")
	fmt.Printf("sudo mount -t nfs %s:%s /mnt/nfs\n\n", hostname, *path)

	fmt.Printf("# Or use sambo to mount:\n")
	fmt.Printf("sudo sambo mount nfs -source %s:%s -mountpoint /mnt/nfs\n\n", hostname, *path)

	fmt.Printf("# For persistent mount (survives reboot):\n")
	fmt.Printf("sudo sambo mount nfs -source %s:%s -mountpoint /mnt/nfs -persistent\n", hostname, *path)
	fmt.Println("──────────────────────────────────────────────────────────")

	return nil
}

func nfsRemove(args []string) error {
	fs := flag.NewFlagSet("nfs remove", flag.ExitOnError)
	path := fs.String("path", "", "Export path (required)")
	fs.Parse(args)

	if *path == "" {
		printNFSUsage()
		return fmt.Errorf("path is required")
	}

	exportPath := *path

	// Remove the NFS export configuration
	if err := nfs.Remove(*path); err != nil {
		return fmt.Errorf("failed to remove nfs export: %w", err)
	}

	fmt.Printf("NFS export '%s' removed successfully\n", *path)

	// Ask if user wants to delete the folder and data
	fmt.Printf("\nThe export folder still exists at: %s\n", exportPath)
	fmt.Print("Do you want to delete this folder and all its data? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "y" || response == "yes" {
		// Confirm the permanent deletion
		fmt.Printf("\n⚠️  WARNING: This will permanently delete the folder and all data at:\n   %s\n", exportPath)
		fmt.Print("Are you absolutely sure? Type 'DELETE' to confirm: ")

		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(confirmation)

		if confirmation == "DELETE" {
			if err := os.RemoveAll(exportPath); err != nil {
				return fmt.Errorf("failed to delete folder: %w", err)
			}
			fmt.Printf("✓ Folder and data deleted: %s\n", exportPath)
		} else {
			fmt.Println("Deletion cancelled. Folder preserved.")
		}
	} else {
		fmt.Println("Folder preserved.")
	}

	return nil
}

func nfsModify(args []string) error {
	fs := flag.NewFlagSet("nfs modify", flag.ExitOnError)
	path := fs.String("path", "", "Export path (required)")
	clients := fs.String("clients", "", "New client specification")
	readOnly := fs.Bool("readonly", false, "Make export read-only")
	sync := fs.Bool("sync", true, "Use sync mode")
	noRootSquash := fs.Bool("no-root-squash", false, "Disable root squashing")

	fs.Parse(args)

	if *path == "" {
		printNFSUsage()
		return fmt.Errorf("path is required")
	}

	updates := make(map[string]interface{})
	if *clients != "" {
		updates["clients"] = *clients
	}

	// Rebuild options if any option flags were provided
	options := []string{}
	if *readOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}
	if *sync {
		options = append(options, "sync")
	} else {
		options = append(options, "async")
	}
	if *noRootSquash {
		options = append(options, "no_root_squash")
	} else {
		options = append(options, "root_squash")
	}
	options = append(options, "no_subtree_check")

	updates["options"] = joinStrings(options, ",")

	if err := nfs.Modify(*path, updates); err != nil {
		return fmt.Errorf("failed to modify nfs export: %w", err)
	}

	fmt.Printf("NFS export '%s' modified successfully\n", *path)
	return nil
}

func nfsShow(args []string) error {
	fs := flag.NewFlagSet("nfs show", flag.ExitOnError)
	path := fs.String("path", "", "Export path (required)")
	fs.Parse(args)

	if *path == "" {
		printNFSUsage()
		return fmt.Errorf("path is required")
	}

	export, err := nfs.Get(*path)
	if err != nil {
		return fmt.Errorf("failed to get nfs export: %w", err)
	}

	fmt.Printf("Path:        %s\n", export.Path)
	fmt.Printf("Clients:     %s\n", export.Clients)
	fmt.Printf("Options:     %s\n", export.Options)

	return nil
}

func printNFSUsage() {
	fmt.Println(`NFS Export Management

USAGE:
    sambo nfs <subcommand> [options]

SUBCOMMANDS:
    list, ls                    List all NFS exports
    create, add                 Create a new NFS export
    remove, rm, delete          Remove an NFS export
    modify, update              Modify an existing NFS export
    show                        Show details of a specific export

CREATE/MODIFY OPTIONS:
    -path <path>                Export directory path
    -clients <spec>             Client specification (IP, CIDR, hostname, or *)
    -readonly                   Make export read-only (default: false)
    -sync                       Use sync mode (default: true)
    -no-root-squash             Disable root squashing (default: false)

EXAMPLES:
    sambo nfs list
    sambo nfs create -path /mnt/backup -clients 192.168.1.0/24
    sambo nfs create -path /mnt/public -clients "*" -readonly
    sambo nfs modify -path /mnt/backup -clients 192.168.1.100
    sambo nfs remove -path /mnt/backup
    sambo nfs show -path /mnt/backup`)
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func getHostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "server", nil // fallback to generic name
	}
	return hostname, nil
}
