use super::flags;
use super::util::{confirm, parse_csv, print_warnings, RULE};
use crate::samba::{Manager, Share, ShareUpdate};
use crate::Result;

pub fn handle(manager: &Manager, args: &[String]) -> Result<()> {
    let Some(subcommand) = args.first() else {
        print_usage();
        return Ok(());
    };
    let rest = &args[1..];
    match subcommand.as_str() {
        "list" | "ls" => list(manager),
        "create" | "add" => create(manager, rest),
        "remove" | "rm" | "delete" => remove(manager, rest),
        "modify" | "update" => modify(manager, rest),
        "show" => show(manager, rest),
        "-h" | "--help" | "help" => {
            print_usage();
            Ok(())
        }
        other => {
            print_usage();
            Err(format!("unknown samba subcommand: {other}"))
        }
    }
}

fn print_share(share: &Share) {
    println!("Name:         {}", share.name);
    println!("Path:         {}", share.path);
    println!("Comment:      {}", share.comment);
    println!("Read Only:    {}", share.read_only);
    println!("Browseable:   {}", share.browseable);
    println!("Time Machine: {}", share.time_machine);
    if share.time_machine && !share.time_machine_max_size.is_empty() && share.time_machine_max_size != "0" {
        println!("TM Max Size:  {}", share.time_machine_max_size);
    }
    if !share.valid_users.is_empty() {
        println!("Valid Users:  [{}]", share.valid_users.join(" "));
    }
}

fn list(manager: &Manager) -> Result<()> {
    let shares = manager
        .list()
        .map_err(|e| format!("failed to list samba shares: {e}"))?;
    if shares.is_empty() {
        println!("No Samba shares configured");
        return Ok(());
    }
    println!("Samba Shares:\n{RULE}");
    for share in &shares {
        print_share(share);
        println!("{RULE}");
    }
    Ok(())
}

fn create(manager: &Manager, args: &[String]) -> Result<()> {
    let f = flags::parse(
        args,
        &["name", "path", "comment", "users", "type", "tmsize"],
        &["readonly", "browseable", "timemachine"],
    )?;
    let (name, path) = (f.string("name", ""), f.string("path", ""));
    if name.is_empty() || path.is_empty() {
        print_usage();
        return Err("name and path are required".into());
    }
    let share = Share {
        name: name.clone(),
        path,
        comment: f.string("comment", ""),
        read_only: f.boolean("readonly", false),
        browseable: f.boolean("browseable", true),
        valid_users: parse_csv(&f.string("users", "")),
        share_type: f.string("type", "general"),
        time_machine: f.boolean("timemachine", false),
        time_machine_max_size: f.string("tmsize", "0"),
    };
    let warnings = manager
        .create(&share)
        .map_err(|e| format!("failed to create samba share: {e}"))?;
    print_warnings(&warnings);
    println!("Samba share '{name}' created successfully");
    Ok(())
}

fn remove(manager: &Manager, args: &[String]) -> Result<()> {
    let f = flags::parse(args, &["name"], &["y"])?;
    let name = f.string("name", "");
    if name.is_empty() {
        print_usage();
        return Err("name is required".into());
    }
    if !confirm(&format!("Remove Samba share '{name}'?"), f.boolean("y", false)) {
        println!("Cancelled");
        return Ok(());
    }
    let warnings = manager
        .remove(&name)
        .map_err(|e| format!("failed to remove samba share: {e}"))?;
    print_warnings(&warnings);
    println!("Samba share '{name}' removed successfully");
    Ok(())
}

fn modify(manager: &Manager, args: &[String]) -> Result<()> {
    let f = flags::parse(args, &["name", "comment", "users"], &["readonly", "browseable"])?;
    let name = f.string("name", "");
    if name.is_empty() {
        print_usage();
        return Err("name is required".into());
    }
    // A flag left at its default means "leave this setting alone".
    let non_empty = |value: String| (!value.is_empty()).then_some(value);
    let update = ShareUpdate {
        comment: non_empty(f.string("comment", "")),
        read_only: f.boolean("readonly", false).then_some(true),
        browseable: (!f.boolean("browseable", true)).then_some(false),
        valid_users: non_empty(f.string("users", "")).map(|users| parse_csv(&users)),
        time_machine: None,
    };
    let warnings = manager
        .modify(&name, update)
        .map_err(|e| format!("failed to modify samba share: {e}"))?;
    print_warnings(&warnings);
    println!("Samba share '{name}' modified successfully");
    Ok(())
}

fn show(manager: &Manager, args: &[String]) -> Result<()> {
    let name = flags::parse(args, &["name"], &[])?.string("name", "");
    if name.is_empty() {
        print_usage();
        return Err("name is required".into());
    }
    print_share(
        &manager
            .get(&name)
            .map_err(|e| format!("failed to get samba share: {e}"))?,
    );
    Ok(())
}

fn print_usage() {
    println!(
        r#"Samba Share Management

USAGE:
    sambo samba <subcommand> [options]

SUBCOMMANDS:
    list, ls                    List all Samba shares
    create, add                 Create a new Samba share
    remove, rm, delete          Remove a Samba share
    modify, update              Modify an existing Samba share
    show                        Show details of a specific share

CREATE/MODIFY OPTIONS:
    -name <name>                Share name
    -path <path>                Share directory path
    -comment <text>             Share description
    -readonly                   Make share read-only (default: false)
    -browseable                 Make share browseable (default: true)
    -users <user1,user2>        Comma-separated list of valid users
    -type <type>                Share preset type (general, timemachine, unifi-protect, media)
    -timemachine                Enable Apple Time Machine (deprecated)
    -tmsize <size>              Time Machine max size (e.g., 500G, 1T, 0=unlimited)

REMOVE OPTIONS:
    -name <name>                Share name
    -y                          Skip the confirmation prompt

EXAMPLES:
    sambo samba list
    sambo samba create -name docs -path /mnt/documents -comment "Document Share"
    sambo samba create -name protect -path /mnt/video -type unifi-protect
    sambo samba create -name movies -path /mnt/media -type media
    sambo samba create -name backup -path /mnt/backup -type timemachine -comment "Time Machine Backup"
    sambo samba create -name backup -path /mnt/backup -type timemachine -tmsize 500G -comment "Limited TM"
    sambo samba create -name private -path /mnt/private -users alice,bob -readonly
    sambo samba modify -name docs -users alice,bob,charlie
    sambo samba remove -name docs
    sambo samba show -name docs"#
    );
}
