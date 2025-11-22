package cmd

import (
	"flag"
	"fmt"
	"sambo/pkg/user"
)

func handleUser(args []string) error {
	if len(args) < 1 {
		printUserUsage()
		return nil
	}

	subcommand := args[0]

	switch subcommand {
	case "list", "ls":
		return userList()
	case "add", "create":
		return userAdd(args[1:])
	case "remove", "rm", "delete":
		return userRemove(args[1:])
	case "passwd", "password":
		return userPassword(args[1:])
	case "show":
		return userShow(args[1:])
	case "-h", "--help", "help":
		printUserUsage()
		return nil
	default:
		printUserUsage()
		return fmt.Errorf("unknown user subcommand: %s", subcommand)
	}
}

func userList() error {
	users, err := user.List()
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	if len(users) == 0 {
		fmt.Println("No Samba users configured")
		return nil
	}

	fmt.Println("Samba Users:")
	fmt.Println("──────────────────────────────────────────────────────────")
	for _, u := range users {
		fmt.Printf("Username:    %s\n", u.Username)
		fmt.Printf("UID:         %d\n", u.UID)
		fmt.Printf("Enabled:     %t\n", u.Enabled)
		fmt.Println("──────────────────────────────────────────────────────────")
	}

	return nil
}

func userAdd(args []string) error {
	fs := flag.NewFlagSet("user add", flag.ExitOnError)
	username := fs.String("username", "", "Username (required)")
	password := fs.String("password", "", "Password (required)")
	createSystem := fs.Bool("create-system", true, "Create system user if not exists")

	fs.Parse(args)

	if *username == "" || *password == "" {
		printUserUsage()
		return fmt.Errorf("username and password are required")
	}

	if err := user.Add(*username, *password, *createSystem); err != nil {
		return fmt.Errorf("failed to add user: %w", err)
	}

	fmt.Printf("User '%s' added successfully\n", *username)
	return nil
}

func userRemove(args []string) error {
	fs := flag.NewFlagSet("user remove", flag.ExitOnError)
	username := fs.String("username", "", "Username (required)")
	removeSystem := fs.Bool("remove-system", false, "Also remove system user")

	fs.Parse(args)

	if *username == "" {
		printUserUsage()
		return fmt.Errorf("username is required")
	}

	if err := user.Remove(*username, *removeSystem); err != nil {
		return fmt.Errorf("failed to remove user: %w", err)
	}

	fmt.Printf("User '%s' removed successfully\n", *username)
	return nil
}

func userPassword(args []string) error {
	fs := flag.NewFlagSet("user password", flag.ExitOnError)
	username := fs.String("username", "", "Username (required)")
	password := fs.String("password", "", "New password (required)")

	fs.Parse(args)

	if *username == "" || *password == "" {
		printUserUsage()
		return fmt.Errorf("username and password are required")
	}

	if err := user.SetPassword(*username, *password); err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	fmt.Printf("Password for user '%s' updated successfully\n", *username)
	return nil
}

func userShow(args []string) error {
	fs := flag.NewFlagSet("user show", flag.ExitOnError)
	username := fs.String("username", "", "Username (required)")
	fs.Parse(args)

	if *username == "" {
		printUserUsage()
		return fmt.Errorf("username is required")
	}

	u, err := user.Get(*username)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	fmt.Printf("Username:    %s\n", u.Username)
	fmt.Printf("UID:         %d\n", u.UID)
	fmt.Printf("Enabled:     %t\n", u.Enabled)

	return nil
}

func printUserUsage() {
	fmt.Println(`User Management

USAGE:
    sambo user <subcommand> [options]

SUBCOMMANDS:
    list, ls                    List all Samba users
    add, create                 Add a new Samba user
    remove, rm, delete          Remove a Samba user
    passwd, password            Change user password
    show                        Show details of a specific user

ADD OPTIONS:
    -username <name>            Username (required)
    -password <pass>            Password (required)
    -create-system              Create system user if not exists (default: true)

REMOVE OPTIONS:
    -username <name>            Username (required)
    -remove-system              Also remove system user (default: false)

PASSWORD OPTIONS:
    -username <name>            Username (required)
    -password <pass>            New password (required)

EXAMPLES:
    sambo user list
    sambo user add -username alice -password secret123
    sambo user passwd -username alice -password newsecret456
    sambo user remove -username alice
    sambo user show -username alice`)
}
