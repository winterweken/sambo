package cmd

import (
	"flag"
	"fmt"

	"sambo/pkg/confirm"
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

	// Build options using shared helper
	// Note: sync flag is inverted — CLI uses -sync=true (default), BuildOptions takes asyncMode
	export.Options = nfs.BuildOptions(*readOnly, !*sync, *noRootSquash)

	if err := nfs.Create(export); err != nil {
		return fmt.Errorf("failed to create nfs export: %w", err)
	}

	fmt.Printf("NFS export '%s' created successfully\n", *path)
	return nil
}

func nfsRemove(args []string) error {
	fs := flag.NewFlagSet("nfs remove", flag.ExitOnError)
	path := fs.String("path", "", "Export path (required)")
	yes := fs.Bool("y", false, "Skip confirmation prompt")
	fs.Parse(args)

	if *path == "" {
		printNFSUsage()
		return fmt.Errorf("path is required")
	}

	// Confirm deletion
	if !confirm.Action(fmt.Sprintf("Remove NFS export '%s'?", *path), *yes) {
		fmt.Println("Cancelled")
		return nil
	}

	if err := nfs.Remove(*path); err != nil {
		return fmt.Errorf("failed to remove nfs export: %w", err)
	}

	fmt.Printf("NFS export '%s' removed successfully\n", *path)
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

	// Rebuild options using shared helper
	updates["options"] = nfs.BuildOptions(*readOnly, !*sync, *noRootSquash)

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

