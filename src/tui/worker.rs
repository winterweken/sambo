//! Carries out the app's requests. This is the only part of the UI that
//! touches the system, and it always runs off the UI thread.

use std::time::Duration;

use super::app::{Component, Loaded, Request, Response, ScanResult, SystemInfo, Tab};
use crate::mount::{self, discover};
use crate::samba::Manager;
use crate::system::packages as pkg;
use crate::{nfs, service, user, Result};

const SCAN_TIMEOUT: Duration = Duration::from_millis(300);

type Probe = fn() -> bool;

/// The installable components: key, name, and how to tell whether it is present.
const COMPONENTS: [(&str, &str, Probe); 5] = [
    ("samba", "Samba Server", pkg::is_samba_installed),
    ("nfs", "NFS Server", pkg::is_nfs_installed),
    ("cifs", "CIFS Client", pkg::is_cifs_installed),
    ("nfs-client", "NFS Client", pkg::is_nfs_client_installed),
    ("avahi", "Avahi/Bonjour", pkg::is_avahi_installed),
];

pub fn execute(manager: &Manager, request: Request) -> Response {
    match request {
        Request::Load(tab) => Response::Loaded(tab, load(manager, tab)),
        Request::Scan => Response::Scanned(scan()),
        change => Response::Done(apply(manager, change)),
    }
}

fn load(manager: &Manager, tab: Tab) -> Result<Loaded> {
    Ok(match tab {
        Tab::Shares => Loaded::Shares(manager.list()?),
        Tab::Exports => Loaded::Exports(nfs::list()?),
        Tab::Mounts => Loaded::Mounts(mount::list()?),
        Tab::Users => Loaded::Users(user::list()?),
        Tab::System => Loaded::System(system_info()),
    })
}

fn system_info() -> SystemInfo {
    let package_manager = match pkg::detect_package_manager() {
        pkg::PackageManager::Unknown => None,
        known => Some(known.name()),
    };
    SystemInfo {
        package_manager,
        components: COMPONENTS
            .iter()
            .map(|&(key, name, installed)| Component {
                key,
                name,
                installed: installed(),
            })
            .collect(),
        services: vec![("Samba", service::check_samba()), ("NFS", service::check_nfs())],
    }
}

fn scan() -> Result<ScanResult> {
    let subnet = discover::local_subnet().map_err(|e| format!("Failed to detect the local subnet: {e}"))?;
    let servers =
        discover::discover_nfs_servers(&subnet, SCAN_TIMEOUT).map_err(|e| format!("Scan failed: {e}"))?;
    Ok(ScanResult { subnet, servers })
}

/// Applies a change and returns the warnings it produced.
fn apply(manager: &Manager, request: Request) -> Result<Vec<String>> {
    let done = |result: Result<()>| result.map(|()| Vec::new());
    match request {
        Request::CreateShare(share) => {
            manager.check_installed().map_err(not_installed)?;
            manager.create(&share)
        }
        Request::ModifyShare { name, update } => manager.modify(&name, update),
        Request::RemoveShare(name) => manager.remove(&name),
        Request::CreateExport(export) => {
            nfs::check_installed().map_err(not_installed)?;
            nfs::create(&export)
        }
        Request::ModifyExport {
            path,
            clients,
            options,
        } => nfs::modify(&path, Some(clients), Some(options)),
        Request::RemoveExport(path) => done(nfs::remove(&path)),
        Request::AddUser { username, password } => done(user::add(&username, &password, true)),
        Request::SetPassword { username, password } => done(user::set_password(&username, &password)),
        Request::RemoveUser(username) => done(user::remove(&username, false)),
        Request::MountCifs {
            source,
            mount_point,
            options,
            persistent,
        } => done(mount::mount_cifs(&source, &mount_point, &options, persistent)),
        Request::MountNfs {
            source,
            mount_point,
            persistent,
        } => done(mount::mount_nfs(&source, &mount_point, "defaults", persistent)),
        Request::Unmount {
            mount_point,
            remove_persistent,
        } => done(mount::unmount(&mount_point, remove_persistent)),
        Request::SetPersistent {
            mount_point,
            persistent,
        } => done(mount::set_persistent(&mount_point, persistent)),
        Request::Install(key) => done(install(key).map_err(|e| format!("Failed to install {key}: {e}"))),
        Request::Load(_) | Request::Scan => Ok(Vec::new()),
    }
}

/// The backend's message includes shell instructions meant for the CLI; here
/// the System tab is the way to install.
fn not_installed(error: String) -> String {
    match error.split_once("\n\n") {
        Some((summary, _)) => format!("{summary} Install it from the System tab."),
        None => error,
    }
}

fn install(key: &str) -> Result<()> {
    let manager = pkg::detect_package_manager();
    let packages = match key {
        "samba" => pkg::samba_packages(manager),
        "nfs" => pkg::nfs_packages(manager),
        "cifs" => pkg::cifs_packages(manager),
        "nfs-client" => pkg::nfs_client_packages(manager),
        "avahi" => pkg::avahi_packages(manager),
        other => return Err(format!("unknown component '{other}'")),
    };
    pkg::install_packages(&packages)?;
    // The service name is only known once the package is there.
    let service = match key {
        "samba" => Some(pkg::samba_service_name()),
        "nfs" => Some(pkg::nfs_service_name()),
        "avahi" => pkg::avahi_service_name(),
        _ => None,
    };
    service.map_or(Ok(()), pkg::enable_service)
}
