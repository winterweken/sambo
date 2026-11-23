use super::*;

const ALL: [PackageManager; 7] = [Apt, Yum, Dnf, Pacman, Zypper, Apk, Homebrew];

#[test]
fn samba_packages_per_manager() {
    assert_eq!(samba_packages(Apt), ["samba"]);
    assert_eq!(samba_packages(Yum), ["samba", "samba-common-tools"]);
    assert_eq!(samba_packages(Dnf), ["samba", "samba-common-tools"]);
    assert_eq!(samba_packages(Homebrew), ["samba"]);
}

#[test]
fn nfs_server_packages() {
    assert_eq!(nfs_packages(Apt), ["nfs-kernel-server"]);
    assert_eq!(nfs_packages(Dnf), ["nfs-utils"]);
    assert_eq!(nfs_packages(Zypper), ["nfs-kernel-server"]);
    assert!(nfs_packages(Homebrew).is_empty());
}

#[test]
fn client_packages() {
    assert_eq!(cifs_packages(Apt), ["cifs-utils"]);
    assert!(cifs_packages(Homebrew).is_empty());
    assert_eq!(nfs_client_packages(Apt), ["nfs-common"]);
    assert_eq!(nfs_client_packages(Zypper), ["nfs-client"]);
    assert_eq!(nfs_client_packages(Pacman), ["nfs-utils"]);
}

#[test]
fn avahi_packages_per_manager() {
    assert_eq!(avahi_packages(Apt), ["avahi-daemon"]);
    assert_eq!(avahi_packages(Apk), ["avahi", "dbus"]);
    assert!(avahi_packages(Homebrew).is_empty());
}

#[test]
fn every_linux_manager_has_packages_for_every_component() {
    for pm in ALL.into_iter().filter(|pm| *pm != Homebrew) {
        for packages in [
            samba_packages(pm),
            nfs_packages(pm),
            cifs_packages(pm),
            nfs_client_packages(pm),
        ] {
            assert!(!packages.is_empty(), "{pm:?}");
        }
    }
}

#[test]
fn install_command_per_manager() {
    let cmd = |pm| install_command(pm, &["pkg1", "pkg2"]).unwrap().join(" ");
    assert_eq!(cmd(Apt), "apt-get install -y pkg1 pkg2");
    assert_eq!(cmd(Dnf), "dnf install -y pkg1 pkg2");
    assert_eq!(cmd(Yum), "yum install -y pkg1 pkg2");
    assert_eq!(cmd(Pacman), "pacman -S --noconfirm pkg1 pkg2");
    assert_eq!(cmd(Zypper), "zypper install -y pkg1 pkg2");
    assert_eq!(cmd(Apk), "apk add --no-cache pkg1 pkg2");
    assert_eq!(cmd(Homebrew), "brew install pkg1 pkg2");
}

#[test]
fn install_command_fails_for_unknown_manager() {
    assert!(install_command(Unknown, &["pkg"]).is_err());
}

#[test]
fn manager_names() {
    assert_eq!(Apt.name(), "apt");
    assert_eq!(Homebrew.name(), "homebrew");
    assert_eq!(Unknown.name(), "unknown");
}

#[test]
fn samba_service_name_from_unit_list() {
    assert_eq!(
        samba_service_from_units("smbd.service enabled\nnmbd.service enabled"),
        "smbd"
    );
    assert_eq!(samba_service_from_units("smb.service enabled"), "smb");
    assert_eq!(samba_service_from_units("other.service enabled"), "smbd");
}

#[test]
fn nfs_service_name_from_unit_list() {
    assert_eq!(
        nfs_service_from_units("nfs-kernel-server.service enabled"),
        "nfs-kernel-server"
    );
    assert_eq!(nfs_service_from_units("nfs-server.service enabled"), "nfs-server");
    assert_eq!(nfs_service_from_units("other.service enabled"), "nfs-server");
}
