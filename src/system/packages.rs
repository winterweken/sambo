//! Package manager detection and installation of sambo's dependencies.

use crate::platform;
use crate::system::{has_command, run, succeeds};
use crate::Result;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PackageManager {
    Apt,
    Yum,
    Dnf,
    Pacman,
    Zypper,
    Apk,
    Homebrew,
    Unknown,
}

use PackageManager::*;

impl PackageManager {
    pub fn name(self) -> &'static str {
        match self {
            Apt => "apt",
            Yum => "yum",
            Dnf => "dnf",
            Pacman => "pacman",
            Zypper => "zypper",
            Apk => "apk",
            Homebrew => "homebrew",
            Unknown => "unknown",
        }
    }
}

pub fn detect_package_manager() -> PackageManager {
    if platform::is_macos() {
        return if has_command("brew") { Homebrew } else { Unknown };
    }
    [
        (Apt, "apt-get"),
        (Dnf, "dnf"),
        (Yum, "yum"),
        (Pacman, "pacman"),
        (Zypper, "zypper"),
        (Apk, "apk"),
    ]
    .into_iter()
    .find(|(_, cmd)| has_command(cmd))
    .map(|(pm, _)| pm)
    .unwrap_or(Unknown)
}

pub fn is_samba_installed() -> bool {
    if platform::is_macos() {
        return has_command("smbd") || succeeds("launchctl", &["list", "com.apple.smbd"]);
    }
    ["smbd", "smbpasswd", "testparm"]
        .iter()
        .all(|cmd| has_command(cmd))
}

pub fn is_nfs_installed() -> bool {
    if platform::is_macos() {
        return has_command("nfsd");
    }
    if !has_command("exportfs") {
        return false;
    }
    systemd_units().is_some_and(|units| {
        units.contains("nfs-server.service") || units.contains("nfs-kernel-server.service")
    })
}

pub fn is_cifs_installed() -> bool {
    has_command(if platform::is_macos() {
        "mount_smbfs"
    } else {
        "mount.cifs"
    })
}

pub fn is_nfs_client_installed() -> bool {
    has_command(if platform::is_macos() {
        "mount_nfs"
    } else {
        "mount.nfs"
    })
}

/// Avahi provides the mDNS advertisement Time Machine needs; macOS has Bonjour built in.
pub fn is_avahi_installed() -> bool {
    platform::is_macos() || has_command("avahi-daemon")
}

pub fn samba_packages(pm: PackageManager) -> Vec<&'static str> {
    match pm {
        Yum | Dnf => vec!["samba", "samba-common-tools"],
        _ => vec!["samba"],
    }
}

pub fn nfs_packages(pm: PackageManager) -> Vec<&'static str> {
    match pm {
        Homebrew => vec![], // built into macOS
        Apt | Zypper => vec!["nfs-kernel-server"],
        _ => vec!["nfs-utils"],
    }
}

pub fn cifs_packages(pm: PackageManager) -> Vec<&'static str> {
    match pm {
        Homebrew => vec![], // built into macOS
        _ => vec!["cifs-utils"],
    }
}

pub fn nfs_client_packages(pm: PackageManager) -> Vec<&'static str> {
    match pm {
        Homebrew => vec![], // built into macOS
        Apt => vec!["nfs-common"],
        Zypper => vec!["nfs-client"],
        _ => vec!["nfs-utils"],
    }
}

pub fn avahi_packages(pm: PackageManager) -> Vec<&'static str> {
    match pm {
        Homebrew => vec![], // built into macOS
        Apt => vec!["avahi-daemon"],
        Apk => vec!["avahi", "dbus"],
        _ => vec!["avahi"],
    }
}

/// The command line (program first) that installs `packages`.
pub fn install_command(pm: PackageManager, packages: &[&str]) -> Result<Vec<String>> {
    let prefix: &[&str] = match pm {
        Homebrew => &["brew", "install"],
        Apt => &["apt-get", "install", "-y"],
        Dnf => &["dnf", "install", "-y"],
        Yum => &["yum", "install", "-y"],
        Pacman => &["pacman", "-S", "--noconfirm"],
        Zypper => &["zypper", "install", "-y"],
        Apk => &["apk", "add", "--no-cache"],
        Unknown => return Err("unable to detect package manager".into()),
    };
    Ok(prefix.iter().chain(packages).map(|s| s.to_string()).collect())
}

pub fn install_packages(packages: &[&str]) -> Result<()> {
    if packages.is_empty() {
        return Ok(()); // built-in on this platform
    }
    let pm = detect_package_manager();
    let command = install_command(pm, packages)?;
    if pm == Apt {
        run("apt-get", &["update"]).map_err(|e| format!("failed to update package list: {e}"))?;
    }
    let args: Vec<&str> = command[1..].iter().map(String::as_str).collect();
    run(&command[0], &args).map_err(|e| format!("installation failed: {e}"))?;
    Ok(())
}

/// Enables the service at boot and starts it now.
pub fn enable_service(service: &str) -> Result<()> {
    if !platform::is_macos() {
        run("systemctl", &["enable", service]).map_err(|e| format!("failed to enable {service}: {e}"))?;
        run("systemctl", &["start", service]).map_err(|e| format!("failed to start {service}: {e}"))?;
        return Ok(());
    }
    match service {
        "smbd" | "smb" | "samba" => {
            if succeeds("brew", &["services", "start", "samba"]) {
                return Ok(());
            }
            run("smbd", &["-D"]).map(|_| ())
        }
        "nfsd" | "nfs-server" | "nfs" => {
            run("nfsd", &["enable"]).map_err(|e| format!("failed to enable nfsd: {e}"))?;
            run("nfsd", &["start"]).map_err(|e| format!("failed to start nfsd: {e}"))?;
            Ok(())
        }
        other => {
            let plist = format!("/System/Library/LaunchDaemons/{other}.plist");
            if succeeds("launchctl", &["load", "-w", &plist]) {
                return Ok(());
            }
            run("brew", &["services", "start", other]).map(|_| ())
        }
    }
}

fn systemd_units() -> Option<String> {
    run("systemctl", &["list-unit-files", "--type=service"]).ok()
}

pub fn samba_service_name() -> &'static str {
    if platform::is_macos() {
        return "samba"; // Homebrew service name
    }
    samba_service_from_units(&systemd_units().unwrap_or_default())
}

pub fn nfs_service_name() -> &'static str {
    if platform::is_macos() {
        return "nfsd";
    }
    nfs_service_from_units(&systemd_units().unwrap_or_default())
}

/// Picks the Samba unit name from `systemctl list-unit-files` output.
pub fn samba_service_from_units(units: &str) -> &'static str {
    if !units.contains("smbd.service") && units.contains("smb.service") {
        "smb"
    } else {
        "smbd"
    }
}

/// Picks the NFS server unit name from `systemctl list-unit-files` output.
pub fn nfs_service_from_units(units: &str) -> &'static str {
    if units.contains("nfs-kernel-server.service") {
        "nfs-kernel-server"
    } else {
        "nfs-server"
    }
}

/// None on macOS, where mDNSResponder is always running.
pub fn avahi_service_name() -> Option<&'static str> {
    if platform::is_macos() {
        None
    } else {
        Some("avahi-daemon")
    }
}

#[cfg(test)]
mod tests;
