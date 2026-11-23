use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};

use super::*;
use crate::mount::discover::{NfsExport, NfsServer};
use crate::mount::MountType;
use crate::service::Status;

fn key(code: KeyCode) -> KeyEvent {
    KeyEvent::new(code, KeyModifiers::NONE)
}

fn ctrl(c: char) -> KeyEvent {
    KeyEvent::new(KeyCode::Char(c), KeyModifiers::CONTROL)
}

fn press(app: &mut App, c: char) {
    app.on_key(key(KeyCode::Char(c)));
}

fn type_text(app: &mut App, text: &str) {
    text.chars().for_each(|c| press(app, c));
}

fn share(name: &str) -> Share {
    Share {
        name: name.into(),
        path: format!("/srv/{name}"),
        browseable: true,
        ..Share::default()
    }
}

fn export(path: &str) -> Export {
    Export {
        path: path.into(),
        clients: "*".into(),
        options: "rw,sync,no_subtree_check".into(),
    }
}

fn mount(mount_point: &str, persistent: bool) -> Mount {
    Mount {
        source: "nas:/data".into(),
        mount_point: mount_point.into(),
        kind: MountType::Nfs,
        options: "rw".into(),
        persistent,
        active: true,
    }
}

fn user(name: &str) -> User {
    User {
        username: name.into(),
        uid: 1001,
        enabled: true,
    }
}

fn system(package_manager: Option<&'static str>) -> SystemInfo {
    SystemInfo {
        package_manager,
        components: vec![
            Component {
                key: "samba",
                name: "Samba Server",
                installed: true,
            },
            Component {
                key: "nfs",
                name: "NFS Server",
                installed: false,
            },
        ],
        services: vec![("Samba", Status::Running)],
    }
}

/// An app whose tabs have all loaded with a little data.
fn app() -> App {
    let mut app = App::new(Theme::plain());
    app.take_requests();
    load(
        &mut app,
        Loaded::Shares(vec![share("alpha"), share("beta"), share("gamma")]),
    );
    load(
        &mut app,
        Loaded::Exports(vec![export("/srv/a"), export("/srv/b")]),
    );
    load(
        &mut app,
        Loaded::Mounts(vec![mount("/mnt/a", true), mount("/mnt/b", false)]),
    );
    load(&mut app, Loaded::Users(vec![user("ann"), user("bob")]));
    load(&mut app, Loaded::System(system(Some("apt"))));
    app
}

fn load(app: &mut App, loaded: Loaded) {
    let tab = match loaded {
        Loaded::Shares(_) => Tab::Shares,
        Loaded::Exports(_) => Tab::Exports,
        Loaded::Mounts(_) => Tab::Mounts,
        Loaded::Users(_) => Tab::Users,
        Loaded::System(_) => Tab::System,
    };
    app.on_response(Response::Loaded(tab, Ok(loaded)));
}

fn on_tab(tab: Tab) -> App {
    let mut app = app();
    app.tab = tab;
    app
}

fn form(app: &App) -> &Form {
    match &app.modal {
        Some(Modal::Form(_, form)) => form,
        other => panic!("expected a form, found {other:?}"),
    }
}

fn confirm(app: &App) -> &Confirm {
    match &app.modal {
        Some(Modal::Confirm(confirm)) => confirm,
        other => panic!("expected a confirmation, found {other:?}"),
    }
}

fn level(app: &App) -> Option<Level> {
    app.notice.as_ref().map(|n| n.level)
}

// ── start-up and loading ────────────────────────────────────────────────

#[test]
fn starting_requests_every_tab() {
    let mut app = App::new(Theme::plain());
    assert_eq!(app.take_requests(), Tab::ALL.map(Request::Load).to_vec());
    assert_eq!(app.loading, [true; 5]);
    assert!(app.take_requests().is_empty());
}

#[test]
fn loaded_data_replaces_the_old_and_ends_loading() {
    let app = app();
    assert_eq!(app.loading, [false; 5]);
    assert_eq!(app.data.shares.len(), 3);
    assert_eq!(app.rows(Tab::Mounts), 2);
    assert_eq!(app.rows(Tab::System), 2);
}

#[test]
fn a_failed_load_is_kept_as_the_tab_error() {
    let mut app = app();
    app.on_response(Response::Loaded(Tab::Users, Err("pdbedit missing".into())));
    assert_eq!(app.errors[Tab::Users.index()].as_deref(), Some("pdbedit missing"));
    assert!(app.data.users.is_empty());
    load(&mut app, Loaded::Users(vec![user("ann")]));
    assert_eq!(app.errors[Tab::Users.index()], None);
}

#[test]
fn r_reloads_the_current_tab() {
    let mut app = on_tab(Tab::Exports);
    press(&mut app, 'r');
    assert_eq!(app.take_requests(), vec![Request::Load(Tab::Exports)]);
    assert!(app.loading[Tab::Exports.index()]);
}

// ── navigation ──────────────────────────────────────────────────────────

#[test]
fn tabs_cycle_in_both_directions() {
    let mut app = app();
    app.on_key(key(KeyCode::Tab));
    assert_eq!(app.tab, Tab::Exports);
    app.on_key(key(KeyCode::BackTab));
    app.on_key(key(KeyCode::BackTab));
    assert_eq!(app.tab, Tab::System);
    app.on_key(key(KeyCode::Right));
    assert_eq!(app.tab, Tab::Shares);
}

#[test]
fn number_keys_jump_to_a_tab() {
    let mut app = app();
    press(&mut app, '4');
    assert_eq!(app.tab, Tab::Users);
}

#[test]
fn selection_moves_and_stops_at_the_ends() {
    let mut app = app();
    press(&mut app, 'k');
    assert_eq!(app.selected[0], 0);
    press(&mut app, 'j');
    app.on_key(key(KeyCode::Down));
    app.on_key(key(KeyCode::Down));
    assert_eq!(app.selected[0], 2);
    app.on_key(key(KeyCode::Home));
    assert_eq!(app.selected[0], 0);
    app.on_key(key(KeyCode::End));
    assert_eq!(app.selected[0], 2);
}

#[test]
fn selection_follows_the_same_item_across_a_reload() {
    let mut app = app();
    press(&mut app, 'j');
    press(&mut app, 'j');
    load(&mut app, Loaded::Shares(vec![share("gamma"), share("alpha")]));
    assert_eq!(app.selected[0], 0);
}

#[test]
fn selection_is_clamped_when_the_selected_item_disappears() {
    let mut app = app();
    press(&mut app, 'j');
    press(&mut app, 'j');
    load(&mut app, Loaded::Shares(vec![share("alpha")]));
    assert_eq!(app.selected[0], 0);
}

#[test]
fn question_mark_opens_help_and_escape_closes_it() {
    let mut app = app();
    press(&mut app, '?');
    assert_eq!(app.modal, Some(Modal::Help));
    app.on_key(key(KeyCode::Esc));
    assert_eq!(app.modal, None);
}

// ── quitting ────────────────────────────────────────────────────────────

#[test]
fn q_and_ctrl_c_quit() {
    let mut app = app();
    press(&mut app, 'q');
    assert!(app.should_quit);
    let mut app = self::app();
    app.on_key(ctrl('c'));
    assert!(app.should_quit);
}

#[test]
fn q_typed_into_a_form_does_not_quit_but_ctrl_c_does() {
    let mut app = app();
    press(&mut app, 'a');
    press(&mut app, 'q');
    assert!(!app.should_quit);
    assert_eq!(form(&app).text("name"), "q");
    app.on_key(ctrl('c'));
    assert!(app.should_quit);
}

#[test]
fn quitting_during_an_operation_needs_a_second_ctrl_c() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 'p');
    assert!(app.busy.is_some());
    app.on_key(ctrl('c'));
    assert!(!app.should_quit);
    assert_eq!(level(&app), Some(Level::Warning));
    app.on_key(ctrl('c'));
    assert!(app.should_quit);
}

// ── shares ──────────────────────────────────────────────────────────────

#[test]
fn creating_a_share_sends_the_form_values() {
    let mut app = app();
    press(&mut app, 'a');
    type_text(&mut app, "media");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "/srv/media");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "Films");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "ann, bob");
    app.on_key(ctrl('s'));

    let expected = Share {
        name: "media".into(),
        path: "/srv/media".into(),
        comment: "Films".into(),
        read_only: false,
        browseable: true,
        valid_users: vec!["ann".into(), "bob".into()],
        share_type: "general".into(),
        time_machine: false,
        time_machine_max_size: "0".into(),
    };
    assert_eq!(app.take_requests(), vec![Request::CreateShare(expected)]);
    assert!(app.busy.is_some());
    assert!(matches!(app.modal, Some(Modal::Form(Purpose::CreateShare, _))));
}

#[test]
fn choosing_the_time_machine_type_marks_the_share() {
    let mut app = app();
    press(&mut app, 'a');
    type_text(&mut app, "backup");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "/srv/backup");
    for _ in 0..3 {
        app.on_key(key(KeyCode::Tab));
    }
    app.on_key(key(KeyCode::Right));
    app.on_key(ctrl('s'));
    match app.take_requests().as_slice() {
        [Request::CreateShare(share)] => {
            assert!(share.time_machine);
            assert_eq!(share.share_type, "timemachine");
        }
        other => panic!("unexpected requests {other:?}"),
    }
}

#[test]
fn a_share_without_name_or_path_is_rejected_in_the_form() {
    let mut app = app();
    press(&mut app, 'a');
    app.on_key(ctrl('s'));
    assert!(app.take_requests().is_empty());
    assert!(app.busy.is_none());
    assert_eq!(
        form(&app).error.as_deref(),
        Some("Share name and path are required")
    );
}

#[test]
fn an_invalid_share_name_is_rejected_in_the_form() {
    let mut app = app();
    press(&mut app, 'a');
    type_text(&mut app, "bad/name");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "/srv/x");
    app.on_key(ctrl('s'));
    assert!(app.take_requests().is_empty());
    assert!(form(&app).error.is_some());
}

#[test]
fn success_closes_the_form_reports_it_and_reloads() {
    let mut app = app();
    press(&mut app, 'a');
    type_text(&mut app, "media");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "/srv/media");
    app.on_key(ctrl('s'));
    app.take_requests();

    app.on_response(Response::Done(Ok(vec![])));
    assert_eq!(app.modal, None);
    assert!(app.busy.is_none());
    assert_eq!(level(&app), Some(Level::Success));
    assert!(app.notice.as_ref().unwrap().text.contains("media"));
    assert_eq!(app.take_requests(), vec![Request::Load(Tab::Shares)]);
}

#[test]
fn warnings_are_shown_with_the_success_message() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 'p');
    app.on_response(Response::Done(Ok(vec!["smbd is not running".into()])));
    let notice = app.notice.clone().unwrap();
    assert_eq!(notice.level, Level::Warning);
    assert!(notice.text.contains("smbd is not running"));
}

#[test]
fn failure_keeps_the_form_open_with_the_error() {
    let mut app = app();
    press(&mut app, 'a');
    type_text(&mut app, "media");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "/srv/media");
    app.on_key(ctrl('s'));
    app.take_requests();

    app.on_response(Response::Done(Err("share already exists".into())));
    assert!(app.busy.is_none());
    assert_eq!(form(&app).error.as_deref(), Some("share already exists"));
    assert_eq!(form(&app).text("name"), "media");
    // The change may have been applied in part, so the list is refreshed.
    assert_eq!(app.take_requests(), vec![Request::Load(Tab::Shares)]);
}

#[test]
fn keys_are_ignored_while_a_form_is_being_submitted() {
    let mut app = app();
    press(&mut app, 'a');
    type_text(&mut app, "media");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "/srv/media");
    app.on_key(ctrl('s'));
    app.take_requests();
    type_text(&mut app, "zzz");
    app.on_key(ctrl('s'));
    app.on_key(key(KeyCode::Esc));
    assert_eq!(form(&app).text("path"), "/srv/media");
    assert!(app.take_requests().is_empty());
}

#[test]
fn editing_a_share_prefills_the_form_and_sends_an_update() {
    let mut app = app();
    app.data.shares[1].comment = "old".into();
    app.data.shares[1].valid_users = vec!["ann".into()];
    press(&mut app, 'j');
    press(&mut app, 'e');
    assert_eq!(form(&app).text("name"), "beta");
    assert_eq!(form(&app).text("comment"), "old");
    assert_eq!(form(&app).text("users"), "ann");
    assert!(form(&app).checked("browseable"));

    type_text(&mut app, "er");
    app.on_key(key(KeyCode::Tab));
    app.on_key(key(KeyCode::Tab));
    press(&mut app, ' ');
    app.on_key(ctrl('s'));
    let update = ShareUpdate {
        comment: Some("older".into()),
        read_only: Some(true),
        browseable: Some(true),
        valid_users: Some(vec!["ann".into()]),
        time_machine: Some(false),
    };
    assert_eq!(
        app.take_requests(),
        vec![Request::ModifyShare {
            name: "beta".into(),
            update
        }]
    );
}

#[test]
fn removing_a_share_asks_first() {
    let mut app = app();
    press(&mut app, 'd');
    assert_eq!(confirm(&app).action, Action::RemoveShare("alpha".into()));
    press(&mut app, 'n');
    assert_eq!(app.modal, None);
    assert!(app.take_requests().is_empty());

    press(&mut app, 'd');
    press(&mut app, 'y');
    assert_eq!(app.take_requests(), vec![Request::RemoveShare("alpha".into())]);
    assert_eq!(app.modal, None);
    assert!(app.busy.is_some());
}

#[test]
fn enter_does_not_confirm_a_removal() {
    let mut app = app();
    press(&mut app, 'd');
    app.on_key(key(KeyCode::Enter));
    assert!(app.take_requests().is_empty());
}

#[test]
fn a_failed_removal_is_reported_on_the_status_line() {
    let mut app = app();
    press(&mut app, 'd');
    press(&mut app, 'y');
    app.take_requests();
    app.on_response(Response::Done(Err("cannot write smb.conf".into())));
    assert_eq!(level(&app), Some(Level::Error));
    assert!(app.busy.is_none());
}

#[test]
fn actions_on_an_empty_list_do_nothing() {
    let mut app = app();
    load(&mut app, Loaded::Shares(vec![]));
    press(&mut app, 'd');
    press(&mut app, 'e');
    assert_eq!(app.modal, None);
}

#[test]
fn actions_are_ignored_while_an_operation_runs() {
    let mut app = app();
    press(&mut app, 'd');
    press(&mut app, 'y');
    app.take_requests();
    press(&mut app, 'a');
    press(&mut app, 'd');
    assert_eq!(app.modal, None);
    press(&mut app, 'j');
    assert_eq!(app.selected[0], 1);
}

// ── exports ─────────────────────────────────────────────────────────────

#[test]
fn creating_an_export_builds_its_options() {
    let mut app = on_tab(Tab::Exports);
    press(&mut app, 'a');
    assert_eq!(form(&app).text("clients"), "*");
    type_text(&mut app, "/srv/data");
    app.on_key(key(KeyCode::Tab));
    app.on_key(key(KeyCode::Tab));
    press(&mut app, ' ');
    app.on_key(ctrl('s'));
    let expected = Export {
        path: "/srv/data".into(),
        clients: "*".into(),
        options: crate::nfs::build_options(true, false, false),
    };
    assert_eq!(app.take_requests(), vec![Request::CreateExport(expected)]);
}

#[test]
fn an_export_needs_a_path() {
    let mut app = on_tab(Tab::Exports);
    press(&mut app, 'a');
    app.on_key(ctrl('s'));
    assert!(app.take_requests().is_empty());
    assert!(form(&app).error.is_some());
}

#[test]
fn editing_an_export_prefills_its_flags() {
    let mut app = on_tab(Tab::Exports);
    app.data.exports[0].options = crate::nfs::build_options(true, true, false);
    press(&mut app, 'e');
    assert!(form(&app).checked("read_only"));
    assert!(form(&app).checked("async"));
    assert!(!form(&app).checked("no_root_squash"));
    type_text(&mut app, " 10.0.0.0/24");
    app.on_key(ctrl('s'));
    assert_eq!(
        app.take_requests(),
        vec![Request::ModifyExport {
            path: "/srv/a".into(),
            clients: "* 10.0.0.0/24".into(),
            options: crate::nfs::build_options(true, true, false),
        }]
    );
}

#[test]
fn removing_an_export_asks_first() {
    let mut app = on_tab(Tab::Exports);
    press(&mut app, 'j');
    press(&mut app, 'd');
    press(&mut app, 'y');
    assert_eq!(app.take_requests(), vec![Request::RemoveExport("/srv/b".into())]);
}

// ── mounts ──────────────────────────────────────────────────────────────

#[test]
fn mounting_a_cifs_share_builds_the_credentials_options() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 'c');
    type_text(&mut app, "//nas/media");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "/mnt/media");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "ann");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "pw");
    app.on_key(key(KeyCode::Tab));
    press(&mut app, ' ');
    app.on_key(ctrl('s'));
    assert_eq!(
        app.take_requests(),
        vec![Request::MountCifs {
            source: "//nas/media".into(),
            mount_point: "/mnt/media".into(),
            options: "username=ann,password=pw,uid=1000,gid=1000".into(),
            persistent: true,
        }]
    );
}

#[test]
fn an_invalid_cifs_source_is_rejected_in_the_form() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 'c');
    type_text(&mut app, "nas");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "/mnt/media");
    app.on_key(ctrl('s'));
    assert!(app.take_requests().is_empty());
    assert!(form(&app).error.is_some());
}

#[test]
fn mounting_an_nfs_export() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 'n');
    type_text(&mut app, "nas:/data");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "/mnt/data");
    app.on_key(ctrl('s'));
    assert_eq!(
        app.take_requests(),
        vec![Request::MountNfs {
            source: "nas:/data".into(),
            mount_point: "/mnt/data".into(),
            persistent: false
        }]
    );
}

#[test]
fn unmounting_offers_to_remove_the_persistent_entry() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 'u');
    assert_eq!(confirm(&app).toggle.map(|t| t.1), Some(true));
    press(&mut app, ' ');
    assert_eq!(confirm(&app).toggle.map(|t| t.1), Some(false));
    press(&mut app, 'y');
    assert_eq!(
        app.take_requests(),
        vec![Request::Unmount {
            mount_point: "/mnt/a".into(),
            remove_persistent: false
        }]
    );
}

#[test]
fn unmounting_a_temporary_mount_has_no_option() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 'j');
    press(&mut app, 'u');
    assert_eq!(confirm(&app).toggle, None);
    press(&mut app, 'y');
    assert_eq!(
        app.take_requests(),
        vec![Request::Unmount {
            mount_point: "/mnt/b".into(),
            remove_persistent: false
        }]
    );
}

#[test]
fn p_toggles_whether_a_mount_is_persistent() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 'p');
    assert_eq!(
        app.take_requests(),
        vec![Request::SetPersistent {
            mount_point: "/mnt/a".into(),
            persistent: false
        }]
    );
    app.on_response(Response::Done(Ok(vec![])));
    assert_eq!(app.take_requests(), vec![Request::Load(Tab::Mounts)]);
}

// ── scanning ────────────────────────────────────────────────────────────

fn scan_result() -> ScanResult {
    let export = |path: &str, clients: &[&str]| NfsExport {
        server: "10.0.0.5".into(),
        path: path.into(),
        clients: clients.iter().map(|c| c.to_string()).collect(),
    };
    ScanResult {
        subnet: "10.0.0.0/24".into(),
        servers: vec![NfsServer {
            ip: "10.0.0.5".into(),
            exports: vec![export("/a", &[]), export("/b", &["10.0.0.0/24", "host"])],
        }],
    }
}

#[test]
fn s_starts_a_scan_without_blocking() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 's');
    assert_eq!(app.take_requests(), vec![Request::Scan]);
    assert_eq!(app.modal, Some(Modal::Scan(ScanView::Running)));
    assert!(app.busy.is_none());
}

#[test]
fn scan_results_are_listed_one_row_per_export() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 's');
    app.on_response(Response::Scanned(Ok(scan_result())));
    match &app.modal {
        Some(Modal::Scan(ScanView::Results {
            subnet,
            rows,
            selected,
        })) => {
            assert_eq!(subnet, "10.0.0.0/24");
            assert_eq!(*selected, 0);
            assert_eq!(rows.len(), 2);
            assert_eq!(rows[0].clients, "*");
            assert_eq!(rows[1].clients, "10.0.0.0/24 host");
        }
        other => panic!("unexpected modal {other:?}"),
    }
}

#[test]
fn enter_on_a_scan_result_opens_the_mount_form_for_it() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 's');
    app.on_response(Response::Scanned(Ok(scan_result())));
    press(&mut app, 'j');
    app.on_key(key(KeyCode::Enter));
    assert!(matches!(app.modal, Some(Modal::Form(Purpose::MountNfs, _))));
    assert_eq!(form(&app).text("source"), "10.0.0.5:/b");
}

#[test]
fn a_failed_scan_is_shown_in_the_dialog() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 's');
    app.on_response(Response::Scanned(Err("no network".into())));
    assert_eq!(
        app.modal,
        Some(Modal::Scan(ScanView::Failed("no network".into())))
    );
}

#[test]
fn results_of_a_cancelled_scan_are_dropped() {
    let mut app = on_tab(Tab::Mounts);
    press(&mut app, 's');
    app.on_key(key(KeyCode::Esc));
    app.on_response(Response::Scanned(Ok(scan_result())));
    assert_eq!(app.modal, None);
}

// ── users ───────────────────────────────────────────────────────────────

#[test]
fn adding_a_user() {
    let mut app = on_tab(Tab::Users);
    press(&mut app, 'a');
    type_text(&mut app, "carol");
    app.on_key(key(KeyCode::Tab));
    type_text(&mut app, "s3cret");
    app.on_key(ctrl('s'));
    assert_eq!(
        app.take_requests(),
        vec![Request::AddUser {
            username: "carol".into(),
            password: "s3cret".into()
        }]
    );
}

#[test]
fn a_user_needs_a_password() {
    let mut app = on_tab(Tab::Users);
    press(&mut app, 'a');
    type_text(&mut app, "carol");
    app.on_key(ctrl('s'));
    assert!(app.take_requests().is_empty());
    assert!(form(&app).error.is_some());
}

#[test]
fn changing_the_selected_users_password() {
    let mut app = on_tab(Tab::Users);
    press(&mut app, 'j');
    press(&mut app, 'p');
    assert_eq!(form(&app).text("username"), "bob");
    type_text(&mut app, "newpw");
    app.on_key(ctrl('s'));
    assert_eq!(
        app.take_requests(),
        vec![Request::SetPassword {
            username: "bob".into(),
            password: "newpw".into()
        }]
    );
}

#[test]
fn removing_a_user_asks_first() {
    let mut app = on_tab(Tab::Users);
    press(&mut app, 'd');
    press(&mut app, 'y');
    assert_eq!(app.take_requests(), vec![Request::RemoveUser("ann".into())]);
}

// ── system ──────────────────────────────────────────────────────────────

#[test]
fn installing_a_missing_component_asks_first_and_reloads_everything() {
    let mut app = on_tab(Tab::System);
    press(&mut app, 'j');
    app.on_key(key(KeyCode::Enter));
    assert_eq!(confirm(&app).action, Action::Install("nfs"));
    press(&mut app, 'y');
    assert_eq!(app.take_requests(), vec![Request::Install("nfs")]);
    app.on_response(Response::Done(Ok(vec![])));
    assert_eq!(app.take_requests(), Tab::ALL.map(Request::Load).to_vec());
}

#[test]
fn an_installed_component_is_not_installed_again() {
    let mut app = on_tab(Tab::System);
    press(&mut app, 'i');
    assert_eq!(app.modal, None);
    assert_eq!(
        app.notice.as_ref().unwrap().text,
        "Samba Server is already installed"
    );
}

#[test]
fn nothing_can_be_installed_without_a_package_manager() {
    let mut app = on_tab(Tab::System);
    load(&mut app, Loaded::System(system(None)));
    press(&mut app, 'j');
    press(&mut app, 'i');
    assert_eq!(app.modal, None);
    assert_eq!(level(&app), Some(Level::Error));
}
