package validate

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// invalidShareNameChars contains characters not allowed in share names
	shareNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	// Reserved share names in Samba
	reservedNames = map[string]bool{
		"global":   true,
		"homes":    true,
		"printers": true,
		"print$":   true,
		"ipc$":     true,
	}

	// Critical system paths that should not be shared directly
	criticalPaths = []string{
		"/",
		"/etc",
		"/bin",
		"/sbin",
		"/usr",
		"/lib",
		"/lib64",
		"/boot",
		"/dev",
		"/proc",
		"/sys",
		"/run",
		"/var/run",
		"/root",
	}
)

// ShareName validates a Samba share name
func ShareName(name string) error {
	if name == "" {
		return errors.New("share name is required")
	}

	if len(name) > 80 {
		return errors.New("share name must be 80 characters or less")
	}

	if !shareNameRegex.MatchString(name) {
		return errors.New("share name can only contain letters, numbers, underscores, and hyphens")
	}

	if reservedNames[strings.ToLower(name)] {
		return errors.New("share name is reserved by Samba")
	}

	return nil
}

// Path validates a filesystem path for sharing
func Path(path string) error {
	if path == "" {
		return errors.New("path is required")
	}

	if !filepath.IsAbs(path) {
		return errors.New("path must be an absolute path")
	}

	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	// Check against critical system paths
	for _, critical := range criticalPaths {
		if cleanPath == critical {
			return errors.New("cannot share critical system directory: " + critical)
		}
	}

	return nil
}

// MountPoint validates a mount point path
func MountPoint(path string) error {
	if path == "" {
		return errors.New("mount point is required")
	}

	if !filepath.IsAbs(path) {
		return errors.New("mount point must be an absolute path")
	}

	cleanPath := filepath.Clean(path)

	// Check against critical system paths
	for _, critical := range criticalPaths {
		if cleanPath == critical {
			return errors.New("cannot mount to critical system directory: " + critical)
		}
	}

	return nil
}

// CIFSSource validates a CIFS source path (//server/share)
func CIFSSource(source string) error {
	if source == "" {
		return errors.New("source is required")
	}

	if !strings.HasPrefix(source, "//") {
		return errors.New("CIFS source must start with // (e.g., //server/share)")
	}

	parts := strings.Split(strings.TrimPrefix(source, "//"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return errors.New("CIFS source must be in format //server/share")
	}

	return nil
}

// NFSSource validates an NFS source path (server:/path)
func NFSSource(source string) error {
	if source == "" {
		return errors.New("source is required")
	}

	if !strings.Contains(source, ":") {
		return errors.New("NFS source must be in format server:/path")
	}

	parts := strings.SplitN(source, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return errors.New("NFS source must be in format server:/path")
	}

	if !strings.HasPrefix(parts[1], "/") {
		return errors.New("NFS export path must be absolute (start with /)")
	}

	return nil
}

// Username validates a username
func Username(username string) error {
	if username == "" {
		return errors.New("username is required")
	}

	if len(username) > 32 {
		return errors.New("username must be 32 characters or less")
	}

	// Linux username rules: start with letter or underscore, contain only alphanumeric, underscore, hyphen
	usernameRegex := regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
	if !usernameRegex.MatchString(strings.ToLower(username)) {
		return errors.New("username must start with a letter or underscore and contain only letters, numbers, underscores, and hyphens")
	}

	return nil
}

// Password validates a password (basic checks)
func Password(password string) error {
	if password == "" {
		return errors.New("password is required")
	}

	if len(password) < 4 {
		return errors.New("password must be at least 4 characters")
	}

	return nil
}

// NFSClients validates NFS client specification
func NFSClients(clients string) error {
	if clients == "" {
		return errors.New("clients specification is required")
	}

	// Allow * for all clients
	if clients == "*" {
		return nil
	}

	// Basic validation - allow IPs, CIDRs, hostnames
	// This is a simple check; NFS will validate more thoroughly
	if strings.ContainsAny(clients, "[]{}()\\|;'\"") {
		return errors.New("clients specification contains invalid characters")
	}

	return nil
}
