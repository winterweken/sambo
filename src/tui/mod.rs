//! The interactive terminal UI.
//!
//! `app` holds all state and reacts to keys and worker responses without
//! doing any I/O; `worker` performs the requested operations on background
//! threads; `ui` draws the state. Nothing blocks the event loop.

mod app;
mod form;
mod input;
mod theme;
mod ui;
mod worker;

use std::sync::mpsc;
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use crossterm::event::{self, Event, KeyEventKind};
use ratatui::DefaultTerminal;

use crate::samba::Manager;
use crate::Result;
use app::App;
use theme::Theme;

const TICK: Duration = Duration::from_millis(100);

pub fn start(manager: Manager) -> Result<()> {
    let mut terminal = ratatui::init();
    let result = run(&mut terminal, Arc::new(manager));
    ratatui::restore();
    result.map_err(|e| format!("terminal error: {e}"))
}

fn run(terminal: &mut DefaultTerminal, manager: Arc<Manager>) -> std::io::Result<()> {
    let (tx, rx) = mpsc::channel();
    let mut app = App::new(Theme::detect());

    while !app.should_quit {
        for request in app.take_requests() {
            let (tx, manager) = (tx.clone(), manager.clone());
            thread::spawn(move || {
                let _ = tx.send(worker::execute(&manager, request));
            });
        }

        terminal.draw(|frame| ui::render(frame, &mut app))?;

        if event::poll(TICK)? {
            if let Event::Key(key) = event::read()? {
                if key.kind == KeyEventKind::Press {
                    app.on_key(key);
                }
            }
        } else {
            app.tick = app.tick.wrapping_add(1);
        }
        while let Ok(response) = rx.try_recv() {
            app.on_response(response);
        }
    }
    Ok(())
}
