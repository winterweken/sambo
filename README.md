# Sambo - Linux Share Management CLI

A command-line interface tool for managing Samba (SMB/CIFS) and NFS shares on Linux headless servers.

## Features

- **Samba Share Management**: Create, list, modify, and remove Samba shares
- **NFS Export Management**: Create, list, modify, and remove NFS exports
- **User Management**: Add, remove, and manage Samba users with password control
- **Simple CLI Interface**: Easy-to-use command structure with helpful output
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

### Build from Source

```bash
# Clone or navigate to the project directory
cd sambo

# Build the binary
go build -o sambo

# Install to system path (optional)
sudo mv sambo /usr/local/bin/

# Or run directly
sudo ./sambo
```

### Quick Install Script

```bash
# Build and install
go build -o sambo && sudo mv sambo /usr/local/bin/
```

## Usage

All commands must be run with root privileges:

```bash
sudo sambo <command> [subcommand] [options]
```

### General Commands

```bash
sambo help              # Show help
sambo version           # Show version
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

## Configuration Files

Sambo modifies the following system configuration files:

- **Samba**: `/etc/samba/smb.conf`
- **NFS**: `/etc/exports`

Backups are created before modifications:
- `/etc/samba/smb.conf.backup`
- `/etc/exports.backup`

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
│   └── user.go         # User commands
└── pkg/                 # Internal packages
    ├── samba/          # Samba management
    │   └── samba.go
    ├── nfs/            # NFS management
    │   └── nfs.go
    └── user/           # User management
        └── user.go
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

v1.0.0
