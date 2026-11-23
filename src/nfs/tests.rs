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
