//! Small helpers shared by the CLI commands.

use std::io::{self, BufRead, Write};

pub const RULE: &str = "──────────────────────────────────────────────────────────";

pub fn parse_csv(s: &str) -> Vec<String> {
    s.split(',')
        .map(str::trim)
        .filter(|p| !p.is_empty())
        .map(str::to_string)
        .collect()
}

/// Builds the mount options for a CIFS share: any extra options, then the
/// credentials, then uid/gid (the invoking sudo user's when known) unless the
/// extra options already set them.
pub fn cifs_options(
    extra: &str,
    username: &str,
    password: &str,
    sudo_uid: Option<&str>,
    sudo_gid: Option<&str>,
) -> String {
    let mut opts: Vec<String> = Vec::new();
    if !extra.is_empty() {
        opts.push(extra.to_string());
    }
    if username.is_empty() {
        opts.push("credentials=/root/.smbcredentials".into());
    } else {
        opts.push(format!("username={username}"));
        if !password.is_empty() {
            opts.push(format!("password={password}"));
        }
    }
    let given = |name: &str| extra.split(',').any(|option| option.trim().starts_with(name));
    if !given("uid=") {
        opts.push(format!("uid={}", sudo_uid.unwrap_or("1000")));
    }
    if !given("gid=") {
        opts.push(format!("gid={}", sudo_gid.unwrap_or("1000")));
    }
    opts.join(",")
}

/// The uid and gid of the user who ran sudo, read through `env` from
/// SUDO_UID and SUDO_GID. Anything that is not a plain number is ignored,
/// because the values end up in mount options.
pub fn sudo_ids(env: impl Fn(&str) -> Option<String>) -> (Option<String>, Option<String>) {
    let id = |name: &str| env(name).filter(|v| !v.is_empty() && v.bytes().all(|b| b.is_ascii_digit()));
    (id("SUDO_UID"), id("SUDO_GID"))
}

/// Asks for confirmation of a destructive action; `skip` answers yes.
pub fn confirm(message: &str, skip: bool) -> bool {
    if skip {
        return true;
    }
    print!("{message} [y/N]: ");
    let _ = io::stdout().flush();
    let mut response = String::new();
    if io::stdin().lock().read_line(&mut response).is_err() {
        return false;
    }
    matches!(response.trim().to_lowercase().as_str(), "y" | "yes")
}

pub fn print_warnings(warnings: &[String]) {
    for warning in warnings {
        println!("⚠️  Warning: {warning}");
    }
}

#[cfg(test)]
mod tests;
