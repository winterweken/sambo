//! The forms and confirmations of each tab, and the work they turn into.

use crate::cli::util::parse_csv;
use crate::mount::Mount;
use crate::nfs::{self, Export, OptionChanges};
use crate::samba::{self, Share, ShareUpdate};
use crate::validate;
use crate::Result;

use super::{Action, Component, Confirm, Purpose, Request, Tab};
use crate::tui::form::{Field, Form};

/// A request for the worker, with what to tell the user about it.
pub struct Job {
    pub request: Request,
    /// Shown while it runs.
    pub label: String,
    /// Shown when it succeeded.
    pub success: String,
    /// The tabs whose data it changes.
    pub reload: Vec<Tab>,
}

fn job(request: Request, label: String, success: String, tab: Tab) -> Job {
    Job {
        request,
        label,
        success,
        reload: vec![tab],
    }
}

// ── forms ───────────────────────────────────────────────────────────────

pub fn create_share() -> Form {
    Form::new(
        "Create Samba Share",
        "Create",
        vec![
            Field::text("name", "Share name", ""),
            Field::text("path", "Path", ""),
            Field::text("comment", "Comment", ""),
            Field::text("users", "Valid users", ""),
            Field::choice("type", "Share type", &samba::SHARE_TYPES, samba::GENERAL),
        ],
    )
}

pub fn modify_share(share: &Share) -> Form {
    Form::new(
        "Edit Samba Share",
        "Save",
        vec![
            Field::read_only("name", "Share name", &share.name),
            Field::read_only("path", "Path", &share.path),
            Field::text("comment", "Comment", &share.comment),
            Field::text("users", "Valid users", &share.valid_users.join(", ")),
            Field::checkbox("read_only", "Read only", share.read_only),
            Field::checkbox("browseable", "Browseable", share.browseable),
            Field::checkbox("time_machine", "Time Machine", share.time_machine),
        ],
    )
}

fn export_flags(options: &str) -> [Field; 3] {
    let (read_only, async_mode, no_root_squash) = nfs::option_flags(options);
    [
        Field::checkbox("read_only", "Read only", read_only),
        Field::checkbox("no_root_squash", "No root squash", no_root_squash),
        Field::checkbox("async", "Async mode", async_mode),
    ]
}

pub fn create_export() -> Form {
    let mut fields = vec![
        Field::text("path", "Export path", ""),
        Field::text("clients", "Clients", "*"),
    ];
    fields.extend(export_flags(""));
    Form::new("Create NFS Export", "Create", fields)
}

pub fn modify_export(export: &Export) -> Form {
    let mut fields = vec![
        Field::read_only("path", "Export path", &export.path),
        Field::text("clients", "Clients", &export.clients),
    ];
    fields.extend(export_flags(&export.options));
    Form::new("Edit NFS Export", "Save", fields)
}

pub fn add_user() -> Form {
    Form::new(
        "Add Samba User",
        "Add",
        vec![
            Field::text("username", "Username", ""),
            Field::password("password", "Password"),
        ],
    )
}

pub fn set_password(username: &str) -> Form {
    Form::new(
        "Change Password",
        "Change",
        vec![
            Field::read_only("username", "Username", username),
            Field::password("password", "New password"),
        ],
    )
}

pub fn mount_cifs() -> Form {
    Form::new(
        "Mount CIFS/SMB Share",
        "Mount",
        vec![
            Field::text("source", "Source", ""),
            Field::text("mount_point", "Mount point", ""),
            Field::text("username", "Username", ""),
            Field::password("password", "Password"),
            Field::checkbox("persistent", "Persistent", false),
        ],
    )
}

pub fn mount_nfs(source: &str) -> Form {
    Form::new(
        "Mount NFS Export",
        "Mount",
        vec![
            Field::text("source", "Source", source),
            Field::text("mount_point", "Mount point", ""),
            Field::checkbox("persistent", "Persistent", false),
        ],
    )
}

/// A one-line reminder of the expected format, shown under the form.
pub fn hint(purpose: &Purpose) -> &'static str {
    match purpose {
        Purpose::CreateShare | Purpose::ModifyShare(_) => {
            "Valid users: comma-separated; empty allows everyone"
        }
        Purpose::CreateExport | Purpose::ModifyExport(_) => {
            "Clients: * or hosts/networks separated by spaces"
        }
        Purpose::AddUser => "Also creates the system account if it does not exist",
        Purpose::SetPassword(_) => "",
        Purpose::MountCifs => "Source: //server/share; without a username /root/.smbcredentials is used",
        Purpose::MountNfs => "Source: server:/path",
    }
}

// ── submitting ──────────────────────────────────────────────────────────

/// Checks a submitted form and turns it into the work to do.
pub fn submission(purpose: &Purpose, form: &Form) -> Result<Job> {
    match purpose {
        Purpose::CreateShare => {
            let (name, path) = (form.text("name"), form.text("path"));
            if name.is_empty() || path.is_empty() {
                return Err("Share name and path are required".into());
            }
            validate::share_name(&name)?;
            validate::path(&path)?;
            let share_type = form.choice("type");
            let share = Share {
                name: name.clone(),
                path,
                comment: form.text("comment"),
                read_only: false,
                browseable: true,
                valid_users: parse_csv(&form.text("users")),
                time_machine: share_type == samba::TIME_MACHINE,
                share_type,
                time_machine_max_size: "0".into(),
            };
            Ok(job(
                Request::CreateShare(share),
                format!("Creating share '{name}'…"),
                format!("Share '{name}' created; Samba reloaded"),
                Tab::Shares,
            ))
        }
        Purpose::ModifyShare(name) => {
            let update = ShareUpdate {
                comment: Some(form.text("comment")),
                read_only: Some(form.checked("read_only")),
                browseable: Some(form.checked("browseable")),
                valid_users: Some(parse_csv(&form.text("users"))),
                time_machine: Some(form.checked("time_machine")),
            };
            Ok(job(
                Request::ModifyShare {
                    name: name.clone(),
                    update,
                },
                format!("Saving share '{name}'…"),
                format!("Share '{name}' updated; Samba reloaded"),
                Tab::Shares,
            ))
        }
        Purpose::CreateExport => {
            let (path, clients, options) = export_values(form)?;
            validate::path(&path)?;
            let export = Export {
                path: path.clone(),
                clients,
                options,
            };
            Ok(job(
                Request::CreateExport(export),
                format!("Creating export {path}…"),
                format!("Export {path} created; NFS exports reloaded"),
                Tab::Exports,
            ))
        }
        Purpose::ModifyExport(export) => {
            let (_, clients, _) = export_values(form)?;
            // Only what the user toggled is changed, so the export's other
            // options and each client's own options survive the edit.
            let (read_only, async_mode, no_root_squash) = nfs::option_flags(&export.options);
            let toggled = |key: &str, before: bool| Some(form.checked(key)).filter(|now| *now != before);
            let changes = OptionChanges {
                read_only: toggled("read_only", read_only),
                async_mode: toggled("async", async_mode),
                no_root_squash: toggled("no_root_squash", no_root_squash),
            };
            let path = &export.path;
            Ok(job(
                Request::ModifyExport {
                    path: path.clone(),
                    clients,
                    changes,
                },
                format!("Saving export {path}…"),
                format!("Export {path} updated; NFS exports reloaded"),
                Tab::Exports,
            ))
        }
        Purpose::AddUser => {
            let (username, password) = (form.text("username"), form.text("password"));
            validate::username(&username)?;
            validate::password(&password)?;
            Ok(job(
                Request::AddUser {
                    username: username.clone(),
                    password,
                },
                format!("Adding user '{username}'…"),
                format!("User '{username}' added"),
                Tab::Users,
            ))
        }
        Purpose::SetPassword(username) => {
            let password = form.text("password");
            validate::password(&password)?;
            Ok(job(
                Request::SetPassword {
                    username: username.clone(),
                    password,
                },
                format!("Changing password of '{username}'…"),
                format!("Password of '{username}' changed"),
                Tab::Users,
            ))
        }
        Purpose::MountCifs => {
            let (source, mount_point) = (form.text("source"), form.text("mount_point"));
            validate::cifs_source(&source)?;
            validate::mount_point(&mount_point)?;
            let persistent = form.checked("persistent");
            Ok(job(
                Request::MountCifs {
                    source: source.clone(),
                    mount_point: mount_point.clone(),
                    username: form.text("username"),
                    password: form.text("password"),
                    persistent,
                },
                format!("Mounting {source}…"),
                format!("{source} mounted at {mount_point}"),
                Tab::Mounts,
            ))
        }
        Purpose::MountNfs => {
            let (source, mount_point) = (form.text("source"), form.text("mount_point"));
            validate::nfs_source(&source)?;
            validate::mount_point(&mount_point)?;
            let persistent = form.checked("persistent");
            Ok(job(
                Request::MountNfs {
                    source: source.clone(),
                    mount_point: mount_point.clone(),
                    persistent,
                },
                format!("Mounting {source}…"),
                format!("{source} mounted at {mount_point}"),
                Tab::Mounts,
            ))
        }
    }
}

/// The `(path, clients, options)` of an export form.
fn export_values(form: &Form) -> Result<(String, String, String)> {
    let path = form.text("path");
    if path.is_empty() {
        return Err("Export path is required".into());
    }
    let clients = match form.text("clients") {
        clients if clients.is_empty() => "*".to_string(),
        clients => clients,
    };
    validate::nfs_clients(&clients)?;
    let options = nfs::build_options(
        form.checked("read_only"),
        form.checked("async"),
        form.checked("no_root_squash"),
    );
    Ok((path, clients, options))
}

// ── confirmations ───────────────────────────────────────────────────────

fn confirm(title: &str, message: String, action: Action) -> Confirm {
    Confirm {
        title: title.to_string(),
        message,
        toggle: None,
        action,
    }
}

pub fn remove_share(share: &Share) -> Confirm {
    confirm(
        "Remove Share",
        format!(
            "Remove the share '{}'? The files in {} are kept.",
            share.name, share.path
        ),
        Action::RemoveShare(share.name.clone()),
    )
}

pub fn remove_export(export: &Export) -> Confirm {
    confirm(
        "Remove Export",
        format!("Stop exporting {}? The files are kept.", export.path),
        Action::RemoveExport(export.path.clone()),
    )
}

pub fn remove_user(username: &str) -> Confirm {
    confirm(
        "Remove User",
        format!("Remove the Samba user '{username}'? The system account is kept."),
        Action::RemoveUser(username.to_string()),
    )
}

pub fn unmount(mount: &Mount) -> Confirm {
    Confirm {
        toggle: mount
            .persistent
            .then_some(("Also remove the persistent entry", true)),
        ..confirm(
            "Unmount",
            format!("Unmount {} from {}?", mount.source, mount.mount_point),
            Action::Unmount(mount.mount_point.clone()),
        )
    }
}

pub fn install(component: &Component, package_manager: &str) -> Confirm {
    confirm(
        "Install",
        format!("Install {} using {package_manager}?", component.name),
        Action::Install(component.key),
    )
}

/// The work a confirmed dialog stands for.
pub fn confirmed(confirm: &Confirm) -> Job {
    match &confirm.action {
        Action::RemoveShare(name) => job(
            Request::RemoveShare(name.clone()),
            format!("Removing share '{name}'…"),
            format!("Share '{name}' removed"),
            Tab::Shares,
        ),
        Action::RemoveExport(path) => job(
            Request::RemoveExport(path.clone()),
            format!("Removing export {path}…"),
            format!("Export {path} removed"),
            Tab::Exports,
        ),
        Action::RemoveUser(username) => job(
            Request::RemoveUser(username.clone()),
            format!("Removing user '{username}'…"),
            format!("User '{username}' removed"),
            Tab::Users,
        ),
        Action::Unmount(mount_point) => job(
            Request::Unmount {
                mount_point: mount_point.clone(),
                remove_persistent: confirm.toggle.is_some_and(|(_, on)| on),
            },
            format!("Unmounting {mount_point}…"),
            format!("{mount_point} unmounted"),
            Tab::Mounts,
        ),
        // A new package can change what every tab is able to show.
        Action::Install(key) => Job {
            request: Request::Install(key),
            label: format!("Installing {key}… this can take a few minutes"),
            success: format!("Successfully installed {key}"),
            reload: Tab::ALL.to_vec(),
        },
    }
}

pub fn set_persistent(mount: &Mount) -> Job {
    let persistent = !mount.persistent;
    let mount_point = &mount.mount_point;
    job(
        Request::SetPersistent {
            mount_point: mount_point.clone(),
            persistent,
        },
        format!("Updating {mount_point}…"),
        if persistent {
            format!("{mount_point} will be mounted at boot")
        } else {
            format!("{mount_point} will no longer be mounted at boot")
        },
        Tab::Mounts,
    )
}
