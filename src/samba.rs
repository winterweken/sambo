//! Samba share management (smb.conf).

use std::io::ErrorKind;
use std::sync::Arc;

use crate::system::{CommandExecutor, FileSystem, Platform};
use crate::{avahi, validate, Result};

pub const GENERAL: &str = "general";
pub const TIME_MACHINE: &str = "timemachine";
pub const UNIFI_PROTECT: &str = "unifi-protect";
pub const MEDIA: &str = "media";

/// Share presets as `(type, display name)`, in the order the UI offers them.
pub const SHARE_TYPES: [(&str, &str); 4] = [
    (GENERAL, "General Purpose"),
    (TIME_MACHINE, "Time Machine"),
    (UNIFI_PROTECT, "Ubiquiti Protect"),
    (MEDIA, "Media Server"),
];

// Comment lines written into preset shares; parsing uses them to recognise the preset.
const UNIFI_MARKER: &str = "# Ubiquiti Protect optimized settings";
const MEDIA_MARKER: &str = "# Media server optimized settings";

const TIME_MACHINE_FRUIT: [&str; 11] = [
    "vfs objects = catia fruit streams_xattr",
    "fruit:metadata = stream",
    "fruit:model = MacSamba",
    "fruit:veto_appledouble = no",
    "fruit:posix_rename = yes",
    "fruit:zero_file_id = yes",
    "fruit:wipe_intentionally_left_blank_rfork = yes",
    "fruit:delete_empty_adfiles = yes",
    "fruit:nfs_aces = no",
    "fruit:aapl = yes",
    "fruit:time machine = yes",
];

/// Reliability settings Time Machine depends on.
const TIME_MACHINE_RELIABILITY: [&str; 4] = [
    "durable handles = yes",
    "kernel oplocks = no",
    "kernel share modes = no",
    "posix locking = no",
];

const UNIFI_SETTINGS: [&str; 11] = [
    UNIFI_MARKER,
    "create mask = 0660",
    "directory mask = 0770",
    "force create mode = 0660",
    "force directory mode = 0770",
    "inherit permissions = yes",
    "nt acl support = yes",
    "store dos attributes = yes",
    "map acl inherit = yes",
    "ea support = yes",
    "vfs objects = streams_xattr",
];

const MEDIA_SETTINGS: [&str; 6] = [
    MEDIA_MARKER,
    "strict locking = no",
    "min receivefile size = 16384",
    "use sendfile = yes",
    "aio read size = 16384",
    "aio write size = 16384",
];

/// The lines a non-Time-Machine preset adds to its share.
fn preset_settings(share_type: &str) -> &'static [&'static str] {
    match share_type {
        UNIFI_PROTECT => &UNIFI_SETTINGS,
        MEDIA => &MEDIA_SETTINGS,
        _ => &[],
    }
}

const BASIC_CONFIG: &str = "[global]
   workgroup = WORKGROUP
   server string = Samba Server
   security = user
   map to guest = Bad User
   dns proxy = no

";

pub trait AvahiManager: Send + Sync {
    fn add_time_machine_share(&self, name: &str, existing: &[String]) -> Result<()>;
    fn remove_time_machine_share(&self, name: &str, remaining: &[String]) -> Result<()>;
}

pub struct RealAvahiManager;

impl AvahiManager for RealAvahiManager {
    fn add_time_machine_share(&self, name: &str, existing: &[String]) -> Result<()> {
        avahi::update_time_machine_service(&avahi::with_share(name, existing))
    }
    fn remove_time_machine_share(&self, name: &str, remaining: &[String]) -> Result<()> {
        avahi::update_time_machine_service(&avahi::without_share(name, remaining))
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Share {
    pub name: String,
    pub path: String,
    pub comment: String,
    pub read_only: bool,
    pub browseable: bool,
    pub valid_users: Vec<String>,
    /// One of the preset constants; empty means general.
    pub share_type: String,
    /// Set for Time Machine shares; takes precedence over `share_type`.
    pub time_machine: bool,
    /// e.g. "500G", "1T"; empty or "0" means unlimited.
    pub time_machine_max_size: String,
}

impl Share {
    pub fn effective_type(&self) -> &str {
        if self.time_machine {
            TIME_MACHINE
        } else if self.share_type.is_empty() {
            GENERAL
        } else {
            &self.share_type
        }
    }
}

/// The fields of a share that `Manager::modify` should change.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ShareUpdate {
    pub comment: Option<String>,
    pub read_only: Option<bool>,
    pub browseable: Option<bool>,
    pub valid_users: Option<Vec<String>>,
    pub time_machine: Option<bool>,
}

fn section_name(line: &str) -> Option<&str> {
    let trimmed = line.trim();
    (trimmed.starts_with('[') && trimmed.ends_with(']')).then(|| trimmed.trim_matches(['[', ']']))
}

/// Samba compares parameter names without regard to case or spaces.
fn normalize_key(key: &str) -> String {
    key.chars()
        .filter(|c| !c.is_whitespace())
        .flat_map(char::to_lowercase)
        .collect()
}

/// The `(normalized key, value)` of a `key = value` line; None for comments,
/// section headers and blank lines.
fn setting(line: &str) -> Option<(String, &str)> {
    let line = line.trim();
    if line.starts_with('#') || line.starts_with(';') {
        return None;
    }
    let (key, value) = line.split_once('=')?;
    Some((normalize_key(key), value.trim()))
}

fn truthy(value: &str) -> bool {
    ["yes", "true", "1", "on"]
        .iter()
        .any(|word| value.eq_ignore_ascii_case(word))
}

/// Inverted spellings of `read only`, already normalized.
const WRITABLE_KEYS: [&str; 3] = ["writable", "writeable", "writeok"];

/// Parses the shares out of smb.conf content, skipping the special sections.
pub fn parse_shares(content: &str) -> Vec<Share> {
    let mut shares: Vec<Share> = Vec::new();
    let mut in_share = false;

    for line in content.lines().map(str::trim) {
        if let Some(name) = section_name(line) {
            in_share = !matches!(name, "global" | "homes" | "printers");
            if in_share {
                shares.push(Share {
                    name: name.to_string(),
                    browseable: true,
                    ..Default::default()
                });
            }
            continue;
        }
        let Some(share) = shares.last_mut().filter(|_| in_share) else {
            continue;
        };
        match line {
            UNIFI_MARKER => share.share_type = UNIFI_PROTECT.to_string(),
            MEDIA_MARKER => share.share_type = MEDIA.to_string(),
            _ => {}
        }
        let Some((key, value)) = setting(line) else {
            continue;
        };
        match key.as_str() {
            "path" => share.path = value.to_string(),
            "comment" => share.comment = value.to_string(),
            "readonly" => share.read_only = truthy(value),
            k if WRITABLE_KEYS.contains(&k) => share.read_only = !truthy(value),
            "browseable" | "browsable" => share.browseable = truthy(value),
            "validusers" => {
                share.valid_users = value
                    .split(|c: char| c == ',' || c.is_whitespace())
                    .filter(|user| !user.is_empty())
                    .map(str::to_string)
                    .collect();
            }
            "fruit:timemachine" => share.time_machine = truthy(value),
            "fruit:timemachinemaxsize" => share.time_machine_max_size = value.to_string(),
            _ => {}
        }
    }
    shares
}

/// Drops the `[name]` section, up to the next section header.
pub fn remove_share_section(content: &str, name: &str) -> String {
    let mut in_target = false;
    content
        .split('\n')
        .filter(|line| {
            if let Some(section) = section_name(line) {
                in_target = section == name;
            }
            !in_target
        })
        .collect::<Vec<_>>()
        .join("\n")
}

/// The smb.conf section for a share, including its preset's settings.
pub fn render_share(share: &Share) -> String {
    let yes_no = |b: bool| if b { "yes" } else { "no" };
    let mut lines = vec![
        String::new(),
        format!("[{}]", share.name),
        format!("   path = {}", share.path),
    ];
    if !share.comment.is_empty() {
        lines.push(format!("   comment = {}", share.comment));
    }
    lines.push(format!("   read only = {}", yes_no(share.read_only)));
    lines.push(format!("   browseable = {}", yes_no(share.browseable)));
    if !share.valid_users.is_empty() {
        lines.push(format!("   valid users = {}", share.valid_users.join(" ")));
    }

    let mut add = |settings: &[&str]| lines.extend(settings.iter().map(|s| format!("   {s}")));
    match share.effective_type() {
        TIME_MACHINE => {
            add(&TIME_MACHINE_FRUIT);
            let max_size = share.time_machine_max_size.as_str();
            if !max_size.is_empty() && max_size != "0" {
                add(&[&format!("fruit:time machine max size = {max_size}")]);
            }
            add(&TIME_MACHINE_RELIABILITY);
        }
        other => add(preset_settings(other)),
    }
    lines.join("\n") + "\n"
}

/// Sets `keys[0]` in a section's lines, or removes it when `value` is None.
/// `keys` lists every spelling of the setting; the first matching line is
/// reused and the others are dropped so that none of them can override it.
fn set_setting(section: &mut Vec<String>, keys: &[&str], value: Option<&str>) {
    let normalized: Vec<String> = keys.iter().map(|key| normalize_key(key)).collect();
    let matches = |line: &String| setting(line).is_some_and(|(key, _)| normalized.contains(&key));
    let first = section.iter().position(matches);
    let mut index = 0;
    section.retain(|line| {
        index += 1;
        Some(index - 1) == first || !matches(line)
    });
    match (first, value) {
        (Some(at), Some(value)) => section[at] = format!("   {} = {value}", keys[0]),
        (Some(at), None) => {
            section.remove(at);
        }
        (None, Some(value)) => section.push(format!("   {} = {value}", keys[0])),
        (None, None) => {}
    }
}

/// Applies the fields of `update` that differ from `current` to the share's
/// section of smb.conf. Every other line, including settings sambo does not
/// manage, is left exactly as it is.
pub fn update_share_section(content: &str, current: &Share, update: &ShareUpdate) -> String {
    let lines: Vec<&str> = content.split('\n').collect();
    let Some(header) = lines
        .iter()
        .position(|line| section_name(line) == Some(current.name.as_str()))
    else {
        return content.to_string();
    };
    let mut end = lines[header + 1..]
        .iter()
        .position(|line| section_name(line).is_some())
        .map_or(lines.len(), |offset| header + 1 + offset);
    // Blank lines before the next section stay between the sections.
    while end > header + 1 && lines[end - 1].trim().is_empty() {
        end -= 1;
    }
    let mut section: Vec<String> = lines[header + 1..end]
        .iter()
        .map(|line| line.to_string())
        .collect();

    fn changed<'a, T: PartialEq>(new: &'a Option<T>, old: &T) -> Option<&'a T> {
        new.as_ref().filter(|new| *new != old)
    }
    let yes_no = |b: bool| if b { "yes" } else { "no" };
    let non_empty = |value: &str| (!value.is_empty()).then(|| value.to_string());

    if let Some(comment) = changed(&update.comment, &current.comment) {
        set_setting(&mut section, &["comment"], non_empty(comment).as_deref());
    }
    if let Some(read_only) = changed(&update.read_only, &current.read_only) {
        let keys = ["read only", "writable", "writeable", "write ok"];
        set_setting(&mut section, &keys, Some(yes_no(*read_only)));
    }
    if let Some(browseable) = changed(&update.browseable, &current.browseable) {
        set_setting(
            &mut section,
            &["browseable", "browsable"],
            Some(yes_no(*browseable)),
        );
    }
    if let Some(users) = changed(&update.valid_users, &current.valid_users) {
        set_setting(
            &mut section,
            &["valid users"],
            non_empty(&users.join(" ")).as_deref(),
        );
    }
    if let Some(time_machine) = changed(&update.time_machine, &current.time_machine) {
        let own = TIME_MACHINE_FRUIT.iter().chain(&TIME_MACHINE_RELIABILITY);
        if *time_machine {
            for (key, value) in own.filter_map(|line| line.split_once(" = ")) {
                set_setting(&mut section, &[key], Some(value));
            }
        } else {
            for (key, _) in own.filter_map(|line| line.split_once(" = ")) {
                set_setting(&mut section, &[key], None);
            }
            set_setting(&mut section, &["fruit:time machine max size"], None);
            // Time Machine may have replaced a setting the share's preset needs.
            for (key, value) in
                (preset_settings(&current.share_type).iter()).filter_map(|line| line.split_once(" = "))
            {
                let key_normalized = normalize_key(key);
                if !section
                    .iter()
                    .any(|line| setting(line).is_some_and(|(k, _)| k == key_normalized))
                {
                    set_setting(&mut section, &[key], Some(value));
                }
            }
        }
    }

    let mut result: Vec<&str> = lines[..=header].to_vec();
    result.extend(section.iter().map(String::as_str));
    result.extend(&lines[end..]);
    result.join("\n")
}

/// Adds the global settings Time Machine needs to the `[global]` section.
/// Returns None when nothing has to change.
pub fn add_time_machine_globals(content: &str) -> Option<String> {
    let mut lines: Vec<&str> = content.split('\n').collect();
    let global = lines.iter().position(|l| l.trim() == "[global]")?;
    let next_section = lines[global + 1..]
        .iter()
        .position(|l| section_name(l).is_some())
        .map(|offset| global + 1 + offset);

    // Only [global] counts: a share may set the same parameters for itself.
    let in_global = &lines[global + 1..next_section.unwrap_or(lines.len())];
    let missing: Vec<&str> = [
        ("min protocol", "   min protocol = SMB2"),
        ("ea support", "   ea support = yes"),
        ("vfs objects", "   vfs objects = catia fruit streams_xattr"),
    ]
    .into_iter()
    .filter(|(key, _)| {
        let key = normalize_key(key);
        !in_global
            .iter()
            .any(|line| setting(line).is_some_and(|(k, _)| k == key))
    })
    .map(|(_, line)| line)
    .collect();
    if missing.is_empty() {
        return None;
    }

    // Insert just before the next section, or at the end of the file.
    let insert_at = next_section.unwrap_or_else(|| {
        if lines.last() == Some(&"") {
            lines.len() - 1
        } else {
            lines.len()
        }
    });
    lines.splice(insert_at..insert_at, missing);
    Some(lines.join("\n"))
}

/// Handles Samba configuration and operations.
pub struct Manager {
    fs: Arc<dyn FileSystem>,
    exec: Arc<dyn CommandExecutor>,
    platform: Arc<dyn Platform>,
    avahi: Arc<dyn AvahiManager>,
}

impl Manager {
    pub fn new(
        fs: Arc<dyn FileSystem>,
        exec: Arc<dyn CommandExecutor>,
        platform: Arc<dyn Platform>,
        avahi: Arc<dyn AvahiManager>,
    ) -> Self {
        Manager {
            fs,
            exec,
            platform,
            avahi,
        }
    }

    /// A manager wired to the real system.
    pub fn real() -> Self {
        use crate::system::{RealCommandExecutor, RealFileSystem, RealPlatform};
        Manager::new(
            Arc::new(RealFileSystem),
            Arc::new(RealCommandExecutor),
            Arc::new(RealPlatform),
            Arc::new(RealAvahiManager),
        )
    }

    fn conf_path(&self) -> String {
        self.platform.samba_config_path()
    }

    fn backup_path(&self) -> String {
        self.conf_path() + ".backup"
    }

    fn read_conf(&self) -> Result<String> {
        self.fs
            .read_to_string(&self.conf_path())
            .map_err(|e| format!("failed to read config: {e}"))
    }

    /// Verifies Samba is installed and creates a basic smb.conf if there is none.
    pub fn check_installed(&self) -> Result<()> {
        if !self.exec.has("smbd") {
            return Err(if self.platform.is_macos() {
                "Samba is not installed.\n\nPlease install Samba:\n  macOS: brew install samba".into()
            } else {
                "Samba is not installed.\n\nPlease install Samba:\n  Debian/Ubuntu: sudo apt-get install samba\n  RHEL/CentOS:   sudo yum install samba\n  Arch:          sudo pacman -S samba".into()
            });
        }
        let conf = self.conf_path();
        if !self.fs.exists(&conf) {
            self.fs
                .create_dir_all(&self.platform.samba_config_dir())
                .map_err(|e| format!("Samba is installed but cannot create config directory: {e}"))?;
            self.fs
                .write(&conf, BASIC_CONFIG)
                .map_err(|e| format!("Samba is installed but cannot create config file: {e}"))?;
        }
        Ok(())
    }

    pub fn list(&self) -> Result<Vec<Share>> {
        let conf = self.conf_path();
        match self.fs.read_to_string(&conf) {
            Ok(content) => Ok(parse_shares(&content)),
            Err(e) if e.kind() == ErrorKind::NotFound => Ok(Vec::new()),
            Err(e) => Err(format!("failed to open {conf}: {e}")),
        }
    }

    pub fn get(&self, name: &str) -> Result<Share> {
        self.list()?
            .into_iter()
            .find(|s| s.name == name)
            .ok_or_else(|| format!("share '{name}' not found"))
    }

    /// Adds a share and reloads Samba. Returns warnings worth showing the user.
    pub fn create(&self, share: &Share) -> Result<Vec<String>> {
        let mut warnings: Vec<String> = self.platform.service_warning("samba").into_iter().collect();

        validate::share_name(&share.name).map_err(|e| format!("invalid share name: {e}"))?;
        validate::path(&share.path).map_err(|e| format!("invalid path: {e}"))?;
        if self.get(&share.name).is_ok() {
            return Err(format!("share '{}' already exists", share.name));
        }
        if !self.fs.exists(&share.path) {
            return Err(format!("path '{}' does not exist", share.path));
        }

        let is_time_machine = share.effective_type() == TIME_MACHINE;
        self.fix_permissions(share)
            .map_err(|e| format!("failed to set share permissions: {e}"))?;
        if is_time_machine {
            self.ensure_time_machine_global_config()
                .map_err(|e| format!("failed to configure Time Machine global settings: {e}"))?;
        }

        self.backup_config()?;
        let content = self.read_conf()? + &render_share(share);
        self.fs
            .write(&self.conf_path(), &content)
            .map_err(|e| format!("failed to write config: {e}"))?;
        self.test_config_or_restore()?;
        self.reload()?;

        if is_time_machine {
            if let Err(e) = self
                .avahi
                .add_time_machine_share(&share.name, &self.time_machine_share_names())
            {
                // Avahi only affects discovery; the share itself works without it.
                warnings.push(format!(
                    "Failed to update Avahi service for Time Machine discovery: {e}"
                ));
            }
        }
        Ok(warnings)
    }

    pub fn remove(&self, name: &str) -> Result<Vec<String>> {
        let share = self.get(name)?;
        self.backup_config()?;
        let content = remove_share_section(&self.read_conf()?, name);
        if let Err(e) = self.fs.write(&self.conf_path(), &content) {
            self.restore_config();
            return Err(format!("failed to write config: {e}"));
        }
        self.test_config_or_restore()?;
        self.reload()?;

        let mut warnings = Vec::new();
        if share.time_machine {
            if let Err(e) = self
                .avahi
                .remove_time_machine_share(name, &self.time_machine_share_names())
            {
                warnings.push(format!("Failed to update Avahi service: {e}"));
            }
        }
        Ok(warnings)
    }

    /// Updates a share in place. The new config is validated in a temp file
    /// and then renamed over smb.conf, so a bad edit never goes live.
    pub fn modify(&self, name: &str, update: ShareUpdate) -> Result<Vec<String>> {
        let current = self.get(name)?;
        let was_time_machine = current.time_machine;
        let share = Share {
            valid_users: update
                .valid_users
                .clone()
                .unwrap_or_else(|| current.valid_users.clone()),
            time_machine: update.time_machine.unwrap_or(was_time_machine),
            ..current.clone()
        };

        // A share that becomes a Time Machine target needs what `create` sets up.
        if share.time_machine && !was_time_machine {
            self.fix_permissions(&share)
                .map_err(|e| format!("failed to set share permissions: {e}"))?;
            self.ensure_time_machine_global_config()
                .map_err(|e| format!("failed to configure Time Machine global settings: {e}"))?;
        }

        self.backup_config()?;
        let conf = self.conf_path();
        let content = update_share_section(&self.read_conf()?, &current, &update);

        let tmp = format!("{conf}.tmp");
        self.fs
            .write(&tmp, &content)
            .map_err(|e| format!("failed to write temp config: {e}"))?;
        if let Err(e) = self.exec.run("testparm", &["-s", &tmp]) {
            let _ = self.fs.remove_file(&tmp);
            return Err(format!("invalid configuration: {e}"));
        }
        if let Err(e) = self.fs.rename(&tmp, &conf) {
            let _ = self.fs.remove_file(&tmp);
            self.restore_config();
            return Err(format!("failed to apply config: {e}"));
        }
        self.reload()?;

        let mut warnings = Vec::new();
        if was_time_machine != share.time_machine {
            let names = self.time_machine_share_names();
            let result = if share.time_machine {
                self.avahi.add_time_machine_share(name, &names)
            } else {
                self.avahi.remove_time_machine_share(name, &names)
            };
            if let Err(e) = result {
                warnings.push(format!("Failed to update Avahi service: {e}"));
            }
        }
        Ok(warnings)
    }

    /// Sets the ownership and mode the share's preset needs on its directory.
    pub fn fix_permissions(&self, share: &Share) -> Result<()> {
        let kind = share.effective_type();
        if kind != TIME_MACHINE && kind != UNIFI_PROTECT {
            return Ok(());
        }
        let owner = share.valid_users.first();
        if kind == TIME_MACHINE && owner.is_none() {
            return Err("Time Machine share has no valid users to assign ownership".into());
        }
        if !self.fs.exists(&share.path) {
            return Err(format!("path '{}' does not exist", share.path));
        }
        if let Some(owner) = owner {
            let uid = self
                .platform
                .system_uid(owner)
                .map_err(|e| format!("failed to resolve UID for user '{owner}': {e}"))?;
            self.fs
                .chown(&share.path, uid)
                .map_err(|e| format!("failed to chown directory: {e}"))?;
        }
        // Time Machine wants the directory private to its owner; Protect needs group write.
        let mode = if kind == TIME_MACHINE { 0o700 } else { 0o770 };
        self.fs
            .chmod(&share.path, mode)
            .map_err(|e| format!("failed to chmod directory: {e}"))
    }

    fn time_machine_share_names(&self) -> Vec<String> {
        self.list()
            .unwrap_or_default()
            .into_iter()
            .filter(|s| s.time_machine)
            .map(|s| s.name)
            .collect()
    }

    fn ensure_time_machine_global_config(&self) -> Result<()> {
        let Some(updated) = add_time_machine_globals(&self.read_conf()?) else {
            return Ok(());
        };
        self.backup_config()?;
        if let Err(e) = self.fs.write(&self.conf_path(), &updated) {
            self.restore_config();
            return Err(format!("failed to write config: {e}"));
        }
        self.test_config_or_restore()
    }

    fn backup_config(&self) -> Result<()> {
        let content = self.read_conf()?;
        self.fs
            .write(&self.backup_path(), &content)
            .map_err(|e| format!("failed to create backup: {e}"))
    }

    fn restore_config(&self) {
        if let Ok(backup) = self.fs.read_to_string(&self.backup_path()) {
            let _ = self.fs.write(&self.conf_path(), &backup);
        }
    }

    fn test_config_or_restore(&self) -> Result<()> {
        self.exec
            .run("testparm", &["-s", &self.conf_path()])
            .map_err(|e| {
                self.restore_config();
                format!("invalid configuration: configuration test failed: {e}")
            })
    }

    fn reload(&self) -> Result<()> {
        type Command = (&'static str, &'static [&'static str]);
        let (first, fallback): (Command, Command) = if self.platform.is_macos() {
            (
                ("pkill", &["-HUP", "smbd"]),
                ("brew", &["services", "restart", "samba"]),
            )
        } else {
            (
                ("systemctl", &["reload", "smbd"]),
                ("service", &["smbd", "reload"]),
            )
        };
        if self.exec.run(first.0, first.1).is_ok() {
            return Ok(());
        }
        self.exec
            .run(fallback.0, fallback.1)
            .map_err(|e| format!("failed to reload samba: {e}"))
    }
}

#[cfg(test)]
mod tests;
