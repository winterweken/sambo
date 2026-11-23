//! UI state and the messages exchanged with the background worker.
//!
//! Nothing here touches the system: keys and worker responses go in, state
//! changes and worker requests come out, which keeps it all testable.

pub mod forms;
mod update;

use crate::mount::discover::NfsServer;
use crate::mount::Mount;
use crate::nfs::{Export, OptionChanges};
use crate::samba::{Share, ShareUpdate};
use crate::service;
use crate::user::User;
use crate::Result;

use super::form::Form;
use super::theme::Theme;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Tab {
    Shares,
    Exports,
    Mounts,
    Users,
    System,
}

impl Tab {
    pub const ALL: [Tab; 5] = [Tab::Shares, Tab::Exports, Tab::Mounts, Tab::Users, Tab::System];

    pub fn index(self) -> usize {
        self as usize
    }

    pub fn title(self) -> &'static str {
        match self {
            Tab::Shares => "Shares",
            Tab::Exports => "Exports",
            Tab::Mounts => "Mounts",
            Tab::Users => "Users",
            Tab::System => "System",
        }
    }
}

/// An installable dependency shown on the System tab.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Component {
    pub key: &'static str,
    pub name: &'static str,
    pub installed: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct SystemInfo {
    /// `None` when no supported package manager was found.
    pub package_manager: Option<&'static str>,
    pub components: Vec<Component>,
    pub services: Vec<(&'static str, service::Status)>,
}

#[derive(Debug, Clone, Default)]
pub struct Data {
    pub shares: Vec<Share>,
    pub exports: Vec<Export>,
    pub mounts: Vec<Mount>,
    pub users: Vec<User>,
    pub system: SystemInfo,
}

/// Work for the background worker.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Request {
    Load(Tab),
    CreateShare(Share),
    ModifyShare {
        name: String,
        update: ShareUpdate,
    },
    RemoveShare(String),
    CreateExport(Export),
    ModifyExport {
        path: String,
        clients: String,
        changes: OptionChanges,
    },
    RemoveExport(String),
    AddUser {
        username: String,
        password: String,
    },
    SetPassword {
        username: String,
        password: String,
    },
    RemoveUser(String),
    MountCifs {
        source: String,
        mount_point: String,
        username: String,
        password: String,
        persistent: bool,
    },
    MountNfs {
        source: String,
        mount_point: String,
        persistent: bool,
    },
    Unmount {
        mount_point: String,
        remove_persistent: bool,
    },
    SetPersistent {
        mount_point: String,
        persistent: bool,
    },
    Install(&'static str),
    Scan,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Loaded {
    Shares(Vec<Share>),
    Exports(Vec<Export>),
    Mounts(Vec<Mount>),
    Users(Vec<User>),
    System(SystemInfo),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ScanResult {
    pub subnet: String,
    pub servers: Vec<NfsServer>,
}

/// What the worker sends back.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Response {
    Loaded(Tab, Result<Loaded>),
    /// The outcome of a change; success carries any warnings.
    Done(Result<Vec<String>>),
    Scanned(Result<ScanResult>),
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Level {
    Info,
    Success,
    Warning,
    Error,
}

/// The message shown on the status line.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Notice {
    pub level: Level,
    pub text: String,
}

/// A change that is running in the background.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Busy {
    pub label: String,
    pub success: String,
    pub reload: Vec<Tab>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Purpose {
    CreateShare,
    ModifyShare(String),
    CreateExport,
    /// The export as it was when the form opened.
    ModifyExport(Export),
    AddUser,
    SetPassword(String),
    MountCifs,
    MountNfs,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Action {
    RemoveShare(String),
    RemoveExport(String),
    RemoveUser(String),
    Unmount(String),
    Install(&'static str),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Confirm {
    pub title: String,
    pub message: String,
    /// An optional yes/no option, toggled with space.
    pub toggle: Option<(&'static str, bool)>,
    pub action: Action,
}

/// One export found by a network scan.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ScanRow {
    pub server: String,
    pub path: String,
    pub clients: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ScanView {
    Running,
    Failed(String),
    Results {
        subnet: String,
        rows: Vec<ScanRow>,
        selected: usize,
    },
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Modal {
    Form(Purpose, Form),
    Confirm(Confirm),
    Scan(ScanView),
    Help,
}

pub struct App {
    pub theme: Theme,
    pub tab: Tab,
    pub data: Data,
    /// Selected row and scroll offset of each tab's table, by `Tab::index`.
    pub selected: [usize; 5],
    pub offsets: [usize; 5],
    pub loading: [bool; 5],
    pub errors: [Option<String>; 5],
    pub modal: Option<Modal>,
    pub busy: Option<Busy>,
    pub notice: Option<Notice>,
    pub should_quit: bool,
    /// Advances while idle; drives the spinner.
    pub tick: usize,
    requests: Vec<Request>,
    quit_armed: bool,
}

#[cfg(test)]
mod tests;
