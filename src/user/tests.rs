use super::*;

const PDBEDIT: &str = "---------------
Unix username:        user1
NT username:
Account Flags:        [U          ]
User SID:             S-1-5-21-1-2-3-1001
---------------
Unix username:        user2
Account Flags:        [DU         ]
---------------
Unix username:testuser
Account Flags:        [UX         ]
";

#[test]
fn pdbedit_output_yields_one_user_per_record() {
    let users = parse_pdbedit(PDBEDIT);
    let names: Vec<&str> = users.iter().map(|u| u.username.as_str()).collect();
    assert_eq!(names, ["user1", "user2", "testuser"]);
}

#[test]
fn disabled_flag_marks_the_user_disabled() {
    let users = parse_pdbedit(PDBEDIT);
    assert!(users[0].enabled);
    assert!(!users[1].enabled);
    assert!(users[2].enabled);
}

#[test]
fn account_flag_d_is_found_anywhere_in_the_brackets() {
    for (flags, disabled) in [
        ("[U          ]", false),
        ("[UD         ]", true),
        ("[U   D      ]", true),
        ("[           ]", false),
    ] {
        let users = parse_pdbedit(&format!("Unix username: u\nAccount Flags:        {flags}\n"));
        assert_eq!(!users[0].enabled, disabled, "{flags}");
    }
}

#[test]
fn empty_pdbedit_output_has_no_users() {
    assert!(parse_pdbedit("").is_empty());
    assert!(parse_pdbedit("Not a username line\n").is_empty());
}

#[test]
fn username_value_is_trimmed() {
    assert_eq!(
        parse_pdbedit("Unix username:     spaced  \n")[0].username,
        "spaced"
    );
}

const PASSWD: &str = "root:x:0:0:root:/root:/bin/bash
testuser:x:1001:1001:Test User:/home/testuser:/bin/bash
sambauser:x:1002:1002::/home/sambauser:/usr/sbin/nologin
nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin
invalid
too:few
broken:x:notanumber:1001::/home/user:/bin/bash
";

#[test]
fn passwd_lookup_finds_the_uid() {
    assert_eq!(parse_passwd_uid(PASSWD, "root"), Some(0));
    assert_eq!(parse_passwd_uid(PASSWD, "testuser"), Some(1001));
    assert_eq!(parse_passwd_uid(PASSWD, "sambauser"), Some(1002));
    assert_eq!(parse_passwd_uid(PASSWD, "nobody"), Some(65534));
}

#[test]
fn passwd_lookup_misses_unknown_malformed_and_case_mismatched_users() {
    assert_eq!(parse_passwd_uid(PASSWD, "nonexistent"), None);
    assert_eq!(parse_passwd_uid(PASSWD, "broken"), None);
    assert_eq!(parse_passwd_uid(PASSWD, "too"), None);
    assert_eq!(parse_passwd_uid(PASSWD, "Root"), None);
}

#[test]
fn dscl_unique_id_is_parsed() {
    assert_eq!(parse_dscl_uid("UniqueID: 501\n"), Some(501));
    assert_eq!(parse_dscl_uid("No such key: UniqueID\n"), None);
}

#[test]
fn next_macos_uid_follows_the_highest_regular_uid() {
    let listing = "_www                     70\nalice                   501\nbob                     503\nnobody                  4294967294\nnfsnobody             65534\n";
    assert_eq!(next_macos_uid(listing), 504);
}

#[test]
fn next_macos_uid_starts_at_501() {
    assert_eq!(next_macos_uid("root 0\n_www 70\n"), 501);
}
