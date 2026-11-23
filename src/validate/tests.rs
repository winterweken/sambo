use super::*;

#[test]
fn share_name_accepts_letters_digits_underscore_hyphen() {
    assert!(share_name("docs_2-b").is_ok());
}

#[test]
fn share_name_rejects_bad_names() {
    assert!(share_name("").is_err());
    assert!(share_name("my share").is_err());
    assert!(share_name(&"a".repeat(81)).is_err());
    assert_eq!(
        share_name("Global").unwrap_err(),
        "share name is reserved by Samba"
    );
    assert!(share_name("homes").is_err());
}

#[test]
fn path_must_be_absolute_and_not_critical() {
    assert!(path("/mnt/data").is_ok());
    assert!(path("").is_err());
    assert!(path("relative/path").is_err());
    assert!(path("./local").is_err());
    assert_eq!(
        path("/etc").unwrap_err(),
        "cannot share critical system directory: /etc"
    );
}

#[test]
fn path_is_normalised_before_the_critical_check() {
    assert!(path("/etc/").is_err());
    assert!(path("/mnt/../etc").is_err());
    assert!(path("//").is_err());
    assert!(path("/etc/samba").is_ok());
}

#[test]
fn mount_point_rules() {
    assert!(mount_point("/mnt/share").is_ok());
    assert!(mount_point("/home/user/mount").is_ok());
    assert!(mount_point("").is_err());
    assert!(mount_point("relative/path").is_err());
    assert!(mount_point("/usr").is_err());
}

#[test]
fn cifs_source_needs_server_and_share() {
    assert!(cifs_source("//server/share").is_ok());
    assert!(cifs_source("//192.168.1.100/share").is_ok());
    assert!(cifs_source("").is_err());
    assert!(cifs_source("/local/path").is_err());
    assert!(cifs_source("//server").is_err());
    assert!(cifs_source("//server/").is_err());
}

#[test]
fn nfs_source_needs_server_and_absolute_path() {
    assert!(nfs_source("server:/export").is_ok());
    assert!(nfs_source("192.168.1.100:/path/to/export").is_ok());
    assert!(nfs_source("").is_err());
    assert!(nfs_source("server").is_err());
    assert!(nfs_source(":/export").is_err());
    assert!(nfs_source("server:relative").is_err());
}

#[test]
fn username_rules() {
    for ok in ["testuser", "user1", "_user", "user-name", "root", "Alice"] {
        assert!(username(ok).is_ok(), "{ok}");
    }
    for bad in ["", "user name", "user\ttab", "1user", "-user"] {
        assert!(username(bad).is_err(), "{bad:?}");
    }
    assert!(username(&"a".repeat(33)).is_err());
}

#[test]
fn password_needs_four_characters() {
    assert!(password("short").is_ok());
    assert!(password("abcd").is_ok());
    assert!(password("abc").is_err());
    assert!(password("").is_err());
}

#[test]
fn nfs_clients_rules() {
    for ok in [
        "*",
        "192.168.1.0/24",
        "192.168.1.100",
        "client.example.com",
        "*.example.com",
        "@netgroup",
    ] {
        assert!(nfs_clients(ok).is_ok(), "{ok}");
    }
    assert!(nfs_clients("").is_err());
    assert!(nfs_clients("a;b").is_err());
    assert!(nfs_clients("a(b)").is_err());
}

#[test]
fn nfs_clients_reject_shell_and_line_break_characters() {
    for bad in ["a`id`", "a$HOME", "a&b", "a<b", "a>b", "a\n/etc *(rw)", "a\rb"] {
        assert!(nfs_clients(bad).is_err(), "{bad:?}");
    }
}
