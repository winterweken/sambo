# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sambo is a CLI tool for managing Samba (SMB/CIFS) and NFS shares on Linux headless servers. It provides both CLI commands and an interactive TUI built with Bubble Tea.

## Build Commands

```bash
make build        # Build binary to build/sambo
make build-all    # Cross-compile for linux-amd64, linux-arm64, linux-arm
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
  - `tui/` - Interactive terminal UI using Bubble Tea
    - `tui.go` - Main TUI model and screen navigation
    - `forms.go` - Form components for create/modify operations
    - `views.go` - List view implementations
    - `select.go` - Selection dialogs

## Key Patterns

- All commands except `help` and `version` require root privileges (checked via `os.Geteuid()`)
- Config modifications follow a backup-modify-test-reload pattern
- Samba changes are validated with `testparm` before applying
- NFS changes are applied with `exportfs -ra`
- The TUI uses a state machine pattern with `screen` constants for navigation

## Dependencies

- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - TUI components
- `github.com/charmbracelet/lipgloss` - TUI styling
