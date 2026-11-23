//! Input validation shared by the CLI and the TUI.

use std::path::{Component, Path};

use crate::Result;

const RESERVED_NAMES: [&str; 5] = ["global", "homes", "printers", "print$", "ipc$"];

/// System paths that must never be shared or mounted over.
const CRITICAL_PATHS: [&str; 14] = [
    "/", "/etc", "/bin", "/sbin", "/usr", "/lib", "/lib64", "/boot", "/dev", "/proc", "/sys", "/run",
    "/var/run", "/root",
];

pub fn share_name(name: &str) -> Result<()> {
    if name.is_empty() {
        return Err("share name is required".into());
    }
    if name.len() > 80 {
        return Err("share name must be 80 characters or less".into());
    }
    if !name
        .chars()
        .all(|c| c.is_ascii_alphanumeric() || c == '_' || c == '-')
    {
        return Err("share name can only contain letters, numbers, underscores, and hyphens".into());
    }
    if RESERVED_NAMES.contains(&name.to_lowercase().as_str()) {
        return Err("share name is reserved by Samba".into());
    }
    Ok(())
}

pub fn path(path: &str) -> Result<()> {
    absolute_non_critical(path, "path", "share")
}

pub fn mount_point(path: &str) -> Result<()> {
    absolute_non_critical(path, "mount point", "mount to")
}

fn absolute_non_critical(path: &str, what: &str, verb: &str) -> Result<()> {
    if path.is_empty() {
        return Err(format!("{what} is required"));
    }
    if !path.starts_with('/') {
        return Err(format!("{what} must be an absolute path"));
    }
    let clean = clean_path(path);
    if let Some(critical) = CRITICAL_PATHS.iter().find(|c| **c == clean) {
        return Err(format!("cannot {verb} critical system directory: {critical}"));
    }
    Ok(())
}

/// Lexically normalises an absolute path: collapses slashes, drops "." and
/// resolves "..", without touching the filesystem.
fn clean_path(path: &str) -> String {
    let mut parts: Vec<&str> = Vec::new();
    for component in Path::new(path).components() {
        match component {
            Component::Normal(part) => parts.push(part.to_str().unwrap_or("")),
            Component::ParentDir => {
                parts.pop();
            }
            _ => {}
        }
    }
    format!("/{}", parts.join("/"))
}

pub fn cifs_source(source: &str) -> Result<()> {
    if source.is_empty() {
        return Err("source is required".into());
    }
    let Some(rest) = source.strip_prefix("//") else {
        return Err("CIFS source must start with // (e.g., //server/share)".into());
    };
    let mut parts = rest.split('/');
    let server = parts.next().unwrap_or("");
    let share = parts.next().unwrap_or("");
    if server.is_empty() || share.is_empty() {
        return Err("CIFS source must be in format //server/share".into());
    }
    Ok(())
}

pub fn nfs_source(source: &str) -> Result<()> {
    if source.is_empty() {
        return Err("source is required".into());
    }
    let format_error = || "NFS source must be in format server:/path".to_string();
    let (server, export) = source.split_once(':').ok_or_else(format_error)?;
    if server.is_empty() || export.is_empty() {
        return Err(format_error());
    }
    if !export.starts_with('/') {
        return Err("NFS export path must be absolute (start with /)".into());
    }
    Ok(())
}

pub fn username(username: &str) -> Result<()> {
    if username.is_empty() {
        return Err("username is required".into());
    }
    if username.len() > 32 {
        return Err("username must be 32 characters or less".into());
    }
    let lower = username.to_lowercase();
    let mut chars = lower.chars();
    let first_ok = chars.next().is_some_and(|c| c.is_ascii_lowercase() || c == '_');
    let rest_ok = chars.all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == '_' || c == '-');
    if !first_ok || !rest_ok {
        return Err("username must start with a letter or underscore and contain only letters, numbers, underscores, and hyphens".into());
    }
    Ok(())
}

pub fn password(password: &str) -> Result<()> {
    if password.is_empty() {
        return Err("password is required".into());
    }
    if password.len() < 4 {
        return Err("password must be at least 4 characters".into());
    }
    Ok(())
}

/// Light check only; NFS itself validates the specification thoroughly.
pub fn nfs_clients(clients: &str) -> Result<()> {
    if clients.is_empty() {
        return Err("clients specification is required".into());
    }
    if clients.contains(|c| "[]{}()\\|;'\"".contains(c)) {
        return Err("clients specification contains invalid characters".into());
    }
    Ok(())
}

#[cfg(test)]
mod tests;
