//! The scriptable command-line interface.

pub mod flags;
mod mount;
mod nfs;
mod samba;
mod user;
pub mod util;

use crate::samba::Manager;
use crate::{tui, Result};

const HELP: [&str; 3] = ["-h", "--help", "help"];

/// Runs the command given by `args` (the process arguments without the program name).
pub fn execute(args: &[String]) -> Result<()> {
    let Some(command) = args.first().map(String::as_str) else {
        print_usage();
        return Ok(());
    };
    let rest = &args[1..];

    match command {
        "help" | "-h" | "--help" => {
            print_usage();
            return Ok(());
        }
        "version" | "-v" | "--version" => {
            println!(
                "sambo {} - Share Management CLI for Linux and macOS",
                env!("CARGO_PKG_VERSION")
            );
            return Ok(());
        }
        _ => {}
    }

    // Read-only mount subcommands work without root.
    let unprivileged = command == "mount"
        && rest.first().is_some_and(|sub| {
            matches!(sub.as_str(), "list" | "ls" | "discover" | "scan" | "exports")
                || HELP.contains(&sub.as_str())
        });
    // SAFETY: geteuid has no preconditions and cannot fail.
    if !unprivileged && unsafe { libc::geteuid() } != 0 {
        return Err("this tool must be run as root (use sudo)".into());
    }

    match command {
        "tui" => tui::start(Manager::real()),
        "samba" => samba::handle(&Manager::real(), rest),
        "nfs" => nfs::handle(rest),
        "mount" => mount::handle(rest),
        "user" => user::handle(rest),
        other => {
            print_usage();
            Err(format!("unknown command: {other}"))
        }
    }
}

fn print_usage() {
    println!(
        "sambo - Share Management CLI for Linux and macOS

USAGE:
    sambo <command> [options]

COMMANDS:
    tui         Launch interactive menu (recommended)
    samba       Manage Samba (SMB/CIFS) shares
    nfs         Manage NFS shares
    mount       Mount and manage network shares (client-side)
    user        Manage share users and permissions
    help        Show this help message
    version     Show version information

Run 'sambo <command> -h' for more information on a specific command.

EXAMPLES:
    sudo sambo tui                    # Launch interactive menu
    sambo samba list
    sambo samba create -name myshare -path /mnt/data
    sambo nfs create -path /mnt/backup -clients 192.168.1.0/24
    sambo mount cifs -source //server/share -mountpoint /mnt/share
    sambo user add -username john -password secret"
    );
}
