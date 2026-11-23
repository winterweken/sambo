use super::flags;
use super::util::{confirm, print_warnings, RULE};
use crate::nfs::{self, Export};
use crate::Result;

pub fn handle(args: &[String]) -> Result<()> {
    let Some(subcommand) = args.first() else {
        print_usage();
        return Ok(());
    };
    let rest = &args[1..];
    match subcommand.as_str() {
        "list" | "ls" => list(),
        "create" | "add" => create(rest),
        "remove" | "rm" | "delete" => remove(rest),
        "modify" | "update" => modify(rest),
        "show" => show(rest),
        "-h" | "--help" | "help" => {
            print_usage();
            Ok(())
        }
        other => {
            print_usage();
            Err(format!("unknown nfs subcommand: {other}"))
        }
    }
}

fn print_export(export: &Export) {
    println!("Path:        {}", export.path);
    println!("Clients:     {}", export.clients);
    println!("Options:     {}", export.options);
}

fn list() -> Result<()> {
    let exports = nfs::list().map_err(|e| format!("failed to list nfs exports: {e}"))?;
    if exports.is_empty() {
        println!("No NFS exports configured");
        return Ok(());
    }
    println!("NFS Exports:\n{RULE}");
    for export in &exports {
        print_export(export);
        println!("{RULE}");
    }
    Ok(())
}

const STRINGS: [&str; 2] = ["path", "clients"];
const BOOLS: [&str; 3] = ["readonly", "sync", "no-root-squash"];

/// The options string for the -readonly / -sync / -no-root-squash flags.
fn options_from(f: &flags::Flags) -> String {
    nfs::build_options(
        f.boolean("readonly", false),
        !f.boolean("sync", true),
        f.boolean("no-root-squash", false),
    )
}

fn required_path(f: &flags::Flags) -> Result<String> {
    let path = f.string("path", "");
    if path.is_empty() {
        print_usage();
        return Err("path is required".into());
    }
    Ok(path)
}

fn create(args: &[String]) -> Result<()> {
    let f = flags::parse(args, &STRINGS, &BOOLS)?;
    let path = required_path(&f)?;
    let export = Export {
        path: path.clone(),
        clients: f.string("clients", "*"),
        options: options_from(&f),
    };
    let warnings = nfs::create(&export).map_err(|e| format!("failed to create nfs export: {e}"))?;
    print_warnings(&warnings);
    println!("NFS export '{path}' created successfully");
    Ok(())
}

fn remove(args: &[String]) -> Result<()> {
    let f = flags::parse(args, &["path"], &["y"])?;
    let path = required_path(&f)?;
    if !confirm(&format!("Remove NFS export '{path}'?"), f.boolean("y", false)) {
        println!("Cancelled");
        return Ok(());
    }
    nfs::remove(&path).map_err(|e| format!("failed to remove nfs export: {e}"))?;
    println!("NFS export '{path}' removed successfully");
    Ok(())
}

fn modify(args: &[String]) -> Result<()> {
    let f = flags::parse(args, &STRINGS, &BOOLS)?;
    let path = required_path(&f)?;
    let clients = Some(f.string("clients", "")).filter(|c| !c.is_empty());
    let warnings = nfs::modify(&path, clients, Some(options_from(&f)))
        .map_err(|e| format!("failed to modify nfs export: {e}"))?;
    print_warnings(&warnings);
    println!("NFS export '{path}' modified successfully");
    Ok(())
}

fn show(args: &[String]) -> Result<()> {
    let path = required_path(&flags::parse(args, &["path"], &[])?)?;
    print_export(&nfs::get(&path).map_err(|e| format!("failed to get nfs export: {e}"))?);
    Ok(())
}

fn print_usage() {
    println!(
        r#"NFS Export Management

USAGE:
    sambo nfs <subcommand> [options]

SUBCOMMANDS:
    list, ls                    List all NFS exports
    create, add                 Create a new NFS export
    remove, rm, delete          Remove an NFS export
    modify, update              Modify an existing NFS export
    show                        Show details of a specific export

CREATE/MODIFY OPTIONS:
    -path <path>                Export directory path
    -clients <spec>             Client specification (IP, CIDR, hostname, or *)
    -readonly                   Make export read-only (default: false)
    -sync                       Use sync mode (default: true)
    -no-root-squash             Disable root squashing (default: false)

REMOVE OPTIONS:
    -path <path>                Export directory path
    -y                          Skip the confirmation prompt

EXAMPLES:
    sambo nfs list
    sambo nfs create -path /mnt/backup -clients 192.168.1.0/24
    sambo nfs create -path /mnt/public -clients "*" -readonly
    sambo nfs modify -path /mnt/backup -clients 192.168.1.100
    sambo nfs remove -path /mnt/backup
    sambo nfs show -path /mnt/backup"#
    );
}
