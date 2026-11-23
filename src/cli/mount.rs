use std::time::Duration;

use super::flags;
use super::util::{cifs_options, RULE};
use crate::mount::{self, discover};
use crate::{platform, Result};

pub fn handle(args: &[String]) -> Result<()> {
    let Some(subcommand) = args.first() else {
        print_usage();
        return Ok(());
    };
    let rest = &args[1..];
    match subcommand.as_str() {
        "list" | "ls" => list(),
        "cifs" | "smb" | "samba" => mount_cifs(rest),
        "nfs" => mount_nfs(rest),
        "unmount" | "umount" | "remove" | "rm" => unmount(rest),
        "discover" | "scan" => run_discover(rest),
        "exports" => exports(rest),
        "-h" | "--help" | "help" => {
            print_usage();
            Ok(())
        }
        other => {
            print_usage();
            Err(format!("unknown mount subcommand: {other}"))
        }
    }
}

fn list() -> Result<()> {
    let mounts = mount::list().map_err(|e| format!("failed to list mounts: {e}"))?;
    if mounts.is_empty() {
        println!("No network mounts found");
        return Ok(());
    }
    let config_file = if platform::is_macos() {
        "/etc/auto_nfs"
    } else {
        "/etc/fstab"
    };
    println!("Network Mounts:\n{RULE}");
    for m in &mounts {
        println!("Type:        {}", m.kind.as_str().to_uppercase());
        println!("Source:      {}", m.source);
        println!("Mount Point: {}", m.mount_point);
        println!("Options:     {}", m.options);
        println!(
            "Active:      {}",
            if m.active {
                "Yes (currently mounted)"
            } else {
                "No (not mounted)"
            }
        );
        if m.persistent {
            println!("Persistent:  Yes (in {config_file})");
        } else {
            println!("Persistent:  No (temporary)");
        }
        println!("{RULE}");
    }
    Ok(())
}

/// Reads the required -source and -mountpoint flags.
fn endpoints(f: &flags::Flags) -> Result<(String, String)> {
    let (source, mount_point) = (f.string("source", ""), f.string("mountpoint", ""));
    if source.is_empty() || mount_point.is_empty() {
        print_usage();
        return Err("source and mountpoint are required".into());
    }
    Ok((source, mount_point))
}

fn mount_cifs(args: &[String]) -> Result<()> {
    let f = flags::parse(
        args,
        &["source", "mountpoint", "username", "password", "options"],
        &["persistent"],
    )?;
    let (source, mount_point) = endpoints(&f)?;
    let password = f.string("password", "");
    if !password.is_empty() {
        println!("⚠️  WARNING: Using -password on the command line is insecure. Your password may be visible in process listings.");
        println!("   Consider using interactive mode or the TUI instead.");
    }
    let (sudo_uid, sudo_gid) = (std::env::var("SUDO_UID").ok(), std::env::var("SUDO_GID").ok());
    let options = cifs_options(
        &f.string("options", ""),
        &f.string("username", ""),
        &password,
        sudo_uid.as_deref().filter(|s| !s.is_empty()),
        sudo_gid.as_deref().filter(|s| !s.is_empty()),
    );
    let persistent = f.boolean("persistent", false);
    mount::mount_cifs(&source, &mount_point, &options, persistent)
        .map_err(|e| format!("failed to mount CIFS share: {e}"))?;
    println!("CIFS share '{source}' mounted at '{mount_point}'");
    if persistent {
        println!("Added to /etc/fstab for automatic mounting at boot");
    }
    Ok(())
}

fn mount_nfs(args: &[String]) -> Result<()> {
    let f = flags::parse(args, &["source", "mountpoint", "options"], &["persistent"])?;
    let (source, mount_point) = endpoints(&f)?;
    let persistent = f.boolean("persistent", false);
    mount::mount_nfs(
        &source,
        &mount_point,
        &f.string("options", "defaults"),
        persistent,
    )
    .map_err(|e| format!("failed to mount NFS share: {e}"))?;
    println!("NFS share '{source}' mounted at '{mount_point}'");
    if persistent {
        println!("Added to /etc/fstab for automatic mounting at boot");
    }
    Ok(())
}

fn unmount(args: &[String]) -> Result<()> {
    let f = flags::parse(args, &["mountpoint"], &["remove-persistent"])?;
    let mount_point = f.string("mountpoint", "");
    if mount_point.is_empty() {
        print_usage();
        return Err("mountpoint is required".into());
    }
    let remove_persistent = f.boolean("remove-persistent", false);
    mount::unmount(&mount_point, remove_persistent).map_err(|e| format!("failed to unmount: {e}"))?;
    println!("Unmounted '{mount_point}'");
    if remove_persistent {
        println!("Removed from /etc/fstab");
    }
    Ok(())
}

fn clients_of(export: &discover::NfsExport) -> String {
    if export.clients.is_empty() {
        "*".into()
    } else {
        export.clients.join(" ")
    }
}

fn run_discover(args: &[String]) -> Result<()> {
    let f = flags::parse(args, &["subnet", "timeout"], &[])?;
    let timeout: u64 = f
        .string("timeout", "500")
        .parse()
        .map_err(|_| "timeout must be a number of milliseconds".to_string())?;
    let mut subnet = f.string("subnet", "");
    if subnet.is_empty() {
        subnet = discover::local_subnet()
            .map_err(|e| format!("failed to detect subnet: {e}. Please specify with -subnet"))?;
        println!("Auto-detected subnet: {subnet}");
    }

    println!("Scanning for NFS servers on {subnet}...");
    let servers = discover::discover_nfs_servers(&subnet, Duration::from_millis(timeout))
        .map_err(|e| format!("scan failed: {e}"))?;
    if servers.is_empty() {
        println!("No NFS servers found");
        return Ok(());
    }

    println!("\nFound {} NFS server(s):\n{RULE}", servers.len());
    for server in &servers {
        println!("Server: {}", server.ip);
        for export in &server.exports {
            println!("  {} → {}", export.path, clients_of(export));
        }
        println!("{RULE}");
    }
    Ok(())
}

fn exports(args: &[String]) -> Result<()> {
    let f = flags::parse(args, &["server"], &[])?;
    let mut server = f.string("server", "");
    if server.is_empty() {
        server = f.positional.first().cloned().ok_or(
            "server is required. Usage: sambo mount exports -server <ip> OR sambo mount exports <ip>",
        )?;
    }

    println!("Querying NFS exports on {server}...\n");
    let exports = discover::get_nfs_exports(&server).map_err(|e| format!("failed to get exports: {e}"))?;
    if exports.is_empty() {
        println!("No NFS exports found");
        return Ok(());
    }

    println!("NFS Exports on {server}:\n{RULE}");
    println!("{:<35} ALLOWED CLIENTS\n{RULE}", "PATH");
    for export in &exports {
        println!("{:<35} {}", export.path, clients_of(export));
    }
    Ok(())
}

fn print_usage() {
    println!(
        r#"Network Mount Management

USAGE:
    sambo mount <subcommand> [options]

SUBCOMMANDS:
    list, ls                    List all network mounts
    cifs, smb, samba            Mount a CIFS/SMB share
    nfs                         Mount an NFS share
    unmount, umount, remove     Unmount a share
    discover, scan              Scan subnet for NFS servers
    exports                     List exports from an NFS server

CIFS/SMB MOUNT OPTIONS:
    -source <share>             Source share (//server/share)
    -mountpoint <path>          Local mount point
    -username <user>            Username (optional)
    -password <pass>            Password (optional)
    -options <opts>             Additional mount options
    -persistent                 Add to /etc/fstab

NFS MOUNT OPTIONS:
    -source <share>             Source share (server:/path)
    -mountpoint <path>          Local mount point
    -options <opts>             Mount options (default: defaults)
    -persistent                 Add to /etc/fstab

UNMOUNT OPTIONS:
    -mountpoint <path>          Mount point to unmount
    -remove-persistent          Also remove from /etc/fstab

DISCOVERY OPTIONS:
    -subnet <cidr>              Subnet to scan (default: auto-detect)
    -timeout <ms>               Timeout per host in milliseconds (default: 500)

EXAMPLES:
    # List all network mounts
    sambo mount list

    # Mount a CIFS share
    sambo mount cifs \
      -source //server/share \
      -mountpoint /mnt/share \
      -username alice \
      -password secret123

    # Mount a CIFS share persistently (survives reboot)
    sambo mount cifs \
      -source //192.168.1.100/backup \
      -mountpoint /mnt/backup \
      -persistent

    # Mount an NFS share
    sambo mount nfs \
      -source server:/export/data \
      -mountpoint /mnt/data

    # Unmount a share and remove it from fstab
    sambo mount unmount \
      -mountpoint /mnt/share \
      -remove-persistent

    # Scan the local subnet, or a specific one, for NFS servers
    sambo mount discover
    sambo mount discover -subnet 192.168.10.0/24

    # List exports from a specific server
    sambo mount exports 192.168.10.199
    sambo mount exports -server myserver.local"#
    );
}
