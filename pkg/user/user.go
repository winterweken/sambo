package user

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"sambo/pkg/platform"
	"sambo/pkg/validate"
)

// User represents a Samba user
type User struct {
	Username string
	UID      int
	Enabled  bool
}

// List returns all Samba users
func List() ([]User, error) {
	cmd := exec.Command("pdbedit", "-L", "-v")
	output, err := cmd.Output()
	if err != nil {
		// If pdbedit fails, it might be that there are no users
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return []User{}, nil
		}
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	var users []User
	lines := strings.Split(string(output), "\n")
	var currentUser *User

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		// Parse pdbedit output
		if strings.HasPrefix(line, "Unix username:") {
			if currentUser != nil {
				users = append(users, *currentUser)
			}
			currentUser = &User{
				Enabled: true, // default
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentUser.Username = strings.TrimSpace(parts[1])
			}
		} else if currentUser != nil && strings.HasPrefix(line, "User SID:") {
			// Could parse SID if needed
		} else if currentUser != nil && strings.Contains(line, "Account Flags:") {
			if strings.Contains(line, "D") {
				currentUser.Enabled = false
			}
		}
	}

	if currentUser != nil {
		users = append(users, *currentUser)
	}

	// Get UIDs from system
	for i := range users {
		uid, err := getSystemUID(users[i].Username)
		if err == nil {
			users[i].UID = uid
		}
	}

	return users, nil
}

// Get retrieves a specific user
func Get(username string) (*User, error) {
	users, err := List()
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if user.Username == username {
			return &user, nil
		}
	}

	return nil, fmt.Errorf("user '%s' not found", username)
}

// Add creates a new Samba user
func Add(username, password string, createSystem bool) error {
	// Validate username
	if err := validate.Username(username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}

	// Validate password
	if err := validate.Password(password); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	// Check if user already exists
	existing, _ := Get(username)
	if existing != nil {
		return fmt.Errorf("user '%s' already exists", username)
	}

	// Create system user if requested
	if createSystem {
		if err := createSystemUser(username); err != nil {
			return fmt.Errorf("failed to create system user: %w", err)
		}
	} else {
		// Check if system user exists
		if _, err := getSystemUID(username); err != nil {
			return fmt.Errorf("system user '%s' does not exist. Use -create-system flag or create manually", username)
		}
	}

	// Add Samba user
	cmd := exec.Command("smbpasswd", "-a", username)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start smbpasswd: %w", err)
	}

	// Write password twice
	fmt.Fprintf(stdin, "%s\n%s\n", password, password)
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to add samba user: %w", err)
	}

	// Enable the user
	cmd = exec.Command("smbpasswd", "-e", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable user: %w", err)
	}

	return nil
}

// Remove deletes a Samba user
func Remove(username string, removeSystem bool) error {
	// Check if user exists
	_, err := Get(username)
	if err != nil {
		return err
	}

	// Remove Samba user
	cmd := exec.Command("smbpasswd", "-x", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove samba user: %w", err)
	}

	// Remove system user if requested
	if removeSystem {
		if err := removeSystemUser(username); err != nil {
			return fmt.Errorf("failed to remove system user: %w", err)
		}
	}

	return nil
}

// SetPassword changes a user's password
func SetPassword(username, password string) error {
	// Validate password
	if err := validate.Password(password); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	// Check if user exists
	_, err := Get(username)
	if err != nil {
		return err
	}

	// Change password
	cmd := exec.Command("smbpasswd", username)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start smbpasswd: %w", err)
	}

	// Write password twice
	fmt.Fprintf(stdin, "%s\n%s\n", password, password)
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	return nil
}

// Helper functions

func createSystemUser(username string) error {
	// Check if user already exists
	if _, err := getSystemUID(username); err == nil {
		return nil // User already exists
	}

	if platform.IsMacOS() {
		return createSystemUserMacOS(username)
	}
	return createSystemUserLinux(username)
}

func createSystemUserLinux(username string) error {
	// Create user with no login shell and no home directory
	cmd := exec.Command("useradd", "-M", "-s", "/usr/sbin/nologin", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func createSystemUserMacOS(username string) error {
	// On macOS, use dscl to create users
	// First, find the next available UID
	uid, err := findNextUIDMacOS()
	if err != nil {
		return fmt.Errorf("failed to find available UID: %w", err)
	}

	// Create the user record
	userPath := "/Users/" + username
	cmds := [][]string{
		{"dscl", ".", "-create", userPath},
		{"dscl", ".", "-create", userPath, "UserShell", "/usr/bin/false"},
		{"dscl", ".", "-create", userPath, "UniqueID", strconv.Itoa(uid)},
		{"dscl", ".", "-create", userPath, "PrimaryGroupID", "20"}, // staff group
		{"dscl", ".", "-create", userPath, "NFSHomeDirectory", "/var/empty"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create user: %s: %w", string(output), err)
		}
	}

	return nil
}

func findNextUIDMacOS() (int, error) {
	// Get list of existing UIDs using dscl
	cmd := exec.Command("dscl", ".", "-list", "/Users", "UniqueID")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	maxUID := 500 // Start from 501 for regular users
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if uid, err := strconv.Atoi(fields[len(fields)-1]); err == nil {
				if uid > maxUID && uid < 65534 {
					maxUID = uid
				}
			}
		}
	}

	return maxUID + 1, nil
}

func removeSystemUser(username string) error {
	if platform.IsMacOS() {
		return removeSystemUserMacOS(username)
	}
	return removeSystemUserLinux(username)
}

func removeSystemUserLinux(username string) error {
	cmd := exec.Command("userdel", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove user: %w", err)
	}

	return nil
}

func removeSystemUserMacOS(username string) error {
	userPath := "/Users/" + username
	cmd := exec.Command("dscl", ".", "-delete", userPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove user: %s: %w", string(output), err)
	}
	return nil
}

func getSystemUID(username string) (int, error) {
	if platform.IsMacOS() {
		return getSystemUIDMacOS(username)
	}
	return getSystemUIDLinux(username)
}

func getSystemUIDLinux(username string) (int, error) {
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) >= 3 && parts[0] == username {
			uid, err := strconv.Atoi(parts[2])
			if err != nil {
				return 0, err
			}
			return uid, nil
		}
	}

	return 0, fmt.Errorf("user not found")
}

func getSystemUIDMacOS(username string) (int, error) {
	// Use dscl to get the UniqueID for a user
	cmd := exec.Command("dscl", ".", "-read", "/Users/"+username, "UniqueID")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("user not found")
	}

	// Output format: "UniqueID: 501"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UniqueID:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				uid, err := strconv.Atoi(parts[1])
				if err != nil {
					return 0, err
				}
				return uid, nil
			}
		}
	}

	return 0, fmt.Errorf("user not found")
}
