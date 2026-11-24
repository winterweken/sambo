//! Seams between sambo and the operating system, so the Samba manager can be
//! tested without touching real files or running real commands.

pub mod packages;

use std::fs;
use std::io;
use std::os::unix::fs::PermissionsExt;
use std::process::{Command, Stdio};

use crate::Result;

pub trait FileSystem: Send + Sync {
    fn read_to_string(&self, path: &str) -> io::Result<String>;
    fn write(&self, path: &str, data: &str) -> io::Result<()>;
    fn exists(&self, path: &str) -> bool;
    fn create_dir_all(&self, path: &str) -> io::Result<()>;
    fn rename(&self, from: &str, to: &str) -> io::Result<()>;
    fn remove_file(&self, path: &str) -> io::Result<()>;
    /// Changes the owner, leaving the group as it is.
    fn chown(&self, path: &str, uid: u32) -> io::Result<()>;
    fn chmod(&self, path: &str, mode: u32) -> io::Result<()>;
}

pub trait CommandExecutor: Send + Sync {
    /// Runs a command quietly; on failure the error carries its output.
    fn run(&self, name: &str, args: &[&str]) -> Result<()>;
    /// Whether an executable of this name is on PATH.
    fn has(&self, name: &str) -> bool;
}

pub trait Platform: Send + Sync {
    fn is_macos(&self) -> bool;
    fn samba_config_path(&self) -> String;
    fn samba_config_dir(&self) -> String;
    fn system_uid(&self, username: &str) -> Result<u32>;
    /// A warning to show when the named service ("samba", "nfs") is not usable.
    fn service_warning(&self, service: &str) -> Option<String>;
}

pub struct RealFileSystem;

impl FileSystem for RealFileSystem {
    fn read_to_string(&self, path: &str) -> io::Result<String> {
        fs::read_to_string(path)
    }
    fn write(&self, path: &str, data: &str) -> io::Result<()> {
        fs::write(path, data)
    }
    fn exists(&self, path: &str) -> bool {
        fs::metadata(path).is_ok()
    }
    fn create_dir_all(&self, path: &str) -> io::Result<()> {
        fs::create_dir_all(path)
    }
    fn rename(&self, from: &str, to: &str) -> io::Result<()> {
        fs::rename(from, to)
    }
    fn remove_file(&self, path: &str) -> io::Result<()> {
        fs::remove_file(path)
    }
    fn chown(&self, path: &str, uid: u32) -> io::Result<()> {
        std::os::unix::fs::chown(path, Some(uid), None)
    }
    fn chmod(&self, path: &str, mode: u32) -> io::Result<()> {
        fs::set_permissions(path, fs::Permissions::from_mode(mode))
    }
}

pub struct RealCommandExecutor;

impl CommandExecutor for RealCommandExecutor {
    fn run(&self, name: &str, args: &[&str]) -> Result<()> {
        run(name, args).map(|_| ())
    }
    fn has(&self, name: &str) -> bool {
        has_command(name)
    }
}

pub struct RealPlatform;

impl Platform for RealPlatform {
    fn is_macos(&self) -> bool {
        crate::platform::is_macos()
    }
    fn samba_config_path(&self) -> String {
        crate::platform::samba_config_path()
    }
    fn samba_config_dir(&self) -> String {
        crate::platform::samba_config_dir()
    }
    fn system_uid(&self, username: &str) -> Result<u32> {
        crate::user::system_uid(username)
    }
    fn service_warning(&self, service: &str) -> Option<String> {
        crate::service::warning(service)
    }
}

/// Runs a command with no terminal attached and returns its stdout.
/// Nothing is ever written to the screen, which the TUI relies on.
pub fn run(name: &str, args: &[&str]) -> Result<String> {
    let output = Command::new(name)
        .args(args)
        .stdin(Stdio::null())
        .output()
        .map_err(|e| format!("{name}: {e}"))?;
    if output.status.success() {
        return Ok(String::from_utf8_lossy(&output.stdout).into_owned());
    }
    let stderr = String::from_utf8_lossy(&output.stderr);
    let stdout = String::from_utf8_lossy(&output.stdout);
    let detail = if stderr.trim().is_empty() {
        stdout.trim()
    } else {
        stderr.trim()
    };
    if detail.is_empty() {
        Err(format!("{name} failed ({})", output.status))
    } else {
        Err(format!("{name} failed: {detail}"))
    }
}

/// True when the command exits successfully.
pub fn succeeds(name: &str, args: &[&str]) -> bool {
    run(name, args).is_ok()
}

pub fn has_command(name: &str) -> bool {
    if name.contains('/') {
        return is_executable(name);
    }
    std::env::var_os("PATH")
        .map(|paths| {
            std::env::split_paths(&paths).any(|dir| is_executable(&dir.join(name).to_string_lossy()))
        })
        .unwrap_or(false)
}

fn is_executable(path: &str) -> bool {
    fs::metadata(path)
        .map(|m| m.is_file() && m.permissions().mode() & 0o111 != 0)
        .unwrap_or(false)
}
