//! Styles. Only the terminal's own 16 colours are used, so the UI follows the
//! user's colour scheme and works on consoles without true-colour support.

use ratatui::style::{Color, Modifier, Style};

#[derive(Debug, Clone)]
pub struct Theme {
    pub title: Style,
    pub accent: Style,
    pub selected: Style,
    pub header: Style,
    pub muted: Style,
    pub success: Style,
    pub warning: Style,
    pub error: Style,
    pub key: Style,
}

impl Theme {
    /// Colour unless the NO_COLOR convention asks for none.
    pub fn detect() -> Theme {
        if std::env::var_os("NO_COLOR").is_some_and(|v| !v.is_empty()) {
            Theme::plain()
        } else {
            Theme::color()
        }
    }

    pub fn color() -> Theme {
        let bold = Style::new().add_modifier(Modifier::BOLD);
        Theme {
            title: bold.fg(Color::Cyan),
            accent: Style::new().fg(Color::Cyan),
            selected: bold.fg(Color::Black).bg(Color::Cyan),
            header: bold.fg(Color::Cyan),
            muted: Style::new().fg(Color::DarkGray),
            success: Style::new().fg(Color::Green),
            warning: Style::new().fg(Color::Yellow),
            error: bold.fg(Color::Red),
            key: bold.fg(Color::Cyan),
        }
    }

    pub fn plain() -> Theme {
        let plain = Style::new();
        let bold = plain.add_modifier(Modifier::BOLD);
        Theme {
            title: bold,
            accent: plain,
            selected: bold.add_modifier(Modifier::REVERSED),
            header: bold.add_modifier(Modifier::UNDERLINED),
            muted: plain.add_modifier(Modifier::DIM),
            success: plain,
            warning: bold,
            error: bold,
            key: bold,
        }
    }
}
