# Sambo - Share Management CLI for Linux & macOS

A command-line tool for managing Samba (SMB/CIFS) and NFS shares on Linux and macOS, featuring both a traditional CLI and an interactive terminal UI (TUI).

## Features

- **Samba Share Management**: Create, list, modify, and remove Samba shares with share type presets
- **NFS Export Management**: Create, list, modify, and remove NFS exports
- **Network Mount Management**: Mount and manage remote CIFS/SMB and NFS shares
- **NFS Server Discovery**: Scan your local network for available NFS servers
- **User Management**: Add, remove, and manage Samba users with password control
- **Interactive TUI**: Beautiful text-based interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **CLI Interface**: Scriptable command structure for automation and headless use
- **Share Type Presets**: Built-in configurations for Time Machine, Ubiquiti Protect, and media servers
- **Configuration Safety**: Automatic backups, validation via `testparm`, and atomic file operations
- **Cross-Platform**: Native support for both Linux and macOS with platform-aware service management

## Requirements

### Linux

- Root access (sudo)
- **Samba**: `samba` package (`smbd`, `smbpasswd`, `testparm`)
- **NFS**: `nfs-kernel-server` or `nfs-utils` (`exportfs`)

### macOS

- Root access (sudo)
- **Samba**: Install via Homebrew (`brew install samba`)
- **NFS**: Built-in (`nfsd`)
- NFS client tools are built-in (`mount_nfs`, `mount_smbfs`)

> **Note**: Sambo auto-detects your platform and uses the correct paths, service managers, and mount commands for each OS.

### macOS Client Setup

For macOS clients connecting to sambo-managed shares, run the client setup script:

```bash
# If sambo is installed
/usr/local/share/sambo/macos-client-setup.sh

# Or download and run directly
curl -fsSL https://raw.githubusercontent.com/winterweken/sambo/main/scripts/macos-client-setup.sh | bash
```

The script checks SMB/CIFS and NFS tools, network connectivity, firewall settings, and automount configuration. Additional options:

```bash
./macos-client-setup.sh --test-smb 192.168.1.10      # Test SMB connectivity
./macos-client-setup.sh --test-nfs 192.168.1.10      # Test NFS connectivity
sudo ./macos-client-setup.sh --setup-automount        # Setup persistent NFS mounts
./macos-client-setup.sh --install-optional             # Install optional tools via Homebrew
```

## Installation

### One-Liner Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/winterweken/sambo/main/scripts/install.sh | bash
```

This automatically detects your OS and architecture, downloads the latest release, and installs to `/usr/local/bin`.

### Download Pre-built Binary

```bash
# Linux AMD64
wget https://github.com/winterweken/sambo/releases/latest/download/sambo-linux-amd64
sudo mv sambo-linux-amd64 /usr/local/bin/sambo && sudo chmod +x /usr/local/bin/sambo

# Linux ARM64 (Raspberry Pi 4, AWS Graviton)
wget https://github.com/winterweken/sambo/releases/latest/download/sambo-linux-arm64
sudo mv sambo-linux-arm64 /usr/local/bin/sambo && sudo chmod +x /usr/local/bin/sambo

# macOS Apple Silicon
wget https://github.com/winterweken/sambo/releases/latest/download/sambo-darwin-arm64
sudo mv sambo-darwin-arm64 /usr/local/bin/sambo && sudo chmod +x /usr/local/bin/sambo

# macOS Intel
wget https://github.com/winterweken/sambo/releases/latest/download/sambo-darwin-amd64
sudo mv sambo-darwin-amd64 /usr/local/bin/sambo && sudo chmod +x /usr/local/bin/sambo

# Verify
sudo sambo version
```

### Build from Source

```bash
git clone https://github.com/winterweken/sambo.git
cd sambo
make build
sudo make install
```

## Usage

All commands require root privileges:

```bash
sudo sambo <command> [subcommand] [options]
```

### Interactive TUI Mode (Recommended)

```bash
sudo sambo tui
```

The TUI provides arrow-key navigation, interactive forms, color-coded feedback, and dependency checking — the easiest way to manage shares.

**Navigation:** `↑/↓` or `j/k` to move, `Enter` to select, `ESC` to go back, `Q` to quit.

### General Commands

```bash
sambo help      # Show help
sambo version   # Show version
sambo tui       # Launch interactive TUI (requires sudo)
```

## Samba Share Management

### List / Show / Create / Modify / Remove

```bash
sudo sambo samba list
sudo sambo samba show -name myshare

# Create a basic share
sudo sambo samba create -name myshare -path /mnt/data

# Create with options
sudo sambo samba create \
  -name documents \
  -path /mnt/documents \
  -comment "Team Documents" \
  -users alice,bob,charlie

# Read-only share
sudo sambo samba create -name readonly -path /mnt/public -readonly

# Modify
sudo sambo samba modify -name myshare -users alice,bob
sudo sambo samba modify -name myshare -comment "New description"

# Remove
sudo sambo samba remove -name myshare
```

### Share Type Presets

Sambo includes optimized presets for common use cases:

#### Apple Time Machine

```bash
sudo sambo samba create -name timemachine -path /mnt/backup/timemachine -type timemachine
```

Automatically configures:
- VFS modules: `catia fruit streams_xattr`
- Apple metadata: `fruit:aapl`, `fruit:time machine = yes`
- Global settings: `min protocol = SMB2`, `ea support = yes`
- Reliability: durable handles, disabled kernel oplocks/share modes
- Avahi/Bonjour mDNS advertisement for auto-discovery on macOS

> Requires Samba 4.8+. The share path must exist and users need valid Samba passwords.

#### Ubiquiti Protect

```bash
sudo sambo samba create -name unifi-protect -path /mnt/video -type unifi-protect
```

Configures: `create mask = 0660`, `directory mask = 0770`, `inherit permissions`, `nt acl support`, `streams_xattr`. Automatically sets `0770` permissions with owner set to the first valid user.

#### Media Server (Plex, Jellyfin)

```bash
sudo sambo samba create -name movies -path /mnt/media -type media
```

Configures: `use sendfile = yes`, `strict locking = no`, `aio read/write size = 16384`.

## NFS Export Management

```bash
sudo sambo nfs list
sudo sambo nfs show -path /mnt/backup

# Create exports
sudo sambo nfs create -path /mnt/data -clients 192.168.1.0/24
sudo sambo nfs create -path /mnt/public -clients "*" -readonly
sudo sambo nfs create -path /mnt/secure -clients 192.168.1.100 -no-root-squash

# Modify
sudo sambo nfs modify -path /mnt/backup -clients 192.168.1.50 -readonly

# Remove
sudo sambo nfs remove -path /mnt/backup
```

### NFS Options Reference

| Use Case | Read Only | No Root Squash | Async Mode |
|----------|-----------|----------------|------------|
| **Media Server** (Plex/Jellyfin) | ✓ | ✗ | ✓ |
| **Backup Target** (Time Machine/rsync) | ✗ | ✓ | ✗ |
| **Public Files** (read-only share) | ✓ | ✗ | ✗ |
| **Development** (fast, not critical) | ✗ | ✗ | ✓ |
| **Production Data** (safe default) | ✗ | ✗ | ✗ |

All exports include `no_subtree_check` by default for better performance.

## Network Mount Management

```bash
sudo sambo mount list

# Mount CIFS/SMB
sudo sambo mount cifs \
  -source //server/share \
  -mountpoint /mnt/share \
  -username alice -password secret123

# Mount NFS
sudo sambo mount nfs \
  -source server:/export/data \
  -mountpoint /mnt/data

# Persistent mount (survives reboot via /etc/fstab or macOS auto_nfs)
sudo sambo mount cifs -source //server/share -mountpoint /mnt/share \
  -username admin -password pass -persistent

# Unmount
sudo sambo mount unmount -mountpoint /mnt/share
sudo sambo mount unmount -mountpoint /mnt/share -remove-persistent
```

### NFS Server Discovery

Scan your local subnet for NFS servers:

```bash
# Auto-detect subnet
sudo sambo mount scan

# Specify subnet
sudo sambo mount scan -subnet 192.168.10.0/24

# Query specific server
sudo sambo mount scan -server 192.168.1.100
```

## User Management

```bash
sudo sambo user list
sudo sambo user show -username alice

# Add user (creates system user automatically)
sudo sambo user add -username alice -password secret123

# Change password
sudo sambo user passwd -username alice -password newsecret789

# Remove user
sudo sambo user remove -username alice
sudo sambo user remove -username alice -remove-system  # Also remove system user
```

## Configuration Files

Sambo modifies the following system files (automatic backups are created before changes):

| File | Purpose | Backup |
|------|---------|--------|
| `/etc/samba/smb.conf` (Linux) or `/opt/homebrew/etc/smb.conf` (macOS) | Samba shares | `.backup` |
| `/etc/exports` | NFS exports | `.backup` |
| `/etc/fstab` (Linux) | Persistent mounts | `.backup` |
| `/etc/auto_nfs` (macOS) | Persistent NFS mounts | `.backup` |
| `/etc/avahi/services/samba.service` (Linux) | Time Machine mDNS | — |

## Troubleshooting

### Samba

```bash
sudo testparm                    # Test configuration
sudo systemctl status smbd       # Service status (Linux)
brew services list               # Service status (macOS)
sudo journalctl -u smbd -f      # Logs (Linux)
```

### NFS

```bash
sudo exportfs -v                 # Current exports
sudo exportfs -ra                # Re-export all
sudo systemctl status nfs-server # Status (Linux)
sudo nfsd status                 # Status (macOS)
```

### Mounts

```bash
sudo sambo mount list            # List network mounts
mount | grep -E 'cifs|nfs'      # System mount check
sudo dmesg | grep -iE 'cifs|nfs' # Kernel messages
sudo umount -f /mnt/stuck        # Force unmount
```

## Security Considerations

1. **Strong passwords** for all Samba users
2. **Restrict NFS exports** to specific IPs or subnets when possible
3. **Read-only mode** for shares that don't require write access
4. **Firewall ports**: Samba (139, 445 TCP), NFS (2049 TCP/UDP)
5. **Credentials files** are stored with `0600` permissions in `/root/.sambo/`

## Development

### Project Structure

```
sambo/
├── main.go                  # Entry point, version injection
├── cmd/                     # CLI command handlers
│   ├── root.go             # Command router, privilege checks
│   ├── samba.go            # Samba share commands
│   ├── nfs.go              # NFS export commands
│   ├── mount.go            # Mount/unmount/scan commands
│   └── user.go             # User management commands
├── pkg/
│   ├── samba/              # Samba Manager (DI pattern)
│   │   ├── samba.go        # Manager, Create, Modify, Remove, List
│   │   └── *_test.go       # Unit tests with mocked dependencies
│   ├── nfs/                # NFS export management
│   │   ├── nfs.go          # CRUD for /etc/exports
│   │   └── options.go      # Shared NFS option builder
│   ├── mount/              # Client-side mounting
│   │   ├── mount.go        # Mount/unmount, fstab management
│   │   └── discover.go     # NFS server discovery (subnet scan)
│   ├── tui/                # Terminal UI (Bubble Tea)
│   │   ├── tui.go          # Model-View-Update state machine
│   │   ├── forms.go        # Form models for create/modify
│   │   ├── views.go        # List and table rendering
│   │   ├── select.go       # Selection dialogs
│   │   └── dependencies.go # Dependency check and install flow
│   ├── avahi/              # Avahi/Bonjour mDNS service files
│   ├── platform/           # OS detection and path resolution
│   ├── service/            # Service status checking and management
│   ├── system/             # Interfaces: FileSystem, CommandExecutor, Platform
│   ├── user/               # Samba user management (pdbedit, smbpasswd)
│   ├── validate/           # Input validation (paths, names, sources)
│   └── confirm/            # Interactive confirmation prompts
├── scripts/                # Install scripts, macOS client setup
└── Makefile                # Build, test, release targets
```

### Build & Test

```bash
make build          # Build binary to ./build/sambo
make build-all      # Cross-compile for all platforms
make test           # Run tests
make test-cover     # Tests with coverage report
make test-race      # Tests with race detector
make lint           # Run golangci-lint
make fmt            # Format code
make install        # Build and install to /usr/local/bin
make release        # Create release tarballs
make package-macos  # Create macOS .pkg installers
```

## Credits

- **[Samba Team](https://www.samba.org/)** — Core file sharing capabilities
- **[Charm](https://charm.sh/)** — Bubble Tea, Bubbles, and Lipgloss for the TUI
- **Contributors**: Carter (Project Lead), Claude / Gemini (AI Assistants)

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
