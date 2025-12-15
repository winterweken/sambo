# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sambo is a CLI tool for managing Samba (SMB/CIFS) and NFS shares on Linux headless servers. It provides both CLI commands and an interactive TUI built with Bubble Tea.

## Build Commands

```bash
make build        # Build binary to build/sambo
make build-all    # Cross-compile for linux-amd64, linux-arm64, linux-arm, darwin-amd64, darwin-arm64
make install      # Build and install to /usr/local/bin (requires sudo)
make test         # Run tests
make fmt          # Format code
make lint         # Run golangci-lint
make clean        # Remove build artifacts
```

For development, build with `make build` and run with `sudo ./build/sambo`.

## Architecture

The codebase follows a clean separation between CLI commands and business logic:

- **cmd/**: CLI command handlers using Go's `flag` package (not cobra)
  - `root.go` - Entry point and main command routing
  - `samba.go`, `nfs.go`, `mount.go`, `user.go` - Subcommand handlers

- **pkg/**: Core business logic packages
  - `samba/` - Samba config parsing and management (`/etc/samba/smb.conf`)
  - `nfs/` - NFS exports management (`/etc/exports`)
  - `mount/` - Client-side mount operations (CIFS/NFS mounting, fstab management)
  - `user/` - Samba user management (smbpasswd)
  - `avahi/` - Avahi/Bonjour service management for Time Machine discovery
  - `system/` - System dependency detection and installation
  - `platform/` - Platform-specific paths (Linux vs macOS)
  - `validate/` - Input validation helpers
  - `tui/` - Interactive terminal UI using Bubble Tea
    - `tui.go` - Main TUI model and screen navigation
    - `forms.go` - Form components for create/modify operations
    - `views.go` - List view implementations
    - `select.go` - Selection dialogs
    - `dependencies.go` - Dependency check/install UI

## Key Patterns

- All commands except `help` and `version` require root privileges (checked via `os.Geteuid()`)
- Config modifications follow a backup-modify-test-reload pattern
- Samba changes are validated with `testparm` before applying
- NFS changes are applied with `exportfs -ra`
- The TUI uses a state machine pattern with `screen` constants for navigation
- Time Machine shares use `vfs objects = catia fruit streams_xattr` and fruit:* options
- Avahi service files are auto-generated for Time Machine discovery

## Time Machine Support

When creating a Time Machine share:
1. Share is created with Apple-compatible VFS options (fruit, catia, streams_xattr)
2. Avahi service file is generated at `/etc/avahi/services/samba-timemachine.service`
3. Share appears in macOS Finder via Bonjour/mDNS

Required for Time Machine:
- Samba user with password (for authentication)
- Share must NOT be read-only
- Directory must be writable by the Samba user

## Dependencies

- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - TUI components (textinput, spinner)
- `github.com/charmbracelet/lipgloss` - TUI styling

## Releasing

```bash
make build-all
gh release create v1.x.x build/sambo-* --title "v1.x.x" --notes "Release notes"
```
