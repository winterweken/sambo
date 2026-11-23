use super::*;
use std::collections::{HashMap, HashSet};

fn keys(pairs: &[(&str, &str)]) -> HashSet<(String, String)> {
    pairs
        .iter()
        .map(|(a, b)| (a.to_string(), b.to_string()))
        .collect()
}

const FSTAB: &str = "# /etc/fstab
UUID=abc / ext4 defaults 0 1

//server/share /mnt/share cifs credentials=/root/.smbcredentials,_netdev 0 0
192.168.1.100:/export /mnt/nfs nfs defaults,_netdev 0 0
/swapfile none swap sw 0 0
";

#[test]
fn fstab_keys_skip_comments_and_blank_lines() {
    let entries = parse_fstab_keys(FSTAB);
    assert!(entries.contains(&("//server/share".into(), "/mnt/share".into())));
    assert!(entries.contains(&("192.168.1.100:/export".into(), "/mnt/nfs".into())));
    assert!(!entries.contains(&("//server/share".into(), "/mnt/other".into())));
    assert_eq!(entries.len(), 4);
}

#[test]
fn proc_mounts_keeps_only_network_filesystems() {
    let content = "//server/share /mnt/share cifs rw,relatime 0 0
192.168.1.100:/export /mnt/nfs nfs rw,vers=3 0 0
server:/v4 /mnt/v4 nfs4 rw 0 0
/dev/sda1 / ext4 rw 0 0
tmpfs /tmp tmpfs rw 0 0
short line
";
    let mounts = parse_proc_mounts(content, &HashSet::new());
    let summary: Vec<(&str, &str, MountType)> = mounts
        .iter()
        .map(|m| (m.source.as_str(), m.mount_point.as_str(), m.kind))
        .collect();
    assert_eq!(
        summary,
        [
            ("//server/share", "/mnt/share", MountType::Cifs),
            ("192.168.1.100:/export", "/mnt/nfs", MountType::Nfs),
            ("server:/v4", "/mnt/v4", MountType::Nfs),
        ]
    );
    assert!(mounts.iter().all(|m| m.active));
    assert_eq!(mounts[0].options, "rw,relatime");
}

#[test]
fn proc_mounts_marks_fstab_entries_persistent() {
    let content = "//server/share /mnt/share cifs rw 0 0\n//server/share /mnt/other cifs rw 0 0\n";
    let mounts = parse_proc_mounts(content, &keys(&[("//server/share", "/mnt/share")]));
    assert!(mounts[0].persistent);
    assert!(!mounts[1].persistent);
}

#[test]
fn mount_type_names() {
    assert_eq!(MountType::Cifs.as_str(), "cifs");
    assert_eq!(MountType::Nfs.as_str(), "nfs");
}

#[test]
fn fstab_line_uses_given_options_and_adds_netdev() {
    assert_eq!(
        fstab_line("//server/share", "/mnt/share", "cifs", "user=test"),
        "//server/share /mnt/share cifs user=test,_netdev 0 0\n"
    );
    assert_eq!(
        fstab_line("s:/e", "/mnt/n", "nfs", "rw,hard,_netdev"),
        "s:/e /mnt/n nfs rw,hard,_netdev 0 0\n"
    );
}

#[test]
fn fstab_line_falls_back_to_default_options() {
    assert_eq!(
        fstab_line("//server/share", "/mnt/share", "cifs", ""),
        "//server/share /mnt/share cifs credentials=/root/.smbcredentials,uid=1000,gid=1000,_netdev 0 0\n"
    );
    assert_eq!(
        fstab_line("s:/e", "/mnt/n", "nfs", ""),
        "s:/e /mnt/n nfs defaults,_netdev 0 0\n"
    );
}

#[test]
fn removing_an_fstab_entry_needs_source_and_mount_point_to_match() {
    let out = remove_fstab_entry(FSTAB, "//server/share", "/mnt/share");
    assert!(!out.contains("/mnt/share"));
    assert!(out.contains("# /etc/fstab"));
    assert!(out.contains("192.168.1.100:/export /mnt/nfs"));
    assert_eq!(remove_fstab_entry(FSTAB, "//server/share", "/mnt/other"), FSTAB);
}

#[test]
fn auto_nfs_maps_mount_point_to_source() {
    let content =
        "# comment\n/Volumes/data\t-fstype=nfs,vers=3,rw,resvport\tnas:/export/data\n\n/mnt/x nas:/x\n";
    let entries = parse_auto_nfs(content);
    assert_eq!(
        entries.get("/Volumes/data").map(String::as_str),
        Some("nas:/export/data")
    );
    assert_eq!(entries.get("/mnt/x").map(String::as_str), Some("nas:/x"));
    assert_eq!(entries.len(), 2);
}

#[test]
fn auto_nfs_line_format() {
    assert_eq!(
        auto_nfs_line("nas:/export", "/Volumes/data"),
        "/Volumes/data\t-fstype=nfs,vers=3,rw,resvport\tnas:/export\n"
    );
}

#[test]
fn removing_an_auto_nfs_entry_matches_the_mount_point() {
    let content = "# keep\n/Volumes/a -opts nas:/a\n/Volumes/ab -opts nas:/ab\n";
    assert_eq!(
        remove_auto_nfs_entry(content, "/Volumes/a"),
        "# keep\n/Volumes/ab -opts nas:/ab\n"
    );
}

const MACOS_MOUNT: &str = "/dev/disk3s1s1 on / (apfs, sealed, local, read-only, journaled)
map auto_home on /System/Volumes/Data/home (autofs, automounted, nobrowse)
map auto_nfs on /Volumes/trigger (autofs, automounted, nobrowse)
//alice@nas/media on /Volumes/media (smbfs, nodev, nosuid, mounted by alice)
nas:/export/data on /Volumes/data (nfs, nodev, nosuid, automounted, nobrowse)
nas:/export/my files on /Volumes/my files (nfs)
";

#[test]
fn macos_mount_output_keeps_smb_and_nfs_and_skips_autofs() {
    let mounts = parse_macos_mounts(MACOS_MOUNT, &HashMap::new());
    let summary: Vec<(&str, &str, MountType)> = mounts
        .iter()
        .map(|m| (m.source.as_str(), m.mount_point.as_str(), m.kind))
        .collect();
    assert_eq!(
        summary,
        [
            ("//alice@nas/media", "/Volumes/media", MountType::Cifs),
            ("nas:/export/data", "/Volumes/data", MountType::Nfs),
            ("nas:/export/my files", "/Volumes/my files", MountType::Nfs),
        ]
    );
}

#[test]
fn macos_nfs_mounts_in_auto_nfs_are_persistent() {
    let auto: HashMap<String, String> =
        [("/Volumes/data".to_string(), "nas:/export/data".to_string())].into();
    let mounts = parse_macos_mounts(MACOS_MOUNT, &auto);
    assert!(!mounts[0].persistent);
    assert!(mounts[1].persistent);
    assert!(!mounts[2].persistent);
}

#[test]
fn credentials_are_split_out_of_cifs_options() {
    let (options, credentials) = split_credentials("vers=3.0,username=ann,password=p,w=1,uid=1000,gid=1000");
    assert_eq!(options, "vers=3.0,w=1,uid=1000,gid=1000");
    assert_eq!(credentials, Some(("ann".to_string(), "p".to_string())));
}

#[test]
fn options_without_a_password_are_left_alone() {
    let options = "username=ann,uid=1000";
    assert_eq!(split_credentials(options), (options.to_string(), None));
    let options = "credentials=/root/.smbcredentials,uid=1000";
    assert_eq!(split_credentials(options), (options.to_string(), None));
}

#[test]
fn a_password_without_a_username_is_still_kept_out_of_the_options() {
    let (options, credentials) = split_credentials("password=pw,uid=1");
    assert_eq!(options, "uid=1");
    assert_eq!(credentials, Some((String::new(), "pw".to_string())));
}

#[test]
fn credentials_files_are_named_after_the_share() {
    assert_eq!(
        credentials_file_name("//nas.local/My Media"),
        "nas.local_My_Media.cred"
    );
    assert_eq!(credentials_file_name("//10.0.0.5/../etc"), "10.0.0.5_.._etc.cred");
}

#[test]
fn credentials_file_content_is_what_mount_cifs_reads() {
    assert_eq!(credentials_content("ann", "pw"), "username=ann\npassword=pw\n");
    assert_eq!(credentials_content("", "pw"), "password=pw\n");
}
