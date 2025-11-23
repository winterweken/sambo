# Sambo - Linux Share Management CLI

A command-line interface tool for managing Samba (SMB/CIFS) and NFS shares on Linux headless servers.

## Features

- **Samba Share Management**: Create, list, modify, and remove Samba shares
- **NFS Export Management**: Create, list, modify, and remove NFS exports
- **Network Mount Management**: Mount and manage remote CIFS/SMB and NFS shares
- **User Management**: Add, remove, and manage Samba users with password control
- **Simple CLI Interface**: Easy-to-use command structure with helpful output
- **Interactive TUI**: Beautiful text-based interface for all operations
- **Configuration Backup**: Automatic backup before making changes
- **Validation**: Tests configuration before applying changes

## Requirements

### System Requirements
- Linux-based operating system
- Root access (sudo)

### Samba Requirements
- `samba` package installed
- `smbpasswd` command available
- `testparm` command available

### NFS Requirements
- `nfs-kernel-server` or `nfs-server` package installed
- `exportfs` command available

## Installation

### Download Pre-built Binary (Recommended)

Download the latest release for your architecture:

```bash
# For AMD64 (most common Intel/AMD servers)
wget https://github.com/winterweken/sambo/releases/latest/download/sambo-linux-amd64
sudo mv sambo-linux-amd64 /usr/local/bin/sambo
sudo chmod +x /usr/local/bin/sambo

# For ARM64 (Raspberry Pi 4, AWS Graviton, etc.)
wget https://github.com/YOUR_USERNAME/sambo/releases/latest/download/sambo-linux-arm64
sudo mv sambo-linux-arm64 /usr/local/bin/sambo
sudo chmod +x /usr/local/bin/sambo

# For ARM (Raspberry Pi 3 and older)
wget https://github.com/YOUR_USERNAME/sambo/releases/latest/download/sambo-linux-arm
sudo mv sambo-linux-arm /usr/local/bin/sambo
sudo chmod +x /usr/local/bin/sambo

# Verify installation
sudo sambo version
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/YOUR_USERNAME/sambo.git
cd sambo

# Build the binary
make build

# Install to system path
sudo make install

# Or run directly
sudo ./build/sambo
```

### Quick Install Script

```bash
# Build and install from source
make build && sudo make install
```

## Usage

All commands must be run with root privileges:

```bash
sudo sambo <command> [subcommand] [options]
```

### Interactive TUI Mode (Recommended)

Sambo includes a beautiful text-based user interface (TUI) similar to DockStarter, making it easy to manage shares interactively:

```bash
sudo sambo tui
```

The TUI provides:
- **Arrow key navigation** through menus
- **Interactive lists** of shares, exports, and users
- **Visual feedback** with color-coded output
- **Easy navigation** - ESC to go back, Q to quit
- **Real-time viewing** of configured shares

**TUI Navigation:**
- `↑/↓` or `j/k` - Move up/down
- `Enter` - Select menu item
- `ESC` - Go back one level
- `Q` - Return to main menu or quit

This is the easiest way to use Sambo, especially for users who prefer interactive menus over CLI commands.

### General Commands

```bash
sambo help              # Show help
sambo version           # Show version
sambo tui               # Launch interactive menu (requires sudo)
```

## Samba Share Management

### List all Samba shares

```bash
sudo sambo samba list
```

### Create a new share

```bash
# Basic share
sudo sambo samba create -name myshare -path /mnt/data

# Share with options
sudo sambo samba create \
  -name documents \
  -path /mnt/documents \
  -comment "Team Documents" \
  -users alice,bob,charlie

# Read-only share
sudo sambo samba create \
  -name readonly \
  -path /mnt/public \
  -readonly \
  -comment "Public Files"
```

### Show share details

```bash
sudo sambo samba show -name myshare
```

### Modify an existing share

```bash
# Add users to share
sudo sambo samba modify -name myshare -users alice,bob

# Change comment
sudo sambo samba modify -name myshare -comment "New description"
```

### Remove a share

```bash
sudo sambo samba remove -name myshare
```

### Apple Time Machine Support

Sambo includes built-in support for Apple Time Machine backups. When enabled, the share is automatically configured with the necessary VFS modules and settings:

```bash
# Using the TUI (recommended)
sudo sambo tui
# Select "Manage Samba Shares" → "Create Share"
# Check the "Time Machine" checkbox

# Using CLI
sudo sambo samba create \
  -name timemachine \
  -path /mnt/backup/timemachine \
  -timemachine \
  -comment "Time Machine Backup"
```

**Time Machine Configuration:**

Sambo automatically configures both global and share-level settings:

*Global Samba settings (added automatically):*
- `min protocol = SMB2` - Required for macOS compatibility
- `ea support = yes` - Extended attributes support
- `vfs objects = catia fruit streams_xattr` - macOS file system modules

*Share-level settings:*
- `fruit:metadata = stream` - Metadata handling
- `fruit:model = MacSamba` - Identifies as Mac-compatible server
- `fruit:aapl = yes` - Apple file system support
- `fruit:time machine = yes` - Time Machine discovery
- `fruit:time machine max size = 500G` - Default quota (configurable in smb.conf)
- Additional fruit module optimizations for macOS compatibility

**Important Notes:**
- The share path must exist and be writable
- Users must be added with valid Samba passwords (`sambo user add`)
- macOS will discover the share automatically in Time Machine preferences
- Recommended to use a dedicated share for Time Machine backups
- Requires Samba 4.8 or newer for full Time Machine support

## NFS Export Management

### List all NFS exports

```bash
sudo sambo nfs list
```

### Create a new export

```bash
# Basic export (all clients)
sudo sambo nfs create -path /mnt/backup

# Export to specific network
sudo sambo nfs create \
  -path /mnt/data \
  -clients 192.168.1.0/24

# Read-only export
sudo sambo nfs create \
  -path /mnt/public \
  -clients "*" \
  -readonly

# Export with no root squash
sudo sambo nfs create \
  -path /mnt/secure \
  -clients 192.168.1.100 \
  -no-root-squash
```

### Show export details

```bash
sudo sambo nfs show -path /mnt/backup
```

### Modify an existing export

```bash
# Change client access
sudo sambo nfs modify -path /mnt/backup -clients 192.168.1.50

# Make read-only
sudo sambo nfs modify -path /mnt/backup -readonly
```

### Remove an export

```bash
sudo sambo nfs remove -path /mnt/backup
```

### NFS Configuration Presets (TUI)

The TUI provides easy checkbox options for common NFS configurations:

**✓ Read Only** - Mount as read-only
- Perfect for: Media servers (Plex, Jellyfin), public file shares
- Options: `ro` instead of `rw`

**✓ No Root Squash** - Allow root access
- Perfect for: Backup destinations, Time Machine, system administration
- Options: `no_root_squash` instead of `root_squash`
- Warning: Only enable for trusted clients

**✓ Async Mode** - Faster performance
- Perfect for: Non-critical data, temporary files, media streaming
- Options: `async` instead of `sync`
- Warning: Data may be lost in case of server crash

**Common Use Cases:**

| Use Case | Read Only | No Root Squash | Async Mode |
|----------|-----------|----------------|------------|
| **Media Server** (Plex/Jellyfin) | ✓ | ✗ | ✓ |
| **Backup Target** (Time Machine/rsync) | ✗ | ✓ | ✗ |
| **Public Files** (read-only share) | ✓ | ✗ | ✗ |
| **Development** (fast, not critical) | ✗ | ✗ | ✓ |
| **Production Data** (safe default) | ✗ | ✗ | ✗ |

All exports include `no_subtree_check` by default for better performance.

## Network Mount Management

Network mount management allows you to mount remote shares on your system (client-side functionality). This complements the server-side share management features.

### List all network mounts

```bash
sudo sambo mount list
```

### Mount a CIFS/SMB share

```bash
# Basic mount
sudo sambo mount cifs \
  -source //server/share \
  -mountpoint /mnt/share \
  -username alice \
  -password secret123

# Persistent mount (survives reboot)
sudo sambo mount cifs \
  -source //192.168.1.100/backup \
  -mountpoint /mnt/backup \
  -username admin \
  -password pass123 \
  -persistent
```

### Mount an NFS share

```bash
# Basic mount
sudo sambo mount nfs \
  -source server:/export/data \
  -mountpoint /mnt/data

# Persistent mount (survives reboot)
sudo sambo mount nfs \
  -source 192.168.1.100:/backups \
  -mountpoint /mnt/backups \
  -persistent
```

### Unmount a share

```bash
# Unmount temporarily
sudo sambo mount unmount -mountpoint /mnt/share

# Unmount and remove from fstab
sudo sambo mount unmount \
  -mountpoint /mnt/share \
  -remove-persistent
```

### Mount Management in TUI

The interactive TUI provides easy forms for mounting shares:

```bash
sudo sambo tui
# Select "Manage Network Mounts"
```

**Mount Features:**
- **List Mounts**: View all currently mounted network shares (CIFS and NFS)
- **Mount CIFS/SMB**: Mount Windows/Samba shares with authentication
- **Mount NFS**: Mount NFS shares from Linux/Unix servers
- **Unmount Share**: Safely unmount network shares
- **Persistent Option**: Automatically mount shares at boot via /etc/fstab

**Important Notes:**
- Mount points will be created automatically if they don't exist
- CIFS mounts support username/password authentication
- Persistent mounts are added to `/etc/fstab` for automatic mounting
- The "Persistent" checkbox in TUI ensures mounts survive reboots

## User Management

### List all Samba users

```bash
sudo sambo user list
```

### Add a new user

```bash
# Create user (creates system user automatically)
sudo sambo user add -username alice -password secret123

# Add user without creating system user
sudo sambo user add \
  -username bob \
  -password pass456 \
  -create-system=false
```

### Show user details

```bash
sudo sambo user show -username alice
```

### Change user password

```bash
sudo sambo user passwd -username alice -password newsecret789
```

### Remove a user

```bash
# Remove Samba user only
sudo sambo user remove -username alice

# Remove both Samba and system user
sudo sambo user remove -username alice -remove-system
```

## Common Workflows

### Setting up a new shared folder

```bash
# 1. Create the directory
sudo mkdir -p /mnt/shared

# 2. Set permissions
sudo chmod 775 /mnt/shared

# 3. Create Samba users
sudo sambo user add -username alice -password pass1
sudo sambo user add -username bob -password pass2

# 4. Create Samba share
sudo sambo samba create \
  -name shared \
  -path /mnt/shared \
  -users alice,bob \
  -comment "Shared Team Folder"

# 5. Create NFS export (optional, for Linux clients)
sudo sambo nfs create \
  -path /mnt/shared \
  -clients 192.168.1.0/24
```

### Removing a share completely

```bash
# 1. Remove Samba share
sudo sambo samba remove -name shared

# 2. Remove NFS export
sudo sambo nfs remove -path /mnt/shared

# 3. Remove users (optional)
sudo sambo user remove -username alice
sudo sambo user remove -username bob
```

### Mounting a remote share persistently

```bash
# 1. Mount a remote CIFS share (from another server)
sudo sambo mount cifs \
  -source //192.168.1.200/documents \
  -mountpoint /mnt/remote-docs \
  -username john \
  -password secret \
  -persistent

# 2. Mount a remote NFS share (from another server)
sudo sambo mount nfs \
  -source 192.168.1.201:/backups \
  -mountpoint /mnt/remote-backups \
  -persistent

# 3. Verify mounts
sudo sambo mount list
```

## Configuration Files

Sambo modifies the following system configuration files:

- **Samba**: `/etc/samba/smb.conf`
- **NFS**: `/etc/exports`
- **Mounts**: `/etc/fstab` (for persistent mounts)

Backups are created before modifications:
- `/etc/samba/smb.conf.backup`
- `/etc/exports.backup`
- `/etc/fstab.backup`

## Troubleshooting

### Samba issues

```bash
# Test Samba configuration
sudo testparm

# Check Samba service status
sudo systemctl status smbd

# Restart Samba service
sudo systemctl restart smbd

# View Samba logs
sudo journalctl -u smbd -f
```

### NFS issues

```bash
# Check current exports
sudo exportfs -v

# Re-export all
sudo exportfs -ra

# Check NFS service status
sudo systemctl status nfs-server
# or
sudo systemctl status nfs-kernel-server

# View NFS logs
sudo journalctl -u nfs-server -f
```

### Mount issues

```bash
# List all network mounts
sudo sambo mount list

# Check if a share is mounted
mount | grep cifs
mount | grep nfs

# View fstab entries
cat /etc/fstab

# Manually mount all fstab entries
sudo mount -a

# Check mount errors
sudo dmesg | grep -i cifs
sudo dmesg | grep -i nfs

# For CIFS authentication issues, check credentials file
sudo cat /root/.smbcredentials

# Unmount a stuck mount
sudo umount -f /mnt/mountpoint
# or force lazy unmount
sudo umount -l /mnt/mountpoint
```

### Permission issues

```bash
# Ensure directory exists and has correct permissions
sudo ls -la /path/to/share

# Fix ownership
sudo chown -R nobody:nogroup /path/to/share

# Fix permissions
sudo chmod -R 775 /path/to/share
```

## Security Considerations

1. **Always use strong passwords** for Samba users
2. **Limit NFS exports** to specific IP addresses or networks when possible
3. **Use read-only mode** for shares that don't require write access
4. **Regular backups** of configuration files are maintained automatically
5. **Firewall rules**: Ensure appropriate ports are open
   - Samba: 139, 445 (TCP)
   - NFS: 2049 (TCP/UDP)

## Development

### Project Structure

```
sambo/
├── main.go              # Entry point
├── cmd/                 # CLI commands
│   ├── root.go         # Main command handler
│   ├── samba.go        # Samba commands
│   ├── nfs.go          # NFS commands
│   ├── mount.go        # Network mount commands
│   └── user.go         # User commands
└── pkg/                 # Internal packages
    ├── samba/          # Samba management
    │   └── samba.go
    ├── nfs/            # NFS management
    │   └── nfs.go
    ├── mount/          # Network mount management
    │   └── mount.go
    ├── user/           # User management
    │   └── user.go
    └── tui/            # Terminal UI
        ├── tui.go
        ├── forms.go
        ├── views.go
        └── select.go
```

### Building

```bash
# Development build
go build -o sambo

# Production build with optimizations
go build -ldflags="-s -w" -o sambo

# Cross-compile for different architectures
GOOS=linux GOARCH=amd64 go build -o sambo-amd64
GOOS=linux GOARCH=arm64 go build -o sambo-arm64
```

## License

This project is provided as-is for managing Linux file shares.

## Contributing

Feel free to submit issues and enhancement requests!

## Version

v1.5.1
