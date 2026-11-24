//! Client-side network mounts (CIFS/SMB and NFS).

pub mod discover;

use std::collections::{HashMap, HashSet};
use std::fs;
use std::io::Write;
use std::os::unix::fs::{DirBuilderExt, OpenOptionsExt, PermissionsExt};
use std::path::{Path, PathBuf};

use crate::platform;
use crate::system::run;
use crate::{validate, Result};

/// Where passwords of CIFS mounts are kept, readable by root only.
const CREDENTIALS_DIR: &str = "/root/.sambo";

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MountType {
    Cifs,
    Nfs,
}

impl MountType {
    pub fn as_str(self) -> &'static str {
        match self {
            MountType::Cifs => "cifs",
            MountType::Nfs => "nfs",
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Mount {
    /// `//server/share` or `server:/path`.
    pub source: String,
    pub mount_point: String,
    pub kind: MountType,
    pub options: String,
    /// Listed in fstab (Linux) or auto_nfs (macOS), so it survives a reboot.
    pub persistent: bool,
    pub active: bool,
}

fn is_ignored(line: &str) -> bool {
    let trimmed = line.trim();
    trimmed.is_empty() || trimmed.starts_with('#')
}

/// The `(source, mount point)` pairs listed in fstab content.
pub fn parse_fstab_keys(content: &str) -> HashSet<(String, String)> {
    content
        .lines()
        .filter(|l| !is_ignored(l))
        .filter_map(|line| {
            let mut fields = line.split_whitespace();
            Some((fields.next()?.to_string(), fields.next()?.to_string()))
        })
        .collect()
}

/// Network mounts in /proc/mounts content.
pub fn parse_proc_mounts(content: &str, fstab: &HashSet<(String, String)>) -> Vec<Mount> {
    content
        .lines()
        .filter_map(|line| {
            let fields: Vec<&str> = line.split_whitespace().collect();
            let kind = match *fields.get(2)? {
                "cifs" => MountType::Cifs,
                "nfs" | "nfs4" => MountType::Nfs,
                _ => return None,
            };
            let (source, mount_point) = (fields[0].to_string(), fields[1].to_string());
            Some(Mount {
                persistent: fstab.contains(&(source.clone(), mount_point.clone())),
                source,
                mount_point,
                kind,
                options: fields.get(3).unwrap_or(&"").to_string(),
                active: true,
            })
        })
        .collect()
}

/// Maps mount point to source for the entries in macOS auto_nfs content
/// (`/mount/point -options server:/path`).
pub fn parse_auto_nfs(content: &str) -> HashMap<String, String> {
    content
        .lines()
        .filter(|l| !is_ignored(l))
        .filter_map(|line| {
            let fields: Vec<&str> = line.split_whitespace().collect();
            (fields.len() >= 2).then(|| (fields[0].to_string(), fields[fields.len() - 1].to_string()))
        })
        .collect()
}

/// Network mounts in macOS `mount` output (`source on /path (type, options)`).
pub fn parse_macos_mounts(output: &str, auto_nfs: &HashMap<String, String>) -> Vec<Mount> {
    output
        .lines()
        .filter_map(|line| {
            // autofs entries are mount triggers, not mounts.
            if line.contains("(autofs") {
                return None;
            }
            let is_smb = line.contains("(smbfs") || line.contains("(cifs");
            let is_nfs = line.contains("(nfs");
            if !is_smb && !is_nfs {
                return None;
            }
            let (source, rest) = line.split_once(" on ")?;
            let mount_point = &rest[..rest.rfind(" (")?];
            Some(Mount {
                source: source.to_string(),
                mount_point: mount_point.to_string(),
                kind: if is_smb { MountType::Cifs } else { MountType::Nfs },
                options: String::new(),
                persistent: is_nfs && auto_nfs.contains_key(mount_point),
                active: true,
            })
        })
        .collect()
}

/// The fstab line for a network mount. Empty options get a sensible default,
/// and `_netdev` is added so the mount waits for the network at boot.
pub fn fstab_line(source: &str, mount_point: &str, fs_type: &str, options: &str) -> String {
    let mut options = match (options, fs_type) {
        ("", "cifs") => "credentials=/root/.smbcredentials,uid=1000,gid=1000".to_string(),
        ("", _) => "defaults".to_string(),
        (given, _) => given.to_string(),
    };
    if !options.contains("_netdev") {
        options.push_str(",_netdev");
    }
    format!("{source} {mount_point} {fs_type} {options} 0 0\n")
}

/// Separates `username=`/`password=` from CIFS mount options when a password
/// is present, so it can go into a private credentials file instead of the
/// world-readable fstab and the process list.
pub fn split_credentials(options: &str) -> (String, Option<(String, String)>) {
    let value = |key: &str| options.split(',').find_map(|option| option.strip_prefix(key));
    let Some(password) = value("password=") else {
        return (options.to_string(), None);
    };
    let username = value("username=").unwrap_or_default();
    let rest: Vec<&str> = options
        .split(',')
        .filter(|option| !option.starts_with("username=") && !option.starts_with("password="))
        .collect();
    (rest.join(","), Some((username.to_string(), password.to_string())))
}

/// A file name for the credentials of one mount, safe whatever the source
/// contains. The suffix keeps mounts of the same share at different mount
/// points, and sources that differ only in replaced characters, apart.
pub fn credentials_file_name(source: &str, mount_point: &str) -> String {
    let safe = |c: char| {
        if c.is_ascii_alphanumeric() || c == '.' || c == '-' {
            c
        } else {
            '_'
        }
    };
    let name: String = source.trim_start_matches('/').chars().map(safe).collect();
    // FNV-1a: the name must stay the same across builds, which the standard
    // library's hasher does not promise.
    let hash = (source.bytes().chain([0]).chain(mount_point.bytes()))
        .fold(0xcbf2_9ce4_8422_2325_u64, |hash, byte| {
            (hash ^ u64::from(byte)).wrapping_mul(0x0100_0000_01b3)
        });
    format!("{name}-{hash:016x}.cred")
}

/// The content of a credentials file as mount.cifs expects it.
pub fn credentials_content(username: &str, password: &str) -> String {
    let user_line = if username.is_empty() {
        String::new()
    } else {
        format!("username={username}\n")
    };
    format!("{user_line}password={password}\n")
}

/// A credentials file written for a mount, with what it replaced.
#[derive(Debug)]
struct StoredCredentials {
    path: PathBuf,
    previous: Option<Vec<u8>>,
}

impl StoredCredentials {
    /// Puts back what was there before, for when the mount did not succeed.
    fn undo(&self) {
        let _ = match &self.previous {
            Some(previous) => fs::write(&self.path, previous),
            None => fs::remove_file(&self.path),
        };
    }
}

/// Moves a password given in CIFS options into a root-only credentials file
/// in `dir` and returns options that refer to that file instead.
fn protect_cifs_password(
    dir: &Path,
    source: &str,
    mount_point: &str,
    options: &str,
) -> Result<(String, Option<StoredCredentials>)> {
    let (rest, Some((username, password))) = split_credentials(options) else {
        return Ok((options.to_string(), None));
    };
    let failed = |e: std::io::Error| format!("failed to store credentials: {e}");
    fs::DirBuilder::new()
        .recursive(true)
        .mode(0o700)
        .create(dir)
        .map_err(failed)?;
    let path = dir.join(credentials_file_name(source, mount_point));
    let previous = fs::read(&path).ok();
    let mut file = fs::OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .mode(0o600)
        .open(&path)
        .map_err(failed)?;
    // An existing file keeps its old mode, so set it explicitly as well.
    file.set_permissions(fs::Permissions::from_mode(0o600))
        .map_err(failed)?;
    file.write_all(credentials_content(&username, &password).as_bytes())
        .map_err(failed)?;
    let credentials = format!("credentials={}", path.display());
    let options = if rest.is_empty() {
        credentials
    } else {
        format!("{credentials},{rest}")
    };
    Ok((options, Some(StoredCredentials { path, previous })))
}

pub fn auto_nfs_line(source: &str, mount_point: &str) -> String {
    format!("{mount_point}\t-fstype=nfs,vers=3,rw,resvport\t{source}\n")
}

pub fn remove_fstab_entry(content: &str, source: &str, mount_point: &str) -> String {
    remove_lines(content, |fields| {
        fields.len() >= 2 && fields[0] == source && fields[1] == mount_point
    })
}

pub fn remove_auto_nfs_entry(content: &str, mount_point: &str) -> String {
    remove_lines(content, |fields| fields[0] == mount_point)
}

/// Drops the entry lines whose whitespace-separated fields match, keeping
/// comments and blank lines.
fn remove_lines(content: &str, matches: impl Fn(&[&str]) -> bool) -> String {
    content
        .split('\n')
        .filter(|line| is_ignored(line) || !matches(&line.split_whitespace().collect::<Vec<_>>()))
        .collect::<Vec<_>>()
        .join("\n")
}

pub fn list() -> Result<Vec<Mount>> {
    if platform::is_macos() {
        let output = run("mount", &[]).map_err(|e| format!("failed to run mount command: {e}"))?;
        let auto_nfs = platform::auto_nfs_path()
            .and_then(|p| fs::read_to_string(p).ok())
            .unwrap_or_default();
        return Ok(parse_macos_mounts(&output, &parse_auto_nfs(&auto_nfs)));
    }
    let mounts =
        fs::read_to_string("/proc/mounts").map_err(|e| format!("failed to read /proc/mounts: {e}"))?;
    Ok(parse_proc_mounts(&mounts, &fstab_keys()))
}

fn fstab_keys() -> HashSet<(String, String)> {
    platform::fstab_path()
        .and_then(|p| fs::read_to_string(p).ok())
        .map(|c| parse_fstab_keys(&c))
        .unwrap_or_default()
}

pub fn get(mount_point: &str) -> Result<Mount> {
    list()?
        .into_iter()
        .find(|m| m.mount_point == mount_point)
        .ok_or_else(|| format!("mount '{mount_point}' not found"))
}

pub fn mount_cifs(source: &str, mount_point: &str, options: &str, persistent: bool) -> Result<()> {
    validate::cifs_source(source)?;
    validate::mount_point(mount_point)?;
    mount_share(source, mount_point, MountType::Cifs, options, persistent)
}

pub fn mount_nfs(source: &str, mount_point: &str, options: &str, persistent: bool) -> Result<()> {
    validate::nfs_source(source)?;
    validate::mount_point(mount_point)?;
    mount_share(source, mount_point, MountType::Nfs, options, persistent)
}

fn mount_share(
    source: &str,
    mount_point: &str,
    kind: MountType,
    options: &str,
    persistent: bool,
) -> Result<()> {
    if get(mount_point).is_ok() {
        return Err(format!("something is already mounted at '{mount_point}'"));
    }
    fs::create_dir_all(mount_point).map_err(|e| format!("failed to create mount point: {e}"))?;

    // mount_smbfs on macOS has no credentials-file option.
    let (protected, stored);
    let options = if kind == MountType::Cifs && !platform::is_macos() {
        (protected, stored) =
            protect_cifs_password(Path::new(CREDENTIALS_DIR), source, mount_point, options)?;
        protected.as_str()
    } else {
        stored = None;
        options
    };

    let mut args: Vec<&str> = Vec::new();
    let program = if platform::is_macos() {
        match kind {
            MountType::Cifs => "mount_smbfs",
            MountType::Nfs => "mount_nfs",
        }
    } else {
        args.extend(["-t", kind.as_str()]);
        "mount"
    };
    if !options.is_empty() {
        args.extend(["-o", options]);
    }
    args.extend([source, mount_point]);
    if let Err(e) = run(program, &args) {
        // A failed attempt must not leave an earlier mount's credentials replaced.
        if let Some(stored) = &stored {
            stored.undo();
        }
        return Err(format!("failed to mount: {e}"));
    }

    if persistent {
        if platform::is_macos() {
            return Err("persistent mounts are not fully supported on macOS. Mount succeeded but will not persist after reboot".into());
        }
        add_to_fstab(source, mount_point, kind.as_str(), options)
            .map_err(|e| format!("mounted successfully but failed to add to fstab: {e}"))?;
    }
    Ok(())
}

/// Unmounts a share, optionally dropping its fstab entry as well.
pub fn unmount(mount_point: &str, remove_persistent: bool) -> Result<()> {
    let mount = get(mount_point)?;
    run("umount", &[mount_point]).map_err(|e| format!("failed to unmount: {e}"))?;
    if remove_persistent && mount.persistent {
        remove_from_fstab(&mount.source, mount_point)
            .map_err(|e| format!("unmounted successfully but failed to remove from fstab: {e}"))?;
    }
    Ok(())
}

/// Makes an active mount persistent across reboots, or stops it being so.
pub fn set_persistent(mount_point: &str, persistent: bool) -> Result<()> {
    let mount = get(mount_point)?;
    if mount.persistent == persistent {
        return Ok(());
    }
    if platform::is_macos() {
        if mount.kind != MountType::Nfs {
            return Err("persistent CIFS/SMB mounts on macOS are not yet supported".into());
        }
        return if persistent {
            add_to_auto_nfs(&mount.source, &mount.mount_point)
        } else {
            remove_from_auto_nfs(&mount.mount_point)
        };
    }
    if persistent {
        add_to_fstab(
            &mount.source,
            &mount.mount_point,
            mount.kind.as_str(),
            &mount.options,
        )
    } else {
        remove_from_fstab(&mount.source, &mount.mount_point)
    }
}

/// Copies `path` to `path.backup` if it exists.
fn backup(path: &str) -> Result<()> {
    if Path::new(path).exists() {
        fs::copy(path, format!("{path}.backup")).map_err(|e| format!("failed to create backup: {e}"))?;
    }
    Ok(())
}

fn restore(path: &str) {
    let _ = fs::copy(format!("{path}.backup"), path);
}

fn append(path: &str, line: &str) -> std::io::Result<()> {
    let mut content = fs::read_to_string(path).unwrap_or_default();
    if !content.is_empty() && !content.ends_with('\n') {
        content.push('\n');
    }
    content.push_str(line);
    fs::write(path, content)
}

fn add_to_fstab(source: &str, mount_point: &str, fs_type: &str, options: &str) -> Result<()> {
    let path = platform::fstab_path().ok_or("fstab not supported on this platform")?;
    backup(path)?;
    if fstab_keys().contains(&(source.to_string(), mount_point.to_string())) {
        return Ok(());
    }
    append(path, &fstab_line(source, mount_point, fs_type, options))
        .map_err(|e| format!("failed to write to fstab: {e}"))
}

fn remove_from_fstab(source: &str, mount_point: &str) -> Result<()> {
    let Some(path) = platform::fstab_path() else {
        return Ok(());
    };
    backup(path)?;
    let content = fs::read_to_string(path).map_err(|e| format!("failed to read fstab: {e}"))?;
    fs::write(path, remove_fstab_entry(&content, source, mount_point)).map_err(|e| {
        restore(path);
        format!("failed to write fstab: {e}")
    })
}

fn add_to_auto_nfs(source: &str, mount_point: &str) -> Result<()> {
    let path = platform::auto_nfs_path().ok_or("auto_nfs not supported on this platform")?;
    backup(path)?;
    let existing = fs::read_to_string(path).unwrap_or_default();
    if parse_auto_nfs(&existing).contains_key(mount_point) {
        return Ok(());
    }
    append(path, &auto_nfs_line(source, mount_point))
        .map_err(|e| format!("failed to write to auto_nfs: {e}"))?;
    reload_automount(path)
}

fn remove_from_auto_nfs(mount_point: &str) -> Result<()> {
    let Some(path) = platform::auto_nfs_path() else {
        return Ok(());
    };
    backup(path)?;
    let content = fs::read_to_string(path).map_err(|e| format!("failed to read auto_nfs: {e}"))?;
    if let Err(e) = fs::write(path, remove_auto_nfs_entry(&content, mount_point)) {
        restore(path);
        return Err(format!("failed to write auto_nfs: {e}"));
    }
    reload_automount(path)
}

fn reload_automount(auto_nfs_path: &str) -> Result<()> {
    run("automount", &["-vc"]).map(|_| ()).map_err(|e| {
        restore(auto_nfs_path);
        format!("failed to reload automount: {e}")
    })
}

#[cfg(test)]
mod tests;
