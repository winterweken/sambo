use super::*;

fn export(path: &str, clients: &str, options: &str) -> Export {
    Export {
        path: path.into(),
        clients: clients.into(),
        options: options.into(),
    }
}

#[test]
fn parses_a_single_client_export() {
    assert_eq!(
        parse_export_line("/srv/nfs 192.168.1.0/24(rw,sync,no_subtree_check)"),
        Some(export("/srv/nfs", "192.168.1.0/24", "rw,sync,no_subtree_check"))
    );
}

#[test]
fn parses_hostname_and_wildcard_clients() {
    assert_eq!(
        parse_export_line("/data host.example.com(ro)"),
        Some(export("/data", "host.example.com", "ro"))
    );
    assert_eq!(
        parse_export_line("/pub *(ro,sync)"),
        Some(export("/pub", "*", "ro,sync"))
    );
}

#[test]
fn multiple_clients_on_one_line_keep_the_first_options() {
    assert_eq!(
        parse_export_line("/srv 10.0.0.1(rw,sync) 10.0.0.2(ro)"),
        Some(export("/srv", "10.0.0.1 10.0.0.2", "rw,sync"))
    );
}

#[test]
fn client_without_options_parses_with_empty_options() {
    assert_eq!(
        parse_export_line("/srv client"),
        Some(export("/srv", "client", ""))
    );
}

#[test]
fn incomplete_lines_are_rejected() {
    assert_eq!(parse_export_line("/srv/nfs"), None);
    assert_eq!(parse_export_line(""), None);
}

#[test]
fn file_parsing_skips_comments_and_blank_lines_and_keeps_order() {
    let content = "# NFS exports\n\n/srv/nfs/public *(ro,sync)\n  # indented comment\n/srv/nfs/private 192.168.1.0/24(rw,sync)\n   \n/srv/nfs/home 10.0.0.0/8(rw,no_root_squash)\n";
    let paths: Vec<String> = parse_exports(content).into_iter().map(|e| e.path).collect();
    assert_eq!(paths, ["/srv/nfs/public", "/srv/nfs/private", "/srv/nfs/home"]);
}

#[test]
fn repeated_paths_are_merged_into_one_export() {
    let content = "/srv 10.0.0.1(rw,sync)\n/other *(ro)\n/srv 10.0.0.2(ro)\n";
    assert_eq!(
        parse_exports(content),
        [
            export("/srv", "10.0.0.1 10.0.0.2", "rw,sync"),
            export("/other", "*", "ro")
        ]
    );
}

#[test]
fn removing_an_export_keeps_comments_and_other_lines() {
    let content = "# header\n/srv/a *(ro)\n\n/srv/b 10.0.0.0/8(rw)\n/srv/a 10.0.0.9(rw)\n";
    assert_eq!(
        remove_export_lines(content, "/srv/a"),
        "# header\n\n/srv/b 10.0.0.0/8(rw)\n"
    );
}

#[test]
fn removing_does_not_match_path_prefixes() {
    let content = "/srv/a *(ro)\n/srv/ab *(ro)\n";
    assert_eq!(remove_export_lines(content, "/srv/a"), "/srv/ab *(ro)\n");
}

#[test]
fn export_line_format() {
    assert_eq!(
        export_line(&export("/home", "192.168.1.0/24", "rw,sync")),
        "/home 192.168.1.0/24(rw,sync)\n"
    );
}

#[test]
fn build_options_covers_each_switch() {
    assert_eq!(
        build_options(false, false, false),
        "rw,sync,root_squash,no_subtree_check"
    );
    assert_eq!(
        build_options(true, true, true),
        "ro,async,no_root_squash,no_subtree_check"
    );
}

#[test]
fn option_flags_are_read_back_by_exact_match() {
    assert_eq!(
        option_flags("ro,async,no_root_squash,no_subtree_check"),
        (true, true, true)
    );
    // "root_squash" must not be mistaken for "no_root_squash", nor "sync" for "async".
    assert_eq!(option_flags("rw,sync,root_squash"), (false, false, false));
}

#[test]
fn every_client_of_an_export_gets_the_options() {
    // In exports(5) options bind to the client they follow, so `a b(rw)` would leave `a` on the defaults.
    assert_eq!(
        export_line(&export("/srv", "10.0.0.0/24 host", "rw,sync")),
        "/srv 10.0.0.0/24(rw,sync) host(rw,sync)\n"
    );
    assert_eq!(export_line(&export("/srv", "*", "ro")), "/srv *(ro)\n");
}

// --- Editing an export in place ---------------------------------------------

const MIXED: &str = "# exports\n/srv a(rw,sync,fsid=1) b(ro,sync)\n/other *(ro)\n";

fn changes(read_only: Option<bool>, async_mode: Option<bool>, no_root_squash: Option<bool>) -> OptionChanges {
    OptionChanges {
        read_only,
        async_mode,
        no_root_squash,
    }
}

#[test]
fn changing_a_flag_keeps_the_other_options() {
    assert_eq!(
        apply_option_changes("rw,sync,fsid=1,anonuid=99", &changes(Some(true), None, None)),
        "ro,sync,fsid=1,anonuid=99"
    );
    assert_eq!(
        apply_option_changes("ro,secure", &changes(None, Some(true), Some(true))),
        "ro,secure,async,no_root_squash"
    );
    assert_eq!(
        apply_option_changes(
            "rw,async,no_root_squash",
            &changes(None, Some(false), Some(false))
        ),
        "rw,sync,root_squash"
    );
    assert_eq!(apply_option_changes("", &changes(Some(true), None, None)), "ro");
}

#[test]
fn no_changes_leave_the_options_untouched() {
    assert_eq!(
        apply_option_changes("rw,fsid=1", &OptionChanges::default()),
        "rw,fsid=1"
    );
}

#[test]
fn editing_clients_keeps_each_clients_own_options() {
    assert_eq!(
        modify_content(MIXED, "/srv", Some("a b c"), &OptionChanges::default()).unwrap(),
        "# exports\n/srv a(rw,sync,fsid=1) b(ro,sync) c(rw,sync,fsid=1)\n/other *(ro)\n"
    );
    assert_eq!(
        modify_content(MIXED, "/srv", Some("b"), &OptionChanges::default()).unwrap(),
        "# exports\n/srv b(ro,sync)\n/other *(ro)\n"
    );
}

#[test]
fn an_edit_without_changes_leaves_the_file_as_it_was() {
    assert_eq!(
        modify_content(MIXED, "/srv", Some("a b"), &OptionChanges::default()).unwrap(),
        MIXED
    );
}

#[test]
fn a_changed_flag_applies_to_every_client_and_nothing_else() {
    assert_eq!(
        modify_content(MIXED, "/srv", None, &changes(None, Some(true), None)).unwrap(),
        "# exports\n/srv a(rw,async,fsid=1) b(ro,async)\n/other *(ro)\n"
    );
}

#[test]
fn an_export_spread_over_several_lines_is_merged_where_it_first_appears() {
    let content = "/srv a(rw)\n/other *(ro)\n/srv b(ro)\n";
    assert_eq!(
        modify_content(content, "/srv", None, &changes(None, None, Some(true))).unwrap(),
        "/srv a(rw,no_root_squash) b(ro,no_root_squash)\n/other *(ro)\n"
    );
}

#[test]
fn editing_a_missing_export_is_an_error() {
    assert!(modify_content(MIXED, "/nope", None, &OptionChanges::default())
        .unwrap_err()
        .contains("not found"));
}

#[test]
fn exports_using_default_option_syntax_are_not_rewritten() {
    let content = "/srv -rw,sync a b\n";
    assert!(modify_content(content, "/srv", None, &OptionChanges::default())
        .unwrap_err()
        .contains("by hand"));
}
