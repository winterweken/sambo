//! NFS export management (/etc/exports).

use std::fs;
use std::path::Path;

use crate::platform;
use crate::system::{has_command, run};
use crate::{service, validate, Result};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Export {
    pub path: String,
    /// Space-separated client specifications.
    pub clients: String,
    pub options: String,
}

/// Builds the standard options string used by both the CLI and the TUI.
pub fn build_options(read_only: bool, async_mode: bool, no_root_squash: bool) -> String {
    [
        if read_only { "ro" } else { "rw" },
        if async_mode { "async" } else { "sync" },
        if no_root_squash {
            "no_root_squash"
        } else {
            "root_squash"
        },
        "no_subtree_check",
    ]
    .join(",")
}

/// Reads `(read_only, async_mode, no_root_squash)` back out of an options string.
pub fn option_flags(options: &str) -> (bool, bool, bool) {
    let has = |wanted: &str| options.split(',').any(|o| o.trim() == wanted);
    (has("ro"), has("async"), has("no_root_squash"))
}

/// Parses `/path client(options) [client(options)...]`. When several clients
/// are listed, the first client's options are kept.
pub fn parse_export_line(line: &str) -> Option<Export> {
    let mut parts = line.split_whitespace();
    let path = parts.next()?;
    let specs: Vec<&str> = parts.collect();
    if specs.is_empty() {
        return None;
    }
    let clients: Vec<&str> = specs.iter().map(|s| s.split('(').next().unwrap_or("")).collect();
    let options = specs[0]
        .split_once('(')
        .map(|(_, o)| o.trim_end_matches(')'))
        .unwrap_or("");
    Some(Export {
        path: path.into(),
        clients: clients.join(" "),
        options: options.into(),
    })
}

fn is_ignored(line: &str) -> bool {
    let trimmed = line.trim();
    trimmed.is_empty() || trimmed.starts_with('#')
}

/// Parses an exports file. Lines for the same path are merged into one export,
/// in the order paths first appear.
pub fn parse_exports(content: &str) -> Vec<Export> {
    let mut exports: Vec<Export> = Vec::new();
    for export in content
        .lines()
        .filter(|l| !is_ignored(l))
        .filter_map(|l| parse_export_line(l.trim()))
    {
        match exports.iter_mut().find(|e| e.path == export.path) {
            Some(existing) => {
                existing.clients.push(' ');
                existing.clients.push_str(&export.clients);
            }
            None => exports.push(export),
        }
    }
    exports
}

/// Drops every line exporting `path`, keeping comments and other exports.
pub fn remove_export_lines(content: &str, path: &str) -> String {
    content
        .split('\n')
        .filter(|line| is_ignored(line) || parse_export_line(line.trim()).is_none_or(|e| e.path != path))
        .collect::<Vec<_>>()
        .join("\n")
}

/// The exports line for an export. Options bind to the client they follow,
/// so each client gets its own copy.
pub fn export_line(export: &Export) -> String {
    let clients: Vec<String> = (export.clients.split_whitespace())
        .map(|client| format!("{client}({})", export.options))
        .collect();
    format!("{} {}\n", export.path, clients.join(" "))
}

fn backup_path() -> String {
    format!("{}.backup", platform::nfs_exports_path())
}

/// Verifies the NFS server is present and makes sure the exports file exists.
pub fn check_installed() -> Result<()> {
    if platform::is_macos() {
        if !has_command("nfsd") {
            return Err("NFS server (nfsd) not found. This should be built into macOS.".into());
        }
    } else if !has_command("exportfs") {
        return Err("NFS server is not installed.\n\nPlease install NFS server:\n  Debian/Ubuntu: sudo apt-get install nfs-kernel-server\n  RHEL/CentOS:   sudo yum install nfs-utils\n  Arch:          sudo pacman -S nfs-utils".into());
    }
    let exports = platform::nfs_exports_path();
    if !Path::new(exports).exists() {
        fs::write(exports, "# NFS exports\n")
            .map_err(|e| format!("NFS is installed but cannot create exports file: {e}"))?;
    }
    Ok(())
}

pub fn list() -> Result<Vec<Export>> {
    let path = platform::nfs_exports_path();
    match fs::read_to_string(path) {
        Ok(content) => Ok(parse_exports(&content)),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(Vec::new()),
        Err(e) => Err(format!("failed to open {path}: {e}")),
    }
}

pub fn get(path: &str) -> Result<Export> {
    list()?
        .into_iter()
        .find(|e| e.path == path)
        .ok_or_else(|| format!("export '{path}' not found"))
}

/// Adds an export and reloads NFS. Returns warnings worth showing the user.
pub fn create(export: &Export) -> Result<Vec<String>> {
    let warnings: Vec<String> = service::warning("nfs").into_iter().collect();

    validate::path(&export.path).map_err(|e| format!("invalid export path: {e}"))?;
    validate::nfs_clients(&export.clients).map_err(|e| format!("invalid clients: {e}"))?;
    if get(&export.path).is_ok() {
        return Err(format!("export '{}' already exists", export.path));
    }
    if !Path::new(&export.path).exists() {
        return Err(format!("path '{}' does not exist", export.path));
    }

    backup()?;
    let exports_path = platform::nfs_exports_path();
    let mut content = fs::read_to_string(exports_path).unwrap_or_default();
    if !content.is_empty() && !content.ends_with('\n') {
        content.push('\n');
    }
    content.push_str(&export_line(export));
    fs::write(exports_path, content).map_err(|e| format!("failed to write export: {e}"))?;

    reload()?;
    Ok(warnings)
}

pub fn remove(path: &str) -> Result<()> {
    get(path)?;
    backup()?;
    let exports_path = platform::nfs_exports_path();
    let content = fs::read_to_string(exports_path).map_err(|e| format!("failed to read exports: {e}"))?;
    if let Err(e) = fs::write(exports_path, remove_export_lines(&content, path)) {
        restore();
        return Err(format!("failed to write exports: {e}"));
    }
    reload()
}

/// Replaces an export's clients and/or options.
pub fn modify(path: &str, clients: Option<String>, options: Option<String>) -> Result<Vec<String>> {
    let mut export = get(path)?;
    if let Some(clients) = clients {
        export.clients = clients;
    }
    if let Some(options) = options {
        export.options = options;
    }
    // Check what `create` will check before the old entry is removed, so a
    // rejected change cannot leave the export deleted.
    validate::nfs_clients(&export.clients).map_err(|e| format!("invalid clients: {e}"))?;
    if !Path::new(&export.path).exists() {
        return Err(format!("path '{}' does not exist", export.path));
    }
    remove(path)?;
    create(&export)
}

fn backup() -> Result<()> {
    let exports_path = platform::nfs_exports_path();
    if !Path::new(exports_path).exists() {
        return fs::write(exports_path, "").map_err(|e| format!("failed to create exports file: {e}"));
    }
    fs::copy(exports_path, backup_path()).map_err(|e| format!("failed to create backup: {e}"))?;
    Ok(())
}

fn restore() {
    let _ = fs::copy(backup_path(), platform::nfs_exports_path());
}

fn reload() -> Result<()> {
    let attempts: &[(&str, &[&str])] = if platform::is_macos() {
        &[("nfsd", &["update"]), ("nfsd", &["restart"])]
    } else {
        &[
            ("exportfs", &["-ra"]),
            ("systemctl", &["restart", "nfs-server"]),
            ("systemctl", &["restart", "nfs-kernel-server"]),
        ]
    };
    let mut last_error = String::new();
    for (name, args) in attempts {
        match run(name, args) {
            Ok(_) => return Ok(()),
            Err(e) => last_error = e,
        }
    }
    Err(format!("failed to reload NFS exports: {last_error}"))
}

#[cfg(test)]
mod tests;
