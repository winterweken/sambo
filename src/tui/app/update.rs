//! How the state reacts to keys and to the worker's responses.

use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};

use super::forms::{self, Job};
use super::*;
use crate::tui::form::Outcome;

const PAGE: usize = 10;

impl App {
    /// Starts with every tab loading.
    pub fn new(theme: Theme) -> App {
        App {
            theme,
            tab: Tab::Shares,
            data: Data::default(),
            selected: [0; 5],
            offsets: [0; 5],
            loading: [true; 5],
            errors: Default::default(),
            modal: None,
            busy: None,
            notice: None,
            should_quit: false,
            tick: 0,
            requests: Tab::ALL.map(Request::Load).to_vec(),
            quit_armed: false,
        }
    }

    /// Hands over the requests made since the last call.
    pub fn take_requests(&mut self) -> Vec<Request> {
        std::mem::take(&mut self.requests)
    }

    /// What identifies each row of a tab, in display order.
    fn keys(&self, tab: Tab) -> Vec<&str> {
        match tab {
            Tab::Shares => self.data.shares.iter().map(|s| s.name.as_str()).collect(),
            Tab::Exports => self.data.exports.iter().map(|e| e.path.as_str()).collect(),
            Tab::Mounts => self.data.mounts.iter().map(|m| m.mount_point.as_str()).collect(),
            Tab::Users => self.data.users.iter().map(|u| u.username.as_str()).collect(),
            Tab::System => self.data.system.components.iter().map(|c| c.key).collect(),
        }
    }

    pub fn rows(&self, tab: Tab) -> usize {
        self.keys(tab).len()
    }

    /// The selected row of the current tab.
    pub fn selection(&self) -> usize {
        self.selected[self.tab.index()]
    }

    fn notify(&mut self, level: Level, text: impl Into<String>) {
        self.notice = Some(Notice {
            level,
            text: text.into(),
        });
    }

    fn reload(&mut self, tab: Tab) {
        self.loading[tab.index()] = true;
        self.requests.push(Request::Load(tab));
    }

    fn start(&mut self, job: Job) {
        self.requests.push(job.request);
        self.busy = Some(Busy {
            label: job.label,
            success: job.success,
            reload: job.reload,
        });
        self.notice = None;
        self.quit_armed = false;
    }

    /// Quits, unless a change is still running and this is the first request.
    fn quit(&mut self) {
        if self.busy.is_some() && !self.quit_armed {
            self.quit_armed = true;
            self.notify(
                Level::Warning,
                "An operation is still running. Press Ctrl+C again to quit anyway.",
            );
        } else {
            self.should_quit = true;
        }
    }

    pub fn on_key(&mut self, key: KeyEvent) {
        if key.modifiers.contains(KeyModifiers::CONTROL) && key.code == KeyCode::Char('c') {
            return self.quit();
        }
        match self.modal.take() {
            Some(modal) => self.modal = self.modal_key(modal, key),
            None => self.main_key(key.code),
        }
    }

    /// Handles a key for the open dialog and returns the dialog to show next.
    fn modal_key(&mut self, modal: Modal, key: KeyEvent) -> Option<Modal> {
        match modal {
            // The form waits for the result of its own submission.
            Modal::Form(purpose, form) if self.busy.is_some() => Some(Modal::Form(purpose, form)),
            Modal::Form(purpose, mut form) => {
                match form.on_key(key) {
                    Outcome::Cancel => return None,
                    Outcome::Pending => {}
                    Outcome::Submit => match forms::submission(&purpose, &form) {
                        Ok(job) => self.start(job),
                        Err(error) => form.error = Some(error),
                    },
                }
                Some(Modal::Form(purpose, form))
            }
            Modal::Confirm(mut confirm) => match key.code {
                KeyCode::Char('y' | 'Y') => {
                    self.start(forms::confirmed(&confirm));
                    None
                }
                KeyCode::Char('n' | 'N' | 'q') | KeyCode::Esc => None,
                KeyCode::Char(' ') => {
                    if let Some((_, on)) = &mut confirm.toggle {
                        *on = !*on;
                    }
                    Some(Modal::Confirm(confirm))
                }
                _ => Some(Modal::Confirm(confirm)),
            },
            Modal::Scan(view) => match (key.code, view) {
                (KeyCode::Esc | KeyCode::Char('q'), _) => None,
                (
                    code,
                    ScanView::Results {
                        subnet,
                        rows,
                        selected,
                    },
                ) => {
                    let selected = match code {
                        KeyCode::Up | KeyCode::Char('k') => selected.saturating_sub(1),
                        KeyCode::Down | KeyCode::Char('j') => {
                            (selected + 1).min(rows.len().saturating_sub(1))
                        }
                        KeyCode::Enter => {
                            if let Some(row) = rows.get(selected) {
                                let source = format!("{}:{}", row.server, row.path);
                                return Some(Modal::Form(Purpose::MountNfs, forms::mount_nfs(&source)));
                            }
                            selected
                        }
                        _ => selected,
                    };
                    Some(Modal::Scan(ScanView::Results {
                        subnet,
                        rows,
                        selected,
                    }))
                }
                (_, view) => Some(Modal::Scan(view)),
            },
            Modal::Help => None,
        }
    }

    fn main_key(&mut self, code: KeyCode) {
        let tabs = Tab::ALL.len();
        let (index, last) = (self.tab.index(), self.rows(self.tab).saturating_sub(1));
        let row = &mut self.selected[index];
        match code {
            KeyCode::Char('q') => self.quit(),
            KeyCode::Esc => self.notice = None,
            KeyCode::Char('?') => self.modal = Some(Modal::Help),
            KeyCode::Tab | KeyCode::Right | KeyCode::Char('l') => self.tab = Tab::ALL[(index + 1) % tabs],
            KeyCode::BackTab | KeyCode::Left | KeyCode::Char('h') => {
                self.tab = Tab::ALL[(index + tabs - 1) % tabs]
            }
            KeyCode::Char(c @ '1'..='5') => self.tab = Tab::ALL[c as usize - '1' as usize],
            KeyCode::Up | KeyCode::Char('k') => *row = row.saturating_sub(1),
            KeyCode::Down | KeyCode::Char('j') => *row = (*row + 1).min(last),
            KeyCode::PageUp => *row = row.saturating_sub(PAGE),
            KeyCode::PageDown => *row = (*row + PAGE).min(last),
            KeyCode::Home | KeyCode::Char('g') => *row = 0,
            KeyCode::End | KeyCode::Char('G') => *row = last,
            KeyCode::Char('r') => self.reload(self.tab),
            // Only one change runs at a time.
            code if self.busy.is_none() => self.action_key(code),
            _ => {}
        }
    }

    /// The keys that open a dialog or change something on the current tab.
    fn action_key(&mut self, code: KeyCode) {
        let row = self.selection();
        let data = &self.data;
        let form = |purpose, form| Some(Modal::Form(purpose, form));
        let mut job = None;
        let modal = match (self.tab, code) {
            (Tab::Shares, KeyCode::Char('a')) => form(Purpose::CreateShare, forms::create_share()),
            (Tab::Shares, KeyCode::Char('e') | KeyCode::Enter) => data.shares.get(row).and_then(|share| {
                form(
                    Purpose::ModifyShare(share.name.clone()),
                    forms::modify_share(share),
                )
            }),
            (Tab::Shares, KeyCode::Char('d') | KeyCode::Delete) => data
                .shares
                .get(row)
                .map(|share| Modal::Confirm(forms::remove_share(share))),

            (Tab::Exports, KeyCode::Char('a')) => form(Purpose::CreateExport, forms::create_export()),
            (Tab::Exports, KeyCode::Char('e') | KeyCode::Enter) => data.exports.get(row).and_then(|export| {
                form(
                    Purpose::ModifyExport(export.clone()),
                    forms::modify_export(export),
                )
            }),
            (Tab::Exports, KeyCode::Char('d') | KeyCode::Delete) => data
                .exports
                .get(row)
                .map(|export| Modal::Confirm(forms::remove_export(export))),

            (Tab::Mounts, KeyCode::Char('c')) => form(Purpose::MountCifs, forms::mount_cifs()),
            (Tab::Mounts, KeyCode::Char('n')) => form(Purpose::MountNfs, forms::mount_nfs("")),
            (Tab::Mounts, KeyCode::Char('u' | 'd') | KeyCode::Delete) => data
                .mounts
                .get(row)
                .map(|mount| Modal::Confirm(forms::unmount(mount))),
            (Tab::Mounts, KeyCode::Char('p')) => {
                job = data.mounts.get(row).map(forms::set_persistent);
                None
            }
            (Tab::Mounts, KeyCode::Char('s')) => {
                self.requests.push(Request::Scan);
                Some(Modal::Scan(ScanView::Running))
            }

            (Tab::Users, KeyCode::Char('a')) => form(Purpose::AddUser, forms::add_user()),
            (Tab::Users, KeyCode::Char('p') | KeyCode::Enter) => data.users.get(row).and_then(|user| {
                form(
                    Purpose::SetPassword(user.username.clone()),
                    forms::set_password(&user.username),
                )
            }),
            (Tab::Users, KeyCode::Char('d') | KeyCode::Delete) => data
                .users
                .get(row)
                .map(|user| Modal::Confirm(forms::remove_user(&user.username))),

            (Tab::System, KeyCode::Char('i') | KeyCode::Enter) => return self.install_selected(),
            _ => return,
        };
        self.modal = modal;
        if let Some(job) = job {
            self.start(job);
        }
    }

    fn install_selected(&mut self) {
        let system = &self.data.system;
        let Some(component) = system.components.get(self.selection()) else {
            return;
        };
        if component.installed {
            let text = format!("{} is already installed", component.name);
            return self.notify(Level::Info, text);
        }
        match system.package_manager {
            Some(package_manager) => {
                self.modal = Some(Modal::Confirm(forms::install(component, package_manager)))
            }
            None => self.notify(
                Level::Error,
                "Cannot install: no supported package manager was found",
            ),
        }
    }

    pub fn on_response(&mut self, response: Response) {
        match response {
            Response::Loaded(tab, result) => self.on_loaded(tab, result),
            Response::Done(result) => self.on_done(result),
            Response::Scanned(result) => {
                // The results of a scan whose dialog was closed are of no use.
                if self.modal == Some(Modal::Scan(ScanView::Running)) {
                    self.modal = Some(Modal::Scan(match result {
                        Ok(scan) => ScanView::Results {
                            subnet: scan.subnet,
                            rows: scan_rows(scan.servers),
                            selected: 0,
                        },
                        Err(error) => ScanView::Failed(error),
                    }));
                }
            }
        }
    }

    fn on_loaded(&mut self, tab: Tab, result: Result<Loaded>) {
        let index = tab.index();
        // Keep the selection on the same item even if the list changed around it.
        let previous = self
            .keys(tab)
            .get(self.selected[index])
            .map(|key| key.to_string());
        self.loading[index] = false;
        let data = &mut self.data;
        match result {
            Ok(loaded) => {
                self.errors[index] = None;
                match loaded {
                    Loaded::Shares(shares) => data.shares = shares,
                    Loaded::Exports(exports) => data.exports = exports,
                    Loaded::Mounts(mounts) => data.mounts = mounts,
                    Loaded::Users(users) => data.users = users,
                    Loaded::System(system) => data.system = system,
                }
            }
            Err(error) => {
                self.errors[index] = Some(error);
                match tab {
                    Tab::Shares => data.shares.clear(),
                    Tab::Exports => data.exports.clear(),
                    Tab::Mounts => data.mounts.clear(),
                    Tab::Users => data.users.clear(),
                    Tab::System => data.system = SystemInfo::default(),
                }
            }
        }
        let keys = self.keys(tab);
        let found = previous.and_then(|key| keys.iter().position(|k| *k == key));
        self.selected[index] = found
            .unwrap_or(self.selected[index])
            .min(keys.len().saturating_sub(1));
    }

    fn on_done(&mut self, result: Result<Vec<String>>) {
        let Some(busy) = self.busy.take() else { return };
        self.quit_armed = false;
        match (result, &mut self.modal) {
            // The form stays open so the input can be corrected.
            (Err(error), Some(Modal::Form(_, form))) => form.error = Some(error),
            (Err(error), _) => self.notify(Level::Error, error),
            (Ok(warnings), _) => {
                self.modal = None;
                if warnings.is_empty() {
                    self.notify(Level::Success, busy.success);
                } else {
                    self.notify(
                        Level::Warning,
                        format!("{} — {}", busy.success, warnings.join("; ")),
                    );
                }
            }
        }
        // Reload even after a failure: the change may have been applied in part.
        busy.reload.into_iter().for_each(|tab| self.reload(tab));
    }
}

fn scan_rows(servers: Vec<NfsServer>) -> Vec<ScanRow> {
    let row = |export: crate::mount::discover::NfsExport| ScanRow {
        server: export.server,
        path: export.path,
        clients: if export.clients.is_empty() {
            "*".to_string()
        } else {
            export.clients.join(" ")
        },
    };
    servers
        .into_iter()
        .flat_map(|server| server.exports)
        .map(row)
        .collect()
}
