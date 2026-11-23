//! Service status checks for the Samba and NFS servers.

use crate::platform;
use crate::system::{has_command, run, succeeds};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Status {
    Running,
    Stopped,
    NotInstalled,
}

pub fn check_samba() -> Status {
    if platform::is_macos() {
        if !has_command("smbd") {
            // Fall back to macOS built-in file sharing.
            if succeeds("launchctl", &["list", "com.apple.smbd"]) {
                return Status::Running;
            }
            return Status::NotInstalled;
        }
        if succeeds("pgrep", &["-x", "smbd"]) {
            return Status::Running;
        }
        if let Ok(list) = run("launchctl", &["list"]) {
            if list.contains("smbd") || list.contains("samba") {
                return Status::Running;
            }
        }
        return Status::Stopped;
    }
    if ["smbd", "samba", "smb"].iter().any(|s| systemd_active(s)) {
        return Status::Running;
    }
    if has_command("smbd") {
        Status::Stopped
    } else {
        Status::NotInstalled
    }
}

pub fn check_nfs() -> Status {
    if platform::is_macos() {
        if let Ok(out) = run("nfsd", &["status"]) {
            if out.contains("is running") {
                return Status::Running;
            }
        }
        return if has_command("nfsd") {
            Status::Stopped
        } else {
            Status::NotInstalled
        };
    }
    if ["nfs-server", "nfs-kernel-server", "nfs"]
        .iter()
        .any(|s| systemd_active(s))
    {
        return Status::Running;
    }
    if has_command("exportfs") {
        Status::Stopped
    } else {
        Status::NotInstalled
    }
}

fn systemd_active(name: &str) -> bool {
    succeeds("systemctl", &["is-active", "--quiet", name])
}

/// A warning for the named service ("samba" or "nfs") when changes may not
/// take effect, or None when it is running.
pub fn warning(service: &str) -> Option<String> {
    let status = match service {
        "samba" => check_samba(),
        "nfs" => check_nfs(),
        _ => return None,
    };
    match status {
        Status::Running => None,
        Status::Stopped => Some(format!(
            "{service} service is not running. Changes may not take effect until service is started."
        )),
        Status::NotInstalled => Some(format!("{service} is not installed.")),
    }
}
