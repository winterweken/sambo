use super::*;
use std::collections::HashMap;
use std::io;
use std::sync::Mutex;

const CONF: &str = "/etc/samba/smb.conf";
const BACKUP: &str = "/etc/samba/smb.conf.backup";

const BASE: &str = "[global]
   workgroup = WORKGROUP

[docs]
   path = /srv/docs
   comment = Documents
   read only = no
   browseable = yes
   valid users = alice bob
";

// --- Test doubles -----------------------------------------------------------

#[derive(Default)]
struct MockFs {
    files: Mutex<HashMap<String, String>>,
    dirs: Vec<String>,
    chowns: Mutex<Vec<(String, u32)>>,
    chmods: Mutex<Vec<(String, u32)>>,
}

impl MockFs {
    fn file(&self, path: &str) -> Option<String> {
        self.files.lock().unwrap().get(path).cloned()
    }
}

impl FileSystem for MockFs {
    fn read_to_string(&self, path: &str) -> io::Result<String> {
        self.file(path)
            .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "file does not exist"))
    }
    fn write(&self, path: &str, data: &str) -> io::Result<()> {
        self.files.lock().unwrap().insert(path.into(), data.into());
        Ok(())
    }
    fn exists(&self, path: &str) -> bool {
        self.dirs.iter().any(|d| d == path) || self.file(path).is_some()
    }
    fn create_dir_all(&self, _path: &str) -> io::Result<()> {
        Ok(())
    }
    fn rename(&self, from: &str, to: &str) -> io::Result<()> {
        let mut files = self.files.lock().unwrap();
        let data = files
            .remove(from)
            .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "missing"))?;
        files.insert(to.into(), data);
        Ok(())
    }
    fn remove_file(&self, path: &str) -> io::Result<()> {
        self.files.lock().unwrap().remove(path);
        Ok(())
    }
    fn chown(&self, path: &str, uid: u32) -> io::Result<()> {
        self.chowns.lock().unwrap().push((path.into(), uid));
        Ok(())
    }
    fn chmod(&self, path: &str, mode: u32) -> io::Result<()> {
        self.chmods.lock().unwrap().push((path.into(), mode));
        Ok(())
    }
}

#[derive(Default)]
struct MockExec {
    calls: Mutex<Vec<String>>,
    available: Vec<&'static str>,
    /// Commands whose line starts with one of these fail.
    failing: Vec<&'static str>,
}

impl CommandExecutor for MockExec {
    fn run(&self, name: &str, args: &[&str]) -> Result<()> {
        let line = format!("{name} {}", args.join(" "));
        self.calls.lock().unwrap().push(line.clone());
        if self.failing.iter().any(|f| line.starts_with(f)) {
            return Err(format!("{name} failed"));
        }
        Ok(())
    }
    fn has(&self, name: &str) -> bool {
        self.available.contains(&name)
    }
}

#[derive(Default)]
struct MockPlatform {
    macos: bool,
    uids: HashMap<String, u32>,
    warning: Option<String>,
}

impl Platform for MockPlatform {
    fn is_macos(&self) -> bool {
        self.macos
    }
    fn samba_config_path(&self) -> String {
        CONF.into()
    }
    fn samba_config_dir(&self) -> String {
        "/etc/samba".into()
    }
    fn system_uid(&self, username: &str) -> Result<u32> {
        self.uids
            .get(username)
            .copied()
            .ok_or_else(|| "user not found".to_string())
    }
    fn service_warning(&self, _service: &str) -> Option<String> {
        self.warning.clone()
    }
}

#[derive(Default)]
struct MockAvahi {
    calls: Mutex<Vec<String>>,
}

impl AvahiManager for MockAvahi {
    fn add_time_machine_share(&self, name: &str, existing: &[String]) -> Result<()> {
        self.calls
            .lock()
            .unwrap()
            .push(format!("add {name} {existing:?}"));
        Ok(())
    }
    fn remove_time_machine_share(&self, name: &str, remaining: &[String]) -> Result<()> {
        self.calls
            .lock()
            .unwrap()
            .push(format!("remove {name} {remaining:?}"));
        Ok(())
    }
}

struct Rig {
    fs: Arc<MockFs>,
    exec: Arc<MockExec>,
    avahi: Arc<MockAvahi>,
    manager: Manager,
}

impl Rig {
    fn build(fs: MockFs, exec: MockExec, platform: MockPlatform) -> Rig {
        let fs = Arc::new(fs);
        let exec = Arc::new(exec);
        let avahi = Arc::new(MockAvahi::default());
        let manager = Manager::new(fs.clone(), exec.clone(), Arc::new(platform), avahi.clone());
        Rig {
            fs,
            exec,
            avahi,
            manager,
        }
    }

    /// Linux host with BASE as smb.conf and the given directories present.
    fn linux(dirs: &[&str]) -> Rig {
        Rig::build(fs_with(BASE, dirs), MockExec::default(), platform_with_users())
    }

    fn conf(&self) -> String {
        self.fs.file(CONF).unwrap()
    }
    fn calls(&self) -> Vec<String> {
        self.exec.calls.lock().unwrap().clone()
    }
    fn avahi_calls(&self) -> Vec<String> {
        self.avahi.calls.lock().unwrap().clone()
    }
}

fn fs_with(conf: &str, dirs: &[&str]) -> MockFs {
    let fs = MockFs {
        dirs: dirs.iter().map(|d| d.to_string()).collect(),
        ..Default::default()
    };
    fs.files.lock().unwrap().insert(CONF.into(), conf.into());
    fs
}

fn platform_with_users() -> MockPlatform {
    MockPlatform {
        uids: [("alice".to_string(), 1001), ("bob".to_string(), 1002)].into(),
        ..Default::default()
    }
}

fn share(name: &str, path: &str) -> Share {
    Share {
        name: name.into(),
        path: path.into(),
        browseable: true,
        ..Default::default()
    }
}

fn users(names: &[&str]) -> Vec<String> {
    names.iter().map(|n| n.to_string()).collect()
}

// --- Parsing ----------------------------------------------------------------

#[test]
fn parses_a_share_and_skips_global() {
    let shares = parse_shares(BASE);
    assert_eq!(shares.len(), 1);
    let docs = &shares[0];
    assert_eq!(docs.name, "docs");
    assert_eq!(docs.path, "/srv/docs");
    assert_eq!(docs.comment, "Documents");
    assert!(!docs.read_only);
    assert!(docs.browseable);
    assert_eq!(docs.valid_users, ["alice", "bob"]);
}

#[test]
fn skips_special_sections_comments_and_blank_lines() {
    let conf = "[global]\n   path = /nope\n[homes]\n   path = /home\n[printers]\n   path = /var/spool\n# [commented]\n; path = /x\n\n[real]\n   path = /srv/real\n";
    let shares = parse_shares(conf);
    assert_eq!(shares.len(), 1);
    assert_eq!(shares[0].name, "real");
    assert_eq!(shares[0].path, "/srv/real");
}

#[test]
fn parses_multiple_shares_in_file_order() {
    let conf = "[a]\n   path = /a\n   read only = yes\n[b]\n   path = /b\n";
    let shares = parse_shares(conf);
    assert_eq!(
        shares.iter().map(|s| s.name.as_str()).collect::<Vec<_>>(),
        ["a", "b"]
    );
    assert!(shares[0].read_only);
    assert!(!shares[1].read_only);
}

#[test]
fn browseable_defaults_to_yes_and_accepts_both_spellings() {
    let conf = "[a]\n   path = /a\n[b]\n   browseable = no\n[c]\n   browsable = No\n";
    let shares = parse_shares(conf);
    assert!(shares[0].browseable);
    assert!(!shares[1].browseable);
    assert!(!shares[2].browseable);
}

#[test]
fn parses_time_machine_settings() {
    let conf = "[tm]\n   path = /tm\n   fruit:time machine = yes\n   fruit:time machine max size = 500G\n";
    let tm = &parse_shares(conf)[0];
    assert!(tm.time_machine);
    assert_eq!(tm.time_machine_max_size, "500G");
    assert_eq!(tm.effective_type(), TIME_MACHINE);
}

#[test]
fn valid_users_may_include_groups_and_may_be_empty() {
    let conf = "[a]\n   valid users = user1 @group1\n[b]\n   valid users =\n";
    let shares = parse_shares(conf);
    assert_eq!(shares[0].valid_users, ["user1", "@group1"]);
    assert!(shares[1].valid_users.is_empty());
}

#[test]
fn preset_type_is_recognised_from_a_rendered_share() {
    for kind in [UNIFI_PROTECT, MEDIA] {
        let original = Share {
            share_type: kind.into(),
            ..share("s", "/srv/s")
        };
        let parsed = parse_shares(&render_share(&original));
        assert_eq!(parsed[0].effective_type(), kind);
    }
}

#[test]
fn effective_type_defaults_to_general_and_time_machine_flag_wins() {
    assert_eq!(share("s", "/s").effective_type(), GENERAL);
    let tm = Share {
        time_machine: true,
        share_type: MEDIA.into(),
        ..share("s", "/s")
    };
    assert_eq!(tm.effective_type(), TIME_MACHINE);
}

// --- Rendering --------------------------------------------------------------

#[test]
fn renders_a_general_share() {
    let s = Share {
        comment: "Docs".into(),
        valid_users: users(&["user1", "user2"]),
        ..share("docs", "/srv/docs")
    };
    assert_eq!(
        render_share(&s),
        "\n[docs]\n   path = /srv/docs\n   comment = Docs\n   read only = no\n   browseable = yes\n   valid users = user1 user2\n"
    );
}

#[test]
fn rendering_omits_empty_comment_and_users() {
    let s = Share {
        read_only: true,
        browseable: false,
        ..share("x", "/x")
    };
    assert_eq!(
        render_share(&s),
        "\n[x]\n   path = /x\n   read only = yes\n   browseable = no\n"
    );
}

#[test]
fn time_machine_share_gets_fruit_settings_and_optional_max_size() {
    let unlimited = render_share(&Share {
        share_type: TIME_MACHINE.into(),
        ..share("tm", "/tm")
    });
    assert!(unlimited.contains("   vfs objects = catia fruit streams_xattr\n"));
    assert!(unlimited.contains("   fruit:time machine = yes\n"));
    assert!(unlimited.contains("   posix locking = no\n"));
    assert!(!unlimited.contains("max size"));

    let limited = render_share(&Share {
        time_machine: true,
        time_machine_max_size: "500G".into(),
        ..share("tm", "/tm")
    });
    assert!(limited.contains("   fruit:time machine max size = 500G\n"));
}

#[test]
fn preset_shares_get_their_tuning() {
    let unifi = render_share(&Share {
        share_type: UNIFI_PROTECT.into(),
        ..share("p", "/p")
    });
    assert!(unifi.contains("   force directory mode = 0770\n"));
    let media = render_share(&Share {
        share_type: MEDIA.into(),
        ..share("m", "/m")
    });
    assert!(media.contains("   use sendfile = yes\n"));
}

// --- Section removal and global settings ------------------------------------

#[test]
fn removing_a_section_leaves_the_others_intact() {
    let conf = "[global]\n   workgroup = W\n\n[a]\n   path = /a\n\n[b]\n   path = /b\n";
    assert_eq!(
        remove_share_section(conf, "a"),
        "[global]\n   workgroup = W\n\n[b]\n   path = /b\n"
    );
}

#[test]
fn removing_the_last_section_drops_everything_after_its_header() {
    assert_eq!(
        remove_share_section("[a]\n   path = /a\n[b]\n   path = /b\n", "b"),
        "[a]\n   path = /a"
    );
}

#[test]
fn time_machine_globals_are_added_inside_the_global_section() {
    let out = add_time_machine_globals(BASE).unwrap();
    let global_end = out.find("[docs]").unwrap();
    for setting in [
        "   min protocol = SMB2",
        "   ea support = yes",
        "   vfs objects = catia fruit streams_xattr",
    ] {
        let at = out.find(setting).unwrap_or_else(|| panic!("missing {setting}"));
        assert!(at < global_end, "{setting} landed outside [global]");
    }
    assert!(out.ends_with("valid users = alice bob\n"));
}

#[test]
fn only_missing_time_machine_globals_are_added() {
    let conf = "[global]\n   min protocol = SMB2\n\n[docs]\n   path = /srv/docs\n";
    let out = add_time_machine_globals(conf).unwrap();
    assert_eq!(out.matches("min protocol").count(), 1);
    assert!(out.contains("   ea support = yes"));
}

#[test]
fn a_shares_own_settings_do_not_count_as_time_machine_globals() {
    let conf = "[global]\n   workgroup = W\n\n[protect]\n   path = /srv/video\n   vfs objects = streams_xattr\n   ea support = yes\n   min protocol = SMB3\n";
    let out = add_time_machine_globals(conf).unwrap();
    let global = &out[..out.find("[protect]").unwrap()];
    for setting in [
        "   min protocol = SMB2",
        "   ea support = yes",
        "   vfs objects = catia fruit streams_xattr",
    ] {
        assert!(global.contains(setting), "missing {setting} in [global]:\n{out}");
    }
}

#[test]
fn time_machine_globals_are_recognised_however_they_are_spelled() {
    let conf = "[global]\n   Min Protocol = SMB3\n   EA support=yes\n   vfs  objects = fruit\n";
    assert_eq!(add_time_machine_globals(conf), None);
}

#[test]
fn nothing_changes_when_all_time_machine_globals_exist() {
    let conf =
        "[global]\n   min protocol = SMB2\n   ea support = yes\n   vfs objects = catia fruit streams_xattr\n";
    assert_eq!(add_time_machine_globals(conf), None);
}

#[test]
fn time_machine_globals_are_added_when_global_is_the_only_section() {
    let out = add_time_machine_globals("[global]\n   workgroup = W\n").unwrap();
    assert!(out.contains("   min protocol = SMB2"));
}

// --- Manager: list / get ----------------------------------------------------

#[test]
fn list_is_empty_when_the_config_is_missing() {
    let rig = Rig::build(MockFs::default(), MockExec::default(), MockPlatform::default());
    assert_eq!(rig.manager.list().unwrap(), []);
}

#[test]
fn get_reports_a_missing_share() {
    let rig = Rig::linux(&[]);
    assert_eq!(rig.manager.get("docs").unwrap().path, "/srv/docs");
    assert_eq!(rig.manager.get("nope").unwrap_err(), "share 'nope' not found");
}

// --- Manager: create --------------------------------------------------------

#[test]
fn create_appends_the_share_validates_and_reloads() {
    let rig = Rig::linux(&["/srv/media"]);
    let new = Share {
        comment: "Media".into(),
        valid_users: users(&["alice", "bob"]),
        ..share("media", "/srv/media")
    };

    assert_eq!(rig.manager.create(&new).unwrap(), Vec::<String>::new());

    assert!(rig.conf().starts_with(BASE));
    assert!(rig.conf().ends_with(&render_share(&new)));
    assert_eq!(
        rig.calls(),
        ["testparm -s /etc/samba/smb.conf", "systemctl reload smbd"]
    );
    assert_eq!(rig.fs.file(BACKUP).unwrap(), BASE);
}

#[test]
fn create_passes_on_the_service_warning() {
    let platform = MockPlatform {
        warning: Some("samba is not installed.".into()),
        ..Default::default()
    };
    let rig = Rig::build(fs_with(BASE, &["/srv/x"]), MockExec::default(), platform);
    assert_eq!(
        rig.manager.create(&share("x", "/srv/x")).unwrap(),
        ["samba is not installed."]
    );
}

#[test]
fn create_rejects_invalid_duplicate_and_missing() {
    let rig = Rig::linux(&["/srv/docs"]);
    assert!(rig
        .manager
        .create(&share("bad name", "/srv/docs"))
        .unwrap_err()
        .starts_with("invalid share name: "));
    assert!(rig
        .manager
        .create(&share("x", "/etc"))
        .unwrap_err()
        .starts_with("invalid path: "));
    assert_eq!(
        rig.manager.create(&share("docs", "/srv/docs")).unwrap_err(),
        "share 'docs' already exists"
    );
    assert_eq!(
        rig.manager.create(&share("x", "/srv/missing")).unwrap_err(),
        "path '/srv/missing' does not exist"
    );
    assert_eq!(rig.conf(), BASE);
    assert!(rig.calls().is_empty());
}

#[test]
fn create_restores_the_config_when_testparm_rejects_it() {
    let exec = MockExec {
        failing: vec!["testparm"],
        ..Default::default()
    };
    let rig = Rig::build(fs_with(BASE, &["/srv/x"]), exec, MockPlatform::default());

    let err = rig.manager.create(&share("x", "/srv/x")).unwrap_err();

    assert!(err.starts_with("invalid configuration: "), "{err}");
    assert_eq!(rig.conf(), BASE);
    assert_eq!(rig.calls(), ["testparm -s /etc/samba/smb.conf"]);
}

#[test]
fn create_time_machine_share_sets_globals_ownership_and_advertises() {
    let rig = Rig::linux(&["/srv/tm"]);
    let tm = Share {
        share_type: TIME_MACHINE.into(),
        valid_users: users(&["alice"]),
        ..share("tm", "/srv/tm")
    };

    rig.manager.create(&tm).unwrap();

    let conf = rig.conf();
    assert!(conf.find("   min protocol = SMB2").unwrap() < conf.find("[docs]").unwrap());
    assert!(conf.contains("[tm]\n   path = /srv/tm\n"));
    assert!(conf.contains("   fruit:time machine = yes\n"));
    assert_eq!(*rig.fs.chowns.lock().unwrap(), [("/srv/tm".to_string(), 1001)]);
    assert_eq!(*rig.fs.chmods.lock().unwrap(), [("/srv/tm".to_string(), 0o700)]);
    assert_eq!(rig.avahi_calls(), ["add tm [\"tm\"]"]);
}

#[test]
fn create_time_machine_share_needs_an_owner() {
    let rig = Rig::linux(&["/srv/tm"]);
    let tm = Share {
        share_type: TIME_MACHINE.into(),
        ..share("tm", "/srv/tm")
    };
    let err = rig.manager.create(&tm).unwrap_err();
    assert_eq!(
        err,
        "failed to set share permissions: Time Machine share has no valid users to assign ownership"
    );
    assert!(!rig.conf().contains("[tm]"));
}

// --- Manager: remove --------------------------------------------------------

#[test]
fn remove_deletes_the_section_validates_and_reloads() {
    let rig = Rig::linux(&[]);
    rig.manager.remove("docs").unwrap();
    assert!(!rig.conf().contains("[docs]"));
    assert!(rig.conf().contains("[global]"));
    assert_eq!(
        rig.calls(),
        ["testparm -s /etc/samba/smb.conf", "systemctl reload smbd"]
    );
    assert!(rig.avahi_calls().is_empty());
}

#[test]
fn remove_reports_a_missing_share() {
    let rig = Rig::linux(&[]);
    assert_eq!(rig.manager.remove("nope").unwrap_err(), "share 'nope' not found");
    assert_eq!(rig.conf(), BASE);
}

#[test]
fn remove_restores_the_config_when_testparm_rejects_it() {
    let exec = MockExec {
        failing: vec!["testparm"],
        ..Default::default()
    };
    let rig = Rig::build(fs_with(BASE, &[]), exec, MockPlatform::default());
    assert!(rig.manager.remove("docs").is_err());
    assert_eq!(rig.conf(), BASE);
}

#[test]
fn removing_a_time_machine_share_updates_avahi_with_the_remaining_ones() {
    let conf = "[global]\n[tm1]\n   path = /a\n   fruit:time machine = yes\n[tm2]\n   path = /b\n   fruit:time machine = yes\n";
    let rig = Rig::build(fs_with(conf, &[]), MockExec::default(), MockPlatform::default());
    rig.manager.remove("tm1").unwrap();
    assert_eq!(rig.avahi_calls(), ["remove tm1 [\"tm2\"]"]);
}

// --- Manager: modify --------------------------------------------------------

#[test]
fn modify_changes_only_the_given_fields() {
    let rig = Rig::linux(&[]);
    let update = ShareUpdate {
        comment: Some("New".into()),
        read_only: Some(true),
        ..Default::default()
    };

    rig.manager.modify("docs", update).unwrap();

    let docs = rig.manager.get("docs").unwrap();
    assert_eq!(docs.comment, "New");
    assert!(docs.read_only);
    assert!(docs.browseable);
    assert_eq!(docs.path, "/srv/docs");
    assert_eq!(docs.valid_users, ["alice", "bob"]);
}

#[test]
fn modify_validates_a_temp_file_then_swaps_it_in() {
    let rig = Rig::linux(&[]);
    rig.manager
        .modify(
            "docs",
            ShareUpdate {
                browseable: Some(false),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(
        rig.calls(),
        ["testparm -s /etc/samba/smb.conf.tmp", "systemctl reload smbd"]
    );
    assert_eq!(rig.fs.file("/etc/samba/smb.conf.tmp"), None);
    assert!(rig.conf().contains("   browseable = no\n"));
    assert_eq!(rig.conf().matches("[docs]").count(), 1);
}

#[test]
fn modify_leaves_the_config_alone_when_testparm_rejects_it() {
    let exec = MockExec {
        failing: vec!["testparm"],
        ..Default::default()
    };
    let rig = Rig::build(fs_with(BASE, &[]), exec, MockPlatform::default());
    let err = rig
        .manager
        .modify(
            "docs",
            ShareUpdate {
                read_only: Some(true),
                ..Default::default()
            },
        )
        .unwrap_err();
    assert!(err.starts_with("invalid configuration: "));
    assert_eq!(rig.conf(), BASE);
    assert_eq!(rig.fs.file("/etc/samba/smb.conf.tmp"), None);
}

#[test]
fn modify_can_clear_valid_users() {
    let rig = Rig::linux(&[]);
    rig.manager
        .modify(
            "docs",
            ShareUpdate {
                valid_users: Some(vec![]),
                ..Default::default()
            },
        )
        .unwrap();
    assert!(rig.manager.get("docs").unwrap().valid_users.is_empty());
}

#[test]
fn modify_keeps_a_preset_shares_tuning() {
    let media = Share {
        share_type: MEDIA.into(),
        ..share("movies", "/srv/movies")
    };
    let conf = format!("[global]\n{}", render_share(&media));
    let rig = Rig::build(fs_with(&conf, &[]), MockExec::default(), MockPlatform::default());

    rig.manager
        .modify(
            "movies",
            ShareUpdate {
                comment: Some("Films".into()),
                ..Default::default()
            },
        )
        .unwrap();

    assert!(rig.conf().contains("   comment = Films\n"));
    assert!(rig.conf().contains("   use sendfile = yes\n"));
}

#[test]
fn modify_toggling_time_machine_updates_avahi() {
    let rig = Rig::linux(&["/srv/docs"]);
    rig.manager
        .modify(
            "docs",
            ShareUpdate {
                time_machine: Some(true),
                ..Default::default()
            },
        )
        .unwrap();
    assert!(rig.conf().contains("   fruit:time machine = yes\n"));
    assert_eq!(rig.avahi_calls(), ["add docs [\"docs\"]"]);

    rig.manager
        .modify(
            "docs",
            ShareUpdate {
                time_machine: Some(false),
                ..Default::default()
            },
        )
        .unwrap();
    assert!(!rig.conf().contains("fruit:time machine"));
    assert_eq!(rig.avahi_calls()[1], "remove docs []");
}

// --- Manager: reload / install / permissions --------------------------------

#[test]
fn linux_reload_falls_back_to_the_service_command() {
    let exec = MockExec {
        failing: vec!["systemctl"],
        ..Default::default()
    };
    let rig = Rig::build(fs_with(BASE, &[]), exec, MockPlatform::default());
    rig.manager.remove("docs").unwrap();
    assert_eq!(rig.calls()[1..], ["systemctl reload smbd", "service smbd reload"]);
}

#[test]
fn reload_failure_is_reported() {
    let exec = MockExec {
        failing: vec!["systemctl", "service"],
        ..Default::default()
    };
    let rig = Rig::build(fs_with(BASE, &[]), exec, MockPlatform::default());
    assert!(rig
        .manager
        .remove("docs")
        .unwrap_err()
        .starts_with("failed to reload samba: "));
}

#[test]
fn macos_reload_signals_smbd_then_falls_back_to_brew() {
    let platform = MockPlatform {
        macos: true,
        ..Default::default()
    };
    let rig = Rig::build(fs_with(BASE, &[]), MockExec::default(), platform);
    rig.manager.remove("docs").unwrap();
    assert_eq!(rig.calls()[1..], ["pkill -HUP smbd"]);

    let platform = MockPlatform {
        macos: true,
        ..Default::default()
    };
    let exec = MockExec {
        failing: vec!["pkill"],
        ..Default::default()
    };
    let rig = Rig::build(fs_with(BASE, &[]), exec, platform);
    rig.manager.remove("docs").unwrap();
    assert_eq!(
        rig.calls()[1..],
        ["pkill -HUP smbd", "brew services restart samba"]
    );
}

#[test]
fn check_installed_requires_smbd() {
    let rig = Rig::linux(&[]);
    assert!(rig
        .manager
        .check_installed()
        .unwrap_err()
        .starts_with("Samba is not installed."));
}

#[test]
fn check_installed_creates_a_basic_config_when_missing() {
    let exec = MockExec {
        available: vec!["smbd"],
        ..Default::default()
    };
    let rig = Rig::build(MockFs::default(), exec, MockPlatform::default());
    rig.manager.check_installed().unwrap();
    assert!(rig.conf().starts_with("[global]\n   workgroup = WORKGROUP\n"));
}

#[test]
fn unifi_share_is_group_writable_and_owned_by_the_first_user() {
    let rig = Rig::linux(&["/srv/p"]);
    let s = Share {
        share_type: UNIFI_PROTECT.into(),
        valid_users: users(&["bob"]),
        ..share("p", "/srv/p")
    };
    rig.manager.fix_permissions(&s).unwrap();
    assert_eq!(*rig.fs.chmods.lock().unwrap(), [("/srv/p".to_string(), 0o770)]);
    assert_eq!(*rig.fs.chowns.lock().unwrap(), [("/srv/p".to_string(), 1002)]);
}

#[test]
fn general_and_media_shares_keep_their_permissions() {
    let rig = Rig::linux(&["/srv/p"]);
    rig.manager.fix_permissions(&share("p", "/srv/p")).unwrap();
    rig.manager
        .fix_permissions(&Share {
            share_type: MEDIA.into(),
            ..share("p", "/srv/p")
        })
        .unwrap();
    assert!(rig.fs.chmods.lock().unwrap().is_empty());
    assert!(rig.fs.chowns.lock().unwrap().is_empty());
}

#[test]
fn time_machine_permissions_fail_for_an_unknown_owner() {
    let rig = Rig::linux(&["/srv/tm"]);
    let s = Share {
        time_machine: true,
        valid_users: users(&["ghost"]),
        ..share("tm", "/srv/tm")
    };
    assert_eq!(
        rig.manager.fix_permissions(&s).unwrap_err(),
        "failed to resolve UID for user 'ghost': user not found"
    );
}

#[test]
fn turning_a_share_into_a_time_machine_share_prepares_it_like_create_does() {
    let rig = Rig::linux(&["/srv/docs"]);
    let update = ShareUpdate {
        time_machine: Some(true),
        ..Default::default()
    };

    rig.manager.modify("docs", update).unwrap();

    let conf = rig.conf();
    assert!(conf.find("   min protocol = SMB2").unwrap() < conf.find("[docs]").unwrap());
    assert!(conf.contains("   fruit:time machine = yes\n"));
    assert_eq!(*rig.fs.chowns.lock().unwrap(), [("/srv/docs".to_string(), 1001)]);
    assert_eq!(*rig.fs.chmods.lock().unwrap(), [("/srv/docs".to_string(), 0o700)]);
}

// --- Editing a share in place ------------------------------------------------

const RESTRICTED: &str = "[global]
   workgroup = WORKGROUP

[docs]
   path = /srv/docs
   # staff only
   Hosts Allow = 10.0.0.0/8
   writable = no
   guest ok = no
   valid users = alice bob

[other]
   path = /srv/other
";

fn restricted_rig() -> Rig {
    Rig::build(
        fs_with(RESTRICTED, &["/srv/docs"]),
        MockExec::default(),
        platform_with_users(),
    )
}

#[test]
fn modify_keeps_settings_sambo_does_not_manage() {
    let rig = restricted_rig();
    rig.manager
        .modify(
            "docs",
            ShareUpdate {
                comment: Some("Docs".into()),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(
        rig.conf(),
        RESTRICTED.replace(
            "   valid users = alice bob\n",
            "   valid users = alice bob\n   comment = Docs\n"
        )
    );
}

#[test]
fn modify_with_unchanged_values_leaves_the_file_as_it_was() {
    let rig = restricted_rig();
    let docs = rig.manager.get("docs").unwrap();
    rig.manager
        .modify(
            "docs",
            ShareUpdate {
                comment: Some(docs.comment),
                read_only: Some(docs.read_only),
                browseable: Some(docs.browseable),
                valid_users: Some(docs.valid_users),
                time_machine: Some(docs.time_machine),
            },
        )
        .unwrap();
    assert_eq!(rig.conf(), RESTRICTED);
}

#[test]
fn writable_no_is_read_as_read_only() {
    let docs = &parse_shares(RESTRICTED)[0];
    assert!(docs.read_only);
}

#[test]
fn keys_are_matched_ignoring_case_and_spaces() {
    let shares = parse_shares("[a]\n   Path = /srv/a\n   ReadOnly = True\n   Valid Users = ann, bob\n");
    assert_eq!(shares[0].path, "/srv/a");
    assert!(shares[0].read_only);
    assert_eq!(shares[0].valid_users, ["ann", "bob"]);
}

#[test]
fn making_a_share_writable_replaces_the_writable_synonym() {
    let rig = restricted_rig();
    rig.manager
        .modify(
            "docs",
            ShareUpdate {
                read_only: Some(false),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(
        rig.conf(),
        RESTRICTED.replace("   writable = no\n", "   read only = no\n")
    );
}

#[test]
fn clearing_a_value_removes_its_line_only() {
    let rig = restricted_rig();
    rig.manager
        .modify(
            "docs",
            ShareUpdate {
                valid_users: Some(vec![]),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(rig.conf(), RESTRICTED.replace("   valid users = alice bob\n", ""));
}

#[test]
fn time_machine_can_be_switched_on_and_off_without_losing_other_settings() {
    let rig = restricted_rig();
    let toggle = |on| ShareUpdate {
        time_machine: Some(on),
        ..Default::default()
    };
    rig.manager.modify("docs", toggle(true)).unwrap();
    let conf = rig.conf();
    assert!(conf.contains("   fruit:time machine = yes\n"));
    assert!(conf.contains("   Hosts Allow = 10.0.0.0/8\n"));
    assert!(conf.contains("[other]\n   path = /srv/other\n"));

    // Only the [global] section keeps what Time Machine needed.
    rig.manager.modify("docs", toggle(false)).unwrap();
    let docs = &RESTRICTED[RESTRICTED.find("[docs]").unwrap()..];
    assert!(rig.conf().ends_with(docs), "{}", rig.conf());
}

#[test]
fn switching_time_machine_off_restores_the_presets_own_settings() {
    let protect = Share {
        share_type: UNIFI_PROTECT.into(),
        valid_users: vec!["alice".into()],
        ..share("cams", "/srv/cams")
    };
    let conf = format!("[global]\n{}", render_share(&protect));
    let rig = Rig::build(
        fs_with(&conf, &["/srv/cams"]),
        MockExec::default(),
        platform_with_users(),
    );
    for on in [true, false] {
        rig.manager
            .modify(
                "cams",
                ShareUpdate {
                    time_machine: Some(on),
                    ..Default::default()
                },
            )
            .unwrap();
    }
    // [global] keeps the Time Machine settings; the share goes back to its own.
    let conf = rig.conf();
    let cams = &conf[conf.find("[cams]").unwrap()..];
    assert!(cams.contains("   vfs objects = streams_xattr\n"), "{cams}");
    assert!(!cams.contains("fruit"), "{cams}");
}
