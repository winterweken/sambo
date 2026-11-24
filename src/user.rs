//! Samba user management (pdbedit / smbpasswd) and the system accounts behind it.

use std::fs;
use std::io::{ErrorKind, Write};
use std::process::{Command, Stdio};

use crate::platform;
use crate::system::run;
use crate::{validate, Result};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct User {
    pub username: String,
    pub uid: u32,
    pub enabled: bool,
}

/// Parses `pdbedit -L -v` output. UIDs are not part of it and are left at 0.
pub fn parse_pdbedit(output: &str) -> Vec<User> {
    let mut users: Vec<User> = Vec::new();
    for line in output.lines().map(str::trim) {
        if let Some(name) = line.strip_prefix("Unix username:") {
            users.push(User {
                username: name.trim().to_string(),
                uid: 0,
                enabled: true,
            });
        } else if let Some((_, flags)) = line.split_once("Account Flags:") {
            if let Some(user) = users.last_mut() {
                if flags.contains('D') {
                    user.enabled = false;
                }
            }
        }
    }
    users
}

/// Looks a user up in /etc/passwd content.
pub fn parse_passwd_uid(content: &str, username: &str) -> Option<u32> {
    content.lines().find_map(|line| {
        let mut fields = line.split(':');
        if fields.next()? != username {
            return None;
        }
        fields.nth(1)?.parse().ok()
    })
}

/// Parses `dscl . -read /Users/<name> UniqueID` output ("UniqueID: 501").
pub fn parse_dscl_uid(output: &str) -> Option<u32> {
    output
        .lines()
        .find_map(|line| line.strip_prefix("UniqueID:")?.trim().parse().ok())
}

/// Next free UID above the regular users in `dscl . -list /Users UniqueID` output.
pub fn next_macos_uid(listing: &str) -> u32 {
    listing
        .lines()
        .filter_map(|line| line.split_whitespace().last()?.parse::<u32>().ok())
        .filter(|uid| *uid < 65534)
        .fold(500, u32::max)
        + 1
}

pub fn system_uid(username: &str) -> Result<u32> {
    let found = if platform::is_macos() {
        run("dscl", &[".", "-read", &format!("/Users/{username}"), "UniqueID"])
            .ok()
            .and_then(|out| parse_dscl_uid(&out))
    } else {
        fs::read_to_string("/etc/passwd")
            .ok()
            .and_then(|c| parse_passwd_uid(&c, username))
    };
    found.ok_or_else(|| "user not found".to_string())
}

pub fn list() -> Result<Vec<User>> {
    let output = Command::new("pdbedit")
        .args(["-L", "-v"])
        .stdin(Stdio::null())
        .stderr(Stdio::null())
        .output()
        .map_err(|e| match e.kind() {
            ErrorKind::NotFound => {
                "failed to list users: pdbedit not found (is Samba installed?)".to_string()
            }
            _ => format!("failed to list users: {e}"),
        })?;
    if !output.status.success() {
        // pdbedit exits non-zero when the user database is empty.
        return Ok(Vec::new());
    }
    let mut users = parse_pdbedit(&String::from_utf8_lossy(&output.stdout));
    for user in &mut users {
        user.uid = system_uid(&user.username).unwrap_or(0);
    }
    Ok(users)
}

pub fn get(username: &str) -> Result<User> {
    list()?
        .into_iter()
        .find(|u| u.username == username)
        .ok_or_else(|| format!("user '{username}' not found"))
}

pub fn add(username: &str, password: &str, create_system: bool) -> Result<()> {
    validate::username(username).map_err(|e| format!("invalid username: {e}"))?;
    validate::password(password).map_err(|e| format!("invalid password: {e}"))?;
    if get(username).is_ok() {
        return Err(format!("user '{username}' already exists"));
    }
    if create_system {
        create_system_user(username).map_err(|e| format!("failed to create system user: {e}"))?;
    } else if system_uid(username).is_err() {
        return Err(format!(
            "system user '{username}' does not exist. Use -create-system flag or create manually"
        ));
    }
    smbpasswd(&["-a", username], password).map_err(|e| format!("failed to add samba user: {e}"))?;
    run("smbpasswd", &["-e", username]).map_err(|e| format!("failed to enable user: {e}"))?;
    Ok(())
}

pub fn remove(username: &str, remove_system: bool) -> Result<()> {
    get(username)?;
    run("smbpasswd", &["-x", username]).map_err(|e| format!("failed to remove samba user: {e}"))?;
    if remove_system {
        let result = if platform::is_macos() {
            run("dscl", &[".", "-delete", &format!("/Users/{username}")])
        } else {
            run("userdel", &[username])
        };
        result.map_err(|e| format!("failed to remove system user: {e}"))?;
    }
    Ok(())
}

pub fn set_password(username: &str, password: &str) -> Result<()> {
    validate::password(password).map_err(|e| format!("invalid password: {e}"))?;
    get(username)?;
    smbpasswd(&[username], password).map_err(|e| format!("failed to set password: {e}"))
}

/// Runs smbpasswd, feeding the password twice on stdin so it never appears in
/// the process list.
fn smbpasswd(args: &[&str], password: &str) -> Result<()> {
    let mut child = Command::new("smbpasswd")
        .args(args)
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("failed to start smbpasswd: {e}"))?;
    if let Some(mut stdin) = child.stdin.take() {
        let _ = write!(stdin, "{password}\n{password}\n");
    }
    let output = child.wait_with_output().map_err(|e| e.to_string())?;
    if output.status.success() {
        Ok(())
    } else {
        Err(String::from_utf8_lossy(&output.stderr).trim().to_string())
    }
}

fn create_system_user(username: &str) -> Result<()> {
    if system_uid(username).is_ok() {
        return Ok(());
    }
    if !platform::is_macos() {
        // No home directory and no login shell: the account only exists for Samba.
        run("useradd", &["-M", "-s", "/usr/sbin/nologin", username])?;
        return Ok(());
    }
    let listing = run("dscl", &[".", "-list", "/Users", "UniqueID"])
        .map_err(|e| format!("failed to find available UID: {e}"))?;
    let uid = next_macos_uid(&listing).to_string();
    let record = format!("/Users/{username}");
    let steps: [&[&str]; 5] = [
        &[],
        &["UserShell", "/usr/bin/false"],
        &["UniqueID", &uid],
        &["PrimaryGroupID", "20"], // staff
        &["NFSHomeDirectory", "/var/empty"],
    ];
    for extra in steps {
        let mut args = vec![".", "-create", record.as_str()];
        args.extend_from_slice(extra);
        run("dscl", &args)?;
    }
    Ok(())
}

#[cfg(test)]
mod tests;
