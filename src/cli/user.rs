use super::flags;
use super::util::{confirm, RULE};
use crate::user::{self, User};
use crate::Result;

pub fn handle(args: &[String]) -> Result<()> {
    let Some(subcommand) = args.first() else {
        print_usage();
        return Ok(());
    };
    let rest = &args[1..];
    match subcommand.as_str() {
        "list" | "ls" => list(),
        "add" | "create" => add(rest),
        "remove" | "rm" | "delete" => remove(rest),
        "passwd" | "password" => password(rest),
        "show" => show(rest),
        "-h" | "--help" | "help" => {
            print_usage();
            Ok(())
        }
        other => {
            print_usage();
            Err(format!("unknown user subcommand: {other}"))
        }
    }
}

fn print_user(user: &User) {
    println!("Username:    {}", user.username);
    println!("UID:         {}", user.uid);
    println!("Enabled:     {}", user.enabled);
}

fn list() -> Result<()> {
    let users = user::list().map_err(|e| format!("failed to list users: {e}"))?;
    if users.is_empty() {
        println!("No Samba users configured");
        return Ok(());
    }
    println!("Samba Users:\n{RULE}");
    for user in &users {
        print_user(user);
        println!("{RULE}");
    }
    Ok(())
}

/// Reads the required -username and -password flags.
fn credentials(f: &flags::Flags) -> Result<(String, String)> {
    let (username, password) = (f.string("username", ""), f.string("password", ""));
    if username.is_empty() || password.is_empty() {
        print_usage();
        return Err("username and password are required".into());
    }
    Ok((username, password))
}

fn required_username(f: &flags::Flags) -> Result<String> {
    let username = f.string("username", "");
    if username.is_empty() {
        print_usage();
        return Err("username is required".into());
    }
    Ok(username)
}

fn add(args: &[String]) -> Result<()> {
    let f = flags::parse(args, &["username", "password"], &["create-system"])?;
    let (username, password) = credentials(&f)?;
    user::add(&username, &password, f.boolean("create-system", true))
        .map_err(|e| format!("failed to add user: {e}"))?;
    println!("User '{username}' added successfully");
    Ok(())
}

fn remove(args: &[String]) -> Result<()> {
    let f = flags::parse(args, &["username"], &["remove-system", "y"])?;
    let username = required_username(&f)?;
    let remove_system = f.boolean("remove-system", false);
    let yes = f.boolean("y", false);

    let message = if remove_system {
        if !yes {
            println!("\n⚠️  WARNING: This action is potentially destructive!");
        }
        format!("Remove user '{username}' and system account?")
    } else {
        format!("Remove Samba user '{username}'?")
    };
    if !confirm(&message, yes) {
        println!("Cancelled");
        return Ok(());
    }
    user::remove(&username, remove_system).map_err(|e| format!("failed to remove user: {e}"))?;
    println!("User '{username}' removed successfully");
    Ok(())
}

fn password(args: &[String]) -> Result<()> {
    let (username, password) = credentials(&flags::parse(args, &["username", "password"], &[])?)?;
    user::set_password(&username, &password).map_err(|e| format!("failed to set password: {e}"))?;
    println!("Password for user '{username}' updated successfully");
    Ok(())
}

fn show(args: &[String]) -> Result<()> {
    let username = required_username(&flags::parse(args, &["username"], &[])?)?;
    print_user(&user::get(&username).map_err(|e| format!("failed to get user: {e}"))?);
    Ok(())
}

fn print_usage() {
    println!(
        "User Management

USAGE:
    sambo user <subcommand> [options]

SUBCOMMANDS:
    list, ls                    List all Samba users
    add, create                 Add a new Samba user
    remove, rm, delete          Remove a Samba user
    passwd, password            Change user password
    show                        Show details of a specific user

ADD OPTIONS:
    -username <name>            Username (required)
    -password <pass>            Password (required)
    -create-system              Create system user if not exists (default: true)

REMOVE OPTIONS:
    -username <name>            Username (required)
    -remove-system              Also remove system user (default: false)
    -y                          Skip the confirmation prompt

PASSWORD OPTIONS:
    -username <name>            Username (required)
    -password <pass>            New password (required)

EXAMPLES:
    sambo user list
    sambo user add -username alice -password secret123
    sambo user passwd -username alice -password newsecret456
    sambo user remove -username alice
    sambo user show -username alice"
    );
}
