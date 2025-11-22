package cmd

import (
	"flag"
	"fmt"
	"os"
	"sambo/pkg/samba"
)

func handleSamba(args []string) error {
	if len(args) < 1 {
		printSambaUsage()
		return nil
	}

	subcommand := args[0]

	switch subcommand {
	case "list", "ls":
		return sambaList()
	case "create", "add":
		return sambaCreate(args[1:])
	case "remove", "rm", "delete":
		return sambaRemove(args[1:])
	case "modify", "update":
		return sambaModify(args[1:])
	case "show":
		return sambaShow(args[1:])
	case "-h", "--help", "help":
		printSambaUsage()
		return nil
	default:
		printSambaUsage()
		return fmt.Errorf("unknown samba subcommand: %s", subcommand)
	}
}

func sambaList() error {
	shares, err := samba.List()
	if err != nil {
		return fmt.Errorf("failed to list samba shares: %w", err)
	}

	if len(shares) == 0 {
		fmt.Println("No Samba shares configured")
		return nil
	}

	fmt.Println("Samba Shares:")
	fmt.Println("──────────────────────────────────────────────────────────")
	for _, share := range shares {
		fmt.Printf("Name:        %s\n", share.Name)
		fmt.Printf("Path:        %s\n", share.Path)
		fmt.Printf("Comment:     %s\n", share.Comment)
		fmt.Printf("Read Only:   %t\n", share.ReadOnly)
		fmt.Printf("Browseable:  %t\n", share.Browseable)
		if len(share.ValidUsers) > 0 {
			fmt.Printf("Valid Users: %v\n", share.ValidUsers)
		}
		fmt.Println("──────────────────────────────────────────────────────────")
	}

	return nil
}

func sambaCreate(args []string) error {
	fs := flag.NewFlagSet("samba create", flag.ExitOnError)
	name := fs.String("name", "", "Share name (required)")
	path := fs.String("path", "", "Share path (required)")
	comment := fs.String("comment", "", "Share description")
	readOnly := fs.Bool("readonly", false, "Make share read-only")
	browseable := fs.Bool("browseable", true, "Make share browseable")
	validUsers := fs.String("users", "", "Comma-separated list of valid users")

	fs.Parse(args)

	if *name == "" || *path == "" {
		printSambaUsage()
		return fmt.Errorf("name and path are required")
	}

	share := samba.Share{
		Name:       *name,
		Path:       *path,
		Comment:    *comment,
		ReadOnly:   *readOnly,
		Browseable: *browseable,
	}

	if *validUsers != "" {
		share.ValidUsers = parseCSV(*validUsers)
	}

	if err := samba.Create(share); err != nil {
		return fmt.Errorf("failed to create samba share: %w", err)
	}

	fmt.Printf("Samba share '%s' created successfully\n\n", *name)

	// Get hostname for mount examples
	hostname, _ := getHostname()

	fmt.Println("Mount this share on a client with:")
	fmt.Println("──────────────────────────────────────────────────────────")
	fmt.Printf("# Temporary mount:\n")
	if *validUsers != "" {
		fmt.Printf("sudo mount -t cifs //%s/%s /mnt/%s -o username=<user>,password=<pass>\n\n", hostname, *name, *name)
	} else {
		fmt.Printf("sudo mount -t cifs //%s/%s /mnt/%s -o guest\n\n", hostname, *name, *name)
	}

	fmt.Printf("# Or use sambo to mount:\n")
	if *validUsers != "" {
		fmt.Printf("sudo sambo mount cifs -source //%s/%s -mountpoint /mnt/%s -username <user> -password <pass>\n\n", hostname, *name, *name)
		fmt.Printf("# For persistent mount (survives reboot):\n")
		fmt.Printf("sudo sambo mount cifs -source //%s/%s -mountpoint /mnt/%s -username <user> -password <pass> -persistent\n", hostname, *name, *name)
	} else {
		fmt.Printf("sudo sambo mount cifs -source //%s/%s -mountpoint /mnt/%s -username guest -password \"\"\n\n", hostname, *name, *name)
		fmt.Printf("# For persistent mount (survives reboot):\n")
		fmt.Printf("sudo sambo mount cifs -source //%s/%s -mountpoint /mnt/%s -username guest -password \"\" -persistent\n", hostname, *name, *name)
	}
	fmt.Println("──────────────────────────────────────────────────────────")

	return nil
}

func sambaRemove(args []string) error {
	fs := flag.NewFlagSet("samba remove", flag.ExitOnError)
	name := fs.String("name", "", "Share name (required)")
	fs.Parse(args)

	if *name == "" {
		printSambaUsage()
		return fmt.Errorf("name is required")
	}

	if err := samba.Remove(*name); err != nil {
		return fmt.Errorf("failed to remove samba share: %w", err)
	}

	fmt.Printf("Samba share '%s' removed successfully\n", *name)
	return nil
}

func sambaModify(args []string) error {
	fs := flag.NewFlagSet("samba modify", flag.ExitOnError)
	name := fs.String("name", "", "Share name (required)")
	comment := fs.String("comment", "", "New share description")
	readOnly := fs.Bool("readonly", false, "Make share read-only")
	browseable := fs.Bool("browseable", true, "Make share browseable")
	validUsers := fs.String("users", "", "Comma-separated list of valid users")

	fs.Parse(args)

	if *name == "" {
		printSambaUsage()
		return fmt.Errorf("name is required")
	}

	updates := make(map[string]interface{})
	if *comment != "" {
		updates["comment"] = *comment
	}
	if fs.Lookup("readonly").Value.String() != "false" {
		updates["readonly"] = *readOnly
	}
	if fs.Lookup("browseable").Value.String() != "true" {
		updates["browseable"] = *browseable
	}
	if *validUsers != "" {
		updates["validusers"] = parseCSV(*validUsers)
	}

	if err := samba.Modify(*name, updates); err != nil {
		return fmt.Errorf("failed to modify samba share: %w", err)
	}

	fmt.Printf("Samba share '%s' modified successfully\n", *name)
	return nil
}

func sambaShow(args []string) error {
	fs := flag.NewFlagSet("samba show", flag.ExitOnError)
	name := fs.String("name", "", "Share name (required)")
	fs.Parse(args)

	if *name == "" {
		printSambaUsage()
		return fmt.Errorf("name is required")
	}

	share, err := samba.Get(*name)
	if err != nil {
		return fmt.Errorf("failed to get samba share: %w", err)
	}

	fmt.Printf("Name:        %s\n", share.Name)
	fmt.Printf("Path:        %s\n", share.Path)
	fmt.Printf("Comment:     %s\n", share.Comment)
	fmt.Printf("Read Only:   %t\n", share.ReadOnly)
	fmt.Printf("Browseable:  %t\n", share.Browseable)
	if len(share.ValidUsers) > 0 {
		fmt.Printf("Valid Users: %v\n", share.ValidUsers)
	}

	return nil
}

func printSambaUsage() {
	fmt.Println(`Samba Share Management

USAGE:
    sambo samba <subcommand> [options]

SUBCOMMANDS:
    list, ls                    List all Samba shares
    create, add                 Create a new Samba share
    remove, rm, delete          Remove a Samba share
    modify, update              Modify an existing Samba share
    show                        Show details of a specific share

CREATE/MODIFY OPTIONS:
    -name <name>                Share name
    -path <path>                Share directory path
    -comment <text>             Share description
    -readonly                   Make share read-only (default: false)
    -browseable                 Make share browseable (default: true)
    -users <user1,user2>        Comma-separated list of valid users

EXAMPLES:
    sambo samba list
    sambo samba create -name docs -path /mnt/documents -comment "Document Share"
    sambo samba create -name private -path /mnt/private -users alice,bob -readonly
    sambo samba modify -name docs -users alice,bob,charlie
    sambo samba remove -name docs
    sambo samba show -name docs`)
}

func parseCSV(s string) []string {
	if s == "" {
		return nil
	}
	result := []string{}
	current := ""
	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else if ch != ' ' {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
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
