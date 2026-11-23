use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};
use ratatui::backend::TestBackend;
use ratatui::Terminal;

use super::*;
use crate::samba::Share;
use crate::tui::app::{Loaded, Response, Tab};
use crate::tui::theme::Theme;

fn draw(app: &mut App, width: u16, height: u16) -> String {
    let mut terminal = Terminal::new(TestBackend::new(width, height)).unwrap();
    terminal.draw(|frame| render(frame, app)).unwrap();
    let buffer = terminal.backend().buffer().clone();
    let mut screen = String::new();
    for y in 0..height {
        for x in 0..width {
            screen.push_str(buffer[(x, y)].symbol());
        }
        screen.push('\n');
    }
    screen
}

fn app() -> App {
    let mut app = App::new(Theme::plain());
    app.take_requests();
    let shares = (0..40)
        .map(|i| Share {
            name: format!("share{i:02}"),
            path: format!("/srv/{i}"),
            ..Share::default()
        })
        .collect();
    app.on_response(Response::Loaded(Tab::Shares, Ok(Loaded::Shares(shares))));
    app
}

fn press(app: &mut App, code: KeyCode) {
    app.on_key(KeyEvent::new(code, KeyModifiers::NONE));
}

#[test]
fn shows_the_tabs_and_the_loaded_shares() {
    let screen = draw(&mut app(), 100, 30);
    for expected in [
        "Shares", "Exports", "Mounts", "Users", "System", "share00", "/srv/0",
    ] {
        assert!(screen.contains(expected), "missing {expected:?} in\n{screen}");
    }
}

#[test]
fn says_so_when_a_tab_is_still_loading() {
    let mut app = app();
    press(&mut app, KeyCode::Tab);
    assert!(draw(&mut app, 100, 30).contains("Loading"));
}

#[test]
fn shows_a_load_error_instead_of_the_table() {
    let mut app = app();
    app.on_response(Response::Loaded(Tab::Shares, Err("smb.conf unreadable".into())));
    assert!(draw(&mut app, 100, 30).contains("smb.conf unreadable"));
}

#[test]
fn shows_how_to_add_the_first_item_when_a_list_is_empty() {
    let mut app = app();
    app.on_response(Response::Loaded(Tab::Shares, Ok(Loaded::Shares(vec![]))));
    assert!(draw(&mut app, 100, 30).contains("No Samba shares"));
}

#[test]
fn scrolls_to_keep_the_selection_visible() {
    let mut app = app();
    press(&mut app, KeyCode::End);
    let screen = draw(&mut app, 100, 20);
    assert!(screen.contains("share39"), "{screen}");
    assert!(!screen.contains("share00"));
}

#[test]
fn a_small_terminal_gets_a_message_instead_of_a_broken_layout() {
    let screen = draw(&mut app(), 30, 6);
    assert!(screen.contains("too small"), "{screen}");
}

#[test]
fn any_size_can_be_drawn() {
    let mut app = app();
    press(&mut app, KeyCode::Char('a'));
    for (width, height) in [(1, 1), (10, 3), (40, 10), (60, 14), (200, 60)] {
        draw(&mut app, width, height);
    }
}

#[test]
fn a_form_shows_its_fields_and_hides_passwords() {
    let mut app = app();
    app.tab = Tab::Users;
    app.on_response(Response::Loaded(Tab::Users, Ok(Loaded::Users(vec![]))));
    press(&mut app, KeyCode::Char('a'));
    for c in "carol".chars() {
        press(&mut app, KeyCode::Char(c));
    }
    press(&mut app, KeyCode::Tab);
    for c in "hunter2".chars() {
        press(&mut app, KeyCode::Char(c));
    }
    let screen = draw(&mut app, 100, 30);
    assert!(screen.contains("Username"));
    assert!(screen.contains("carol"));
    assert!(screen.contains("•••••••"));
    assert!(!screen.contains("hunter2"));
}

#[test]
fn a_confirmation_names_what_will_be_removed() {
    let mut app = app();
    press(&mut app, KeyCode::Char('d'));
    let screen = draw(&mut app, 100, 30);
    assert!(screen.contains("share00"));
    assert!(screen.contains("y"));
}

#[test]
fn the_status_line_shows_the_running_operation() {
    let mut app = app();
    press(&mut app, KeyCode::Char('d'));
    press(&mut app, KeyCode::Char('y'));
    assert!(draw(&mut app, 100, 30).contains("Removing share"));
}

#[test]
fn every_tabs_key_hints_fit_an_80_column_terminal() {
    for tab in Tab::ALL {
        let mut app = app();
        app.tab = tab;
        let screen = draw(&mut app, 80, 24);
        assert!(screen.contains("q quit"), "{tab:?} hints are cut off:\n{screen}");
    }
}
