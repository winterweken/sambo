//! OS detection and per-platform paths.

use std::path::Path;

pub fn is_macos() -> bool {
    cfg!(target_os = "macos")
}

/// Path to smb.conf. On macOS this follows the Homebrew prefix.
pub fn samba_config_path() -> String {
    if is_macos() {
        if Path::new("/opt/homebrew/etc/smb.conf").exists() {
            return "/opt/homebrew/etc/smb.conf".into();
        }
        return "/usr/local/etc/smb.conf".into();
    }
    "/etc/samba/smb.conf".into()
}

pub fn samba_config_dir() -> String {
    if is_macos() {
        if Path::new("/opt/homebrew/bin/brew").exists() {
            return "/opt/homebrew/etc".into();
        }
        return "/usr/local/etc".into();
    }
    "/etc/samba".into()
}

pub fn nfs_exports_path() -> &'static str {
    "/etc/exports"
}

/// fstab is only used for network mounts on Linux.
pub fn fstab_path() -> Option<&'static str> {
    if is_macos() {
        None
    } else {
        Some("/etc/fstab")
    }
}

/// auto_nfs holds persistent NFS mounts on macOS.
pub fn auto_nfs_path() -> Option<&'static str> {
    if is_macos() {
        Some("/etc/auto_nfs")
    } else {
        None
    }
}
