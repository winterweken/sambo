//! Draws the app. Rendering only reads state; the one thing it writes back is
//! each table's scroll offset.

use ratatui::layout::{Constraint, Layout, Rect};
use ratatui::style::Style;
use ratatui::text::{Line, Span};
use ratatui::widgets::{
    Block, BorderType, Cell, Clear, Padding, Paragraph, Row, Table, TableState, Tabs, Wrap,
};
use ratatui::Frame;

use super::app::{forms, App, Confirm, Level, Modal, Purpose, ScanView, Tab};
use super::form::{Field, Form, Kind};
use super::input::Input;
use super::theme::Theme;
use crate::samba;
use crate::service::Status;

const MIN_WIDTH: u16 = 50;
const MIN_HEIGHT: u16 = 12;
/// Below this height the details pane gives its space to the table.
const DETAILS_MIN_HEIGHT: u16 = 16;
const SPINNER: [&str; 10] = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];

pub fn render(frame: &mut Frame, app: &mut App) {
    let area = frame.area();
    if area.width < MIN_WIDTH || area.height < MIN_HEIGHT {
        let message = format!("Terminal too small (need {MIN_WIDTH}x{MIN_HEIGHT})");
        frame.render_widget(Paragraph::new(message).wrap(Wrap { trim: true }), area);
        return;
    }

    let status = status_line(app);
    let status_height = wrapped_height(&status.to_string(), area.width).min(3);
    let [header, body, status_area, keys] = Layout::vertical([
        Constraint::Length(1),
        Constraint::Min(0),
        Constraint::Length(status_height),
        Constraint::Length(1),
    ])
    .areas(area);

    render_header(frame, app, header);
    render_body(frame, app, body);
    frame.render_widget(Paragraph::new(status).wrap(Wrap { trim: true }), status_area);
    frame.render_widget(key_hints(&app.theme, &tab_keys(app.tab)), keys);

    match &app.modal {
        Some(Modal::Form(purpose, form)) => render_form(frame, app, purpose, form, area),
        Some(Modal::Confirm(confirm)) => render_confirm(frame, &app.theme, confirm, area),
        Some(Modal::Scan(view)) => render_scan(frame, app, view, area),
        Some(Modal::Help) => render_help(frame, &app.theme, area),
        None => {}
    }
}

fn spinner(app: &App) -> &'static str {
    SPINNER[app.tick % SPINNER.len()]
}

/// How many rows `text` takes when wrapped to `width`; a little generous,
/// since word wrapping can break earlier than the width.
fn wrapped_height(text: &str, width: u16) -> u16 {
    let width = usize::from(width.max(1));
    let rows: usize = text
        .lines()
        .map(|line| Line::raw(line).width().div_ceil(width).max(1))
        .sum();
    rows.max(1).try_into().unwrap_or(u16::MAX)
}

// ── main screen ─────────────────────────────────────────────────────────

fn render_header(frame: &mut Frame, app: &App, area: Rect) {
    let [brand, tabs] = Layout::horizontal([Constraint::Length(7), Constraint::Min(0)]).areas(area);
    frame.render_widget(Span::styled(" sambo", app.theme.title), brand);
    let titles = Tab::ALL
        .iter()
        .map(|tab| format!("{} {}", tab.index() + 1, tab.title()));
    let tabs_widget = Tabs::new(titles)
        .select(app.tab.index())
        .style(app.theme.muted)
        .highlight_style(app.theme.selected)
        .divider("│");
    frame.render_widget(tabs_widget, tabs);
}

fn status_line(app: &App) -> Line<'static> {
    let theme = &app.theme;
    if let Some(notice) = &app.notice {
        let (mark, style) = match notice.level {
            Level::Info => ("•", theme.accent),
            Level::Success => ("✓", theme.success),
            Level::Warning => ("!", theme.warning),
            Level::Error => ("✗", theme.error),
        };
        return Line::styled(format!(" {mark} {}", notice.text.replace('\n', " ")), style);
    }
    match &app.busy {
        Some(busy) => Line::styled(format!(" {} {}", spinner(app), busy.label), theme.accent),
        None => Line::default(),
    }
}

fn tab_keys(tab: Tab) -> Vec<(&'static str, &'static str)> {
    let mut keys = match tab {
        Tab::Shares | Tab::Exports => vec![("a", "add"), ("e", "edit"), ("d", "remove")],
        Tab::Mounts => {
            vec![
                ("c", "CIFS"),
                ("n", "NFS"),
                ("u", "unmount"),
                ("p", "persist"),
                ("s", "scan"),
            ]
        }
        Tab::Users => vec![("a", "add"), ("p", "password"), ("d", "remove")],
        Tab::System => vec![("i", "install")],
    };
    keys.extend([("r", "reload"), ("?", "help"), ("q", "quit")]);
    keys
}

fn key_hints(theme: &Theme, keys: &[(&'static str, &'static str)]) -> Line<'static> {
    let mut spans = vec![Span::raw(" ")];
    for (key, action) in keys {
        spans.push(Span::styled(*key, theme.key));
        spans.push(Span::styled(format!(" {action}  "), theme.muted));
    }
    Line::from(spans)
}

/// What a tab shows: a table and details of the selected row.
struct View {
    title: &'static str,
    columns: Vec<(&'static str, Constraint)>,
    rows: Vec<Vec<Cell<'static>>>,
    details: Vec<Line<'static>>,
    empty: &'static str,
}

fn text(value: &str) -> Cell<'static> {
    Cell::from(value.to_string())
}

fn flag(theme: &Theme, on: bool, yes: &'static str, no: &'static str) -> Cell<'static> {
    if on {
        Cell::from(yes).style(theme.success)
    } else {
        Cell::from(no).style(theme.muted)
    }
}

fn detail(theme: &Theme, label: &str, value: impl Into<String>) -> Line<'static> {
    Line::from(vec![
        Span::styled(format!("{label}: "), theme.muted),
        Span::raw(value.into()),
    ])
}

fn or_dash(value: &str) -> &str {
    if value.is_empty() {
        "—"
    } else {
        value
    }
}

fn share_type_label(share: &samba::Share) -> &'static str {
    let kind = share.effective_type();
    samba::SHARE_TYPES
        .iter()
        .find(|(value, _)| *value == kind)
        .map_or("Custom", |(_, label)| label)
}

fn view(app: &App) -> View {
    use Constraint::{Fill, Length};
    let theme = &app.theme;
    let row = app.selection();
    match app.tab {
        Tab::Shares => View {
            title: "Samba Shares",
            columns: vec![("Name", Fill(2)), ("Path", Fill(3)), ("Type", Length(16)), ("Access", Length(10))],
            rows: (app.data.shares.iter())
                .map(|share| {
                    vec![
                        text(&share.name),
                        text(&share.path),
                        text(share_type_label(share)),
                        flag(theme, !share.read_only, "read-write", "read-only"),
                    ]
                })
                .collect(),
            details: app.data.shares.get(row).map_or(vec![], |share| {
                let users = if share.valid_users.is_empty() { "everyone".into() } else { share.valid_users.join(", ") };
                let browse = if share.browseable { "yes" } else { "no" };
                vec![
                    detail(theme, "Comment", or_dash(&share.comment)),
                    detail(theme, "Valid users", users),
                    detail(theme, "Browseable", browse),
                ]
            }),
            empty: "No Samba shares configured yet.\n\nPress a to add the first one.",
        },
        Tab::Exports => View {
            title: "NFS Exports",
            columns: vec![("Path", Fill(3)), ("Clients", Fill(2)), ("Options", Fill(3))],
            rows: (app.data.exports.iter())
                .map(|export| vec![text(&export.path), text(&export.clients), text(&export.options)])
                .collect(),
            details: app.data.exports.get(row).map_or(vec![], |export| {
                vec![detail(theme, "Clients", export.clients.as_str()), detail(theme, "Options", export.options.as_str())]
            }),
            empty: "No NFS exports configured yet.\n\nPress a to add the first one.",
        },
        Tab::Mounts => View {
            title: "Network Mounts",
            columns: vec![
                ("Type", Length(5)),
                ("Source", Fill(3)),
                ("Mount point", Fill(3)),
                ("Active", Length(6)),
                ("Persistent", Length(10)),
            ],
            rows: (app.data.mounts.iter())
                .map(|mount| {
                    vec![
                        text(&mount.kind.as_str().to_uppercase()),
                        text(&mount.source),
                        text(&mount.mount_point),
                        flag(theme, mount.active, "yes", "no"),
                        flag(theme, mount.persistent, "yes", "no"),
                    ]
                })
                .collect(),
            details: app.data.mounts.get(row).map_or(vec![], |mount| {
                vec![detail(theme, "Source", mount.source.as_str()), detail(theme, "Options", or_dash(&mount.options))]
            }),
            empty: "No network mounts found.\n\nPress c to mount a CIFS/SMB share, n for NFS, or s to scan for NFS servers.",
        },
        Tab::Users => View {
            title: "Samba Users",
            columns: vec![("Username", Fill(1)), ("UID", Length(8)), ("Status", Length(10))],
            rows: (app.data.users.iter())
                .map(|user| {
                    vec![text(&user.username), text(&user.uid.to_string()), flag(theme, user.enabled, "enabled", "disabled")]
                })
                .collect(),
            details: vec![],
            empty: "No Samba users configured yet.\n\nPress a to add the first one.",
        },
        Tab::System => {
            let system = &app.data.system;
            let mut details = vec![match system.package_manager {
                Some(name) => detail(theme, "Package manager", name),
                None => Line::styled("No supported package manager found; install components manually.", theme.warning),
            }];
            details.extend(system.services.iter().map(|(name, status)| {
                let state = match status {
                    Status::Running => "running",
                    Status::Stopped => "stopped",
                    Status::NotInstalled => "not installed",
                };
                detail(theme, &format!("{name} service"), state)
            }));
            View {
                title: "System Dependencies",
                columns: vec![("Component", Fill(2)), ("Used for", Fill(4)), ("Status", Length(13))],
                rows: (system.components.iter())
                    .map(|component| {
                        vec![
                            text(component.name),
                            text(component_purpose(component.key)),
                            flag(theme, component.installed, "installed", "not installed"),
                        ]
                    })
                    .collect(),
                details,
                empty: "No information about this system.",
            }
        }
    }
}

fn component_purpose(key: &str) -> &'static str {
    match key {
        "samba" => "Sharing files via SMB/CIFS",
        "nfs" => "Sharing files via NFS",
        "cifs" => "Mounting SMB/CIFS shares",
        "nfs-client" => "Mounting NFS exports",
        "avahi" => "Time Machine discovery",
        _ => "",
    }
}

fn panel(theme: &Theme, title: String) -> Block<'static> {
    Block::bordered()
        .border_type(BorderType::Rounded)
        .border_style(theme.muted)
        .title(Span::styled(title, theme.title))
}

fn render_body(frame: &mut Frame, app: &mut App, area: Rect) {
    let index = app.tab.index();
    let theme = app.theme.clone();
    let view = view(app);
    let loading = app.loading[index];
    let title = match (loading, view.rows.len()) {
        (true, _) => format!(" {} {} ", view.title, spinner(app)),
        (false, 0) => format!(" {} ", view.title),
        (false, count) => format!(" {} ({count}) ", view.title),
    };
    let block = panel(&theme, title);

    let message = if let Some(error) = &app.errors[index] {
        Some((format!("{error}\n\nPress r to try again."), theme.error))
    } else if !view.rows.is_empty() {
        None
    } else if loading {
        Some(("Loading…".to_string(), theme.muted))
    } else {
        Some((view.empty.to_string(), theme.muted))
    };
    if let Some((message, style)) = message {
        let block = block.padding(Padding::new(2, 2, 1, 0));
        let paragraph = Paragraph::new(message)
            .style(style)
            .wrap(Wrap { trim: true })
            .block(block);
        frame.render_widget(paragraph, area);
        return;
    }

    let details_height = match view.details.len() {
        lines if lines > 0 && area.height >= DETAILS_MIN_HEIGHT => lines as u16 + 2,
        _ => 0,
    };
    let [table_area, details_area] =
        Layout::vertical([Constraint::Min(0), Constraint::Length(details_height)]).areas(area);

    let header = Row::new(view.columns.iter().map(|(name, _)| *name)).style(theme.header);
    let table = Table::new(
        view.rows.into_iter().map(Row::new),
        view.columns.iter().map(|(_, width)| *width),
    )
    .header(header)
    .block(block)
    .column_spacing(2)
    .row_highlight_style(theme.selected)
    .highlight_symbol("› ");
    let mut state = TableState::default()
        .with_offset(app.offsets[index])
        .with_selected(Some(app.selected[index]));
    frame.render_stateful_widget(table, table_area, &mut state);
    app.offsets[index] = state.offset();

    if details_height > 0 {
        let details = Paragraph::new(view.details)
            .block(panel(&theme, " Details ".into()).padding(Padding::horizontal(1)));
        frame.render_widget(details, details_area);
    }
}

// ── dialogs ─────────────────────────────────────────────────────────────

/// Clears a centred box of at most the given size and returns its inside.
fn dialog(frame: &mut Frame, theme: &Theme, area: Rect, title: &str, width: u16, height: u16) -> Rect {
    let width = width.min(area.width.saturating_sub(4));
    let height = height.min(area.height);
    let rect = Rect {
        x: area.x + (area.width - width) / 2,
        y: area.y + (area.height - height) / 2,
        width,
        height,
    };
    let block = Block::bordered()
        .border_type(BorderType::Rounded)
        .border_style(theme.accent)
        .title(Span::styled(format!(" {title} "), theme.title))
        .padding(Padding::horizontal(1));
    let inner = block.inner(rect);
    frame.render_widget(Clear, rect);
    frame.render_widget(block, rect);
    inner
}

/// The part of an input that fits in `width` cells while keeping the cursor
/// in view, and the cursor's column within it.
fn window(input: &Input, width: u16) -> (String, u16) {
    let chars: Vec<char> = input.display().chars().collect();
    let cursor = input.cursor().min(chars.len());
    let width = usize::from(width.max(1));
    let start = (cursor + 1).saturating_sub(width);
    let end = (start + width).min(chars.len());
    let before: String = chars[start..cursor].iter().collect();
    (
        chars[start..end].iter().collect(),
        Span::raw(before).width() as u16,
    )
}

fn field_value(theme: &Theme, field: &Field, focused: bool, width: u16) -> (Span<'static>, Option<u16>) {
    match &field.kind {
        Kind::Text(input) => {
            let (visible, cursor) = window(input, width);
            (Span::raw(visible), focused.then_some(cursor))
        }
        Kind::Checkbox(checked) => (Span::raw(if *checked { "[x]" } else { "[ ]" }), None),
        Kind::Choice { options, selected } => {
            let label = &options[*selected].1;
            (
                Span::raw(if focused {
                    format!("‹ {label} ›")
                } else {
                    label.clone()
                }),
                None,
            )
        }
        Kind::ReadOnly(value) => (Span::styled(value.clone(), theme.muted), None),
    }
}

fn render_form(frame: &mut Frame, app: &App, purpose: &Purpose, form: &Form, area: Rect) {
    let theme = &app.theme;
    let width = 72.min(area.width.saturating_sub(4));
    let text_width = width.saturating_sub(4);
    let label_width = form
        .fields
        .iter()
        .map(|field| field.label.len())
        .max()
        .unwrap_or(0)
        + 2;
    let value_x = 2 + label_width as u16;
    let value_width = text_width.saturating_sub(value_x);

    let mut footer: Vec<Line> = Vec::new();
    let mut footer_height = 1;
    if let Some(error) = &form.error {
        let error = error.replace('\n', " ");
        footer_height += wrapped_height(&error, text_width).min(3);
        footer.push(Line::styled(error, theme.error));
    }
    let hint = forms::hint(purpose);
    if !hint.is_empty() && form.error.is_none() {
        footer_height += 1;
        footer.push(Line::styled(hint, theme.muted));
    }
    let keys = [
        ("Tab", "move"),
        ("Space", "toggle"),
        ("←/→", "choose"),
        ("Ctrl+S", "submit"),
        ("Esc", "cancel"),
    ];

    let body_height = form.fields.len() as u16 + 2;
    let inner = dialog(
        frame,
        theme,
        area,
        &form.title,
        width,
        body_height + footer_height + 3,
    );
    let [fields_area, _, footer_area, keys_area] = Layout::vertical([
        Constraint::Min(1),
        Constraint::Length(1),
        Constraint::Length(footer_height - 1),
        Constraint::Length(1),
    ])
    .areas(inner);

    let busy = app.busy.is_some();
    let mut cursor = None;
    let mut lines: Vec<Line> = Vec::new();
    for (index, field) in form.fields.iter().enumerate() {
        let focused = index == form.focus() && !busy;
        let (value, cursor_x) = field_value(theme, field, focused, value_width);
        cursor = cursor.or(cursor_x);
        let label = format!(
            "{}{:<label_width$}",
            if focused { "› " } else { "  " },
            field.label
        );
        lines.push(Line::from(vec![
            Span::styled(label, if focused { theme.title } else { Style::new() }),
            value,
        ]));
    }
    lines.push(Line::default());
    lines.push(match &app.busy {
        Some(busy) => Line::styled(format!("  {} {}", spinner(app), busy.label), theme.accent),
        None => {
            let style = if form.on_submit_button() {
                theme.selected
            } else {
                theme.accent
            };
            Line::from(vec![
                Span::raw("  "),
                Span::styled(format!("[ {} ]", form.submit_label), style),
            ])
        }
    });

    // Scroll just enough to keep the focused row inside a short dialog.
    let focus_row = if form.on_submit_button() {
        form.fields.len() + 1
    } else {
        form.focus()
    } as u16;
    let scroll = (focus_row + 1).saturating_sub(fields_area.height);
    frame.render_widget(Paragraph::new(lines).scroll((scroll, 0)), fields_area);
    frame.render_widget(Paragraph::new(footer).wrap(Wrap { trim: true }), footer_area);
    frame.render_widget(key_hints(theme, &keys), keys_area);
    if let Some(x) = cursor {
        frame.set_cursor_position((fields_area.x + value_x + x, fields_area.y + focus_row - scroll));
    }
}

fn render_confirm(frame: &mut Frame, theme: &Theme, confirm: &Confirm, area: Rect) {
    let width = 60.min(area.width.saturating_sub(4));
    let mut lines = vec![Line::raw(confirm.message.clone()), Line::default()];
    let mut keys = vec![("y", "confirm"), ("n", "cancel")];
    if let Some((label, on)) = confirm.toggle {
        lines.push(Line::raw(format!("[{}] {label}", if on { 'x' } else { ' ' })));
        lines.push(Line::default());
        keys.push(("Space", "toggle"));
    }
    lines.push(key_hints(theme, &keys));
    let height = wrapped_height(&confirm.message, width.saturating_sub(4)) + lines.len() as u16 + 2;
    let inner = dialog(frame, theme, area, &confirm.title, width, height);
    frame.render_widget(Paragraph::new(lines).wrap(Wrap { trim: true }), inner);
}

fn render_scan(frame: &mut Frame, app: &App, view: &ScanView, area: Rect) {
    let theme = &app.theme;
    let title = "NFS Server Discovery";
    let close = key_hints(theme, &[("Esc", "close")]);
    match view {
        ScanView::Running => {
            let inner = dialog(frame, theme, area, title, 60, 5);
            let scanning = Line::styled(
                format!("{} Scanning the local network for NFS servers…", spinner(app)),
                theme.accent,
            );
            frame.render_widget(Paragraph::new(vec![scanning, Line::default(), close]), inner);
        }
        ScanView::Failed(error) => {
            let inner = dialog(frame, theme, area, title, 60, wrapped_height(error, 54) + 5);
            let lines = vec![
                Line::styled(error.replace('\n', " "), theme.error),
                Line::default(),
                close,
            ];
            frame.render_widget(Paragraph::new(lines).wrap(Wrap { trim: true }), inner);
        }
        ScanView::Results { subnet, rows, .. } if rows.is_empty() => {
            let inner = dialog(frame, theme, area, title, 60, 7);
            let lines = vec![
                Line::raw(format!("No NFS servers found on {subnet}.")),
                Line::styled("Make sure the servers are running and reachable.", theme.muted),
                Line::default(),
                close,
            ];
            frame.render_widget(Paragraph::new(lines).wrap(Wrap { trim: true }), inner);
        }
        ScanView::Results {
            subnet,
            rows,
            selected,
        } => {
            let inner = dialog(frame, theme, area, title, 80, rows.len() as u16 + 6);
            let [summary, table_area, keys] =
                Layout::vertical([Constraint::Length(1), Constraint::Min(1), Constraint::Length(1)])
                    .areas(inner);
            let found = format!("{} export(s) found on {subnet}", rows.len());
            frame.render_widget(Line::styled(found, theme.success), summary);
            let table = Table::new(
                rows.iter()
                    .map(|row| Row::new([row.server.clone(), row.path.clone(), row.clients.clone()])),
                [Constraint::Length(16), Constraint::Fill(3), Constraint::Fill(2)],
            )
            .header(Row::new(["Server", "Export", "Clients"]).style(theme.header))
            .column_spacing(2)
            .row_highlight_style(theme.selected)
            .highlight_symbol("› ");
            let mut state = TableState::default().with_selected(Some(*selected));
            frame.render_stateful_widget(table, table_area, &mut state);
            frame.render_widget(key_hints(theme, &[("Enter", "mount"), ("Esc", "close")]), keys);
        }
    }
}

fn render_help(frame: &mut Frame, theme: &Theme, area: Rect) {
    const KEYS: [(&str, &str); 9] = [
        ("Tab / ← →", "switch tab (or 1–5)"),
        ("↑ ↓ / j k", "move the selection"),
        ("Home / End", "first / last row"),
        ("r", "reload the current tab"),
        ("Enter", "edit or act on the selected row"),
        ("Esc", "close a dialog, clear the status line"),
        ("?", "this help"),
        ("q", "quit"),
        ("Ctrl+C", "quit from anywhere"),
    ];
    let inner = dialog(frame, theme, area, "Keys", 58, KEYS.len() as u16 + 5);
    let mut lines: Vec<Line> = (KEYS.iter())
        .map(|(key, action)| {
            Line::from(vec![
                Span::styled(format!("{key:<12}"), theme.key),
                Span::raw(*action),
            ])
        })
        .collect();
    lines.push(Line::default());
    lines.push(Line::styled(
        "Each tab's own keys are listed at the bottom.",
        theme.muted,
    ));
    lines.push(Line::styled("Press any key to close.", theme.muted));
    frame.render_widget(Paragraph::new(lines).wrap(Wrap { trim: true }), inner);
}

#[cfg(test)]
mod tests;
