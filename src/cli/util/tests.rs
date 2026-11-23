use super::*;

#[test]
fn csv_is_split_and_trimmed_and_empty_items_dropped() {
    assert_eq!(parse_csv("alice, bob ,,carol"), ["alice", "bob", "carol"]);
    assert!(parse_csv("").is_empty());
    assert!(parse_csv(" , ").is_empty());
}

#[test]
fn cifs_options_use_the_credentials_file_without_a_username() {
    assert_eq!(
        cifs_options("", "", "", None, None),
        "credentials=/root/.smbcredentials,uid=1000,gid=1000"
    );
}

#[test]
fn cifs_options_carry_username_and_password() {
    assert_eq!(
        cifs_options("", "alice", "s3cret", None, None),
        "username=alice,password=s3cret,uid=1000,gid=1000"
    );
    assert_eq!(
        cifs_options("", "alice", "", None, None),
        "username=alice,uid=1000,gid=1000"
    );
}

#[test]
fn cifs_options_prefer_the_sudo_users_ids() {
    assert_eq!(
        cifs_options("", "alice", "", Some("501"), Some("20")),
        "username=alice,uid=501,gid=20"
    );
}

#[test]
fn cifs_options_keep_explicit_uid_and_gid() {
    assert_eq!(
        cifs_options("vers=3.0,uid=7", "alice", "", Some("501"), Some("20")),
        "vers=3.0,uid=7,username=alice,gid=20"
    );
    assert_eq!(
        cifs_options("uid=7,gid=8", "alice", "", None, None),
        "uid=7,gid=8,username=alice"
    );
}

#[test]
fn credentials_that_look_like_uid_or_gid_do_not_suppress_the_defaults() {
    assert_eq!(
        cifs_options("", "guido", "rigid", None, None),
        "username=guido,password=rigid,uid=1000,gid=1000"
    );
}
