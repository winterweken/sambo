//! A modal form: a list of fields followed by a submit button.

use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};

use super::input::Input;

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Kind {
    Text(Input),
    Checkbox(bool),
    /// `(value, label)` options, one of which is selected.
    Choice {
        options: Vec<(String, String)>,
        selected: usize,
    },
    /// Shown for context; cannot be focused.
    ReadOnly(String),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Field {
    pub key: &'static str,
    pub label: &'static str,
    pub kind: Kind,
}

impl Field {
    pub fn text(key: &'static str, label: &'static str, value: &str) -> Field {
        Field {
            key,
            label,
            kind: Kind::Text(Input::new(value)),
        }
    }

    pub fn password(key: &'static str, label: &'static str) -> Field {
        Field {
            key,
            label,
            kind: Kind::Text(Input::masked()),
        }
    }

    pub fn checkbox(key: &'static str, label: &'static str, checked: bool) -> Field {
        Field {
            key,
            label,
            kind: Kind::Checkbox(checked),
        }
    }

    /// A choice among `(value, label)` options; an unknown `selected` value picks the first.
    pub fn choice(key: &'static str, label: &'static str, options: &[(&str, &str)], selected: &str) -> Field {
        let selected = options
            .iter()
            .position(|(value, _)| *value == selected)
            .unwrap_or(0);
        let options = options
            .iter()
            .map(|(v, l)| (v.to_string(), l.to_string()))
            .collect();
        Field {
            key,
            label,
            kind: Kind::Choice { options, selected },
        }
    }

    pub fn read_only(key: &'static str, label: &'static str, value: &str) -> Field {
        Field {
            key,
            label,
            kind: Kind::ReadOnly(value.to_string()),
        }
    }

    /// Applies an editing key; reports whether the value changed.
    fn edit(&mut self, code: KeyCode) -> bool {
        match (&mut self.kind, code) {
            (Kind::Text(input), KeyCode::Char(c)) => input.insert(c),
            (Kind::Text(input), KeyCode::Backspace) => input.backspace(),
            (Kind::Text(input), KeyCode::Delete) => input.delete(),
            (Kind::Text(input), KeyCode::Left) => return moved(input, Input::left),
            (Kind::Text(input), KeyCode::Right) => return moved(input, Input::right),
            (Kind::Text(input), KeyCode::Home) => return moved(input, Input::home),
            (Kind::Text(input), KeyCode::End) => return moved(input, Input::end),
            (Kind::Checkbox(checked), KeyCode::Char(' ')) => *checked = !*checked,
            (Kind::Choice { options, selected }, KeyCode::Right | KeyCode::Char(' ')) => {
                *selected = (*selected + 1) % options.len();
            }
            (Kind::Choice { options, selected }, KeyCode::Left) => {
                *selected = (*selected + options.len() - 1) % options.len();
            }
            _ => return false,
        }
        true
    }
}

/// Moves the cursor, which does not count as a change to the value.
fn moved(input: &mut Input, movement: fn(&mut Input)) -> bool {
    movement(input);
    false
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Outcome {
    Pending,
    Submit,
    Cancel,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Form {
    pub title: String,
    pub submit_label: &'static str,
    pub fields: Vec<Field>,
    /// Shown under the fields until the next edit.
    pub error: Option<String>,
    /// Index of the focused field; `fields.len()` is the submit button.
    focus: usize,
}

impl Form {
    pub fn new(title: &str, submit_label: &'static str, fields: Vec<Field>) -> Form {
        let mut form = Form {
            title: title.to_string(),
            submit_label,
            fields,
            error: None,
            focus: 0,
        };
        form.focus = form.stops()[0];
        form
    }

    pub fn focus(&self) -> usize {
        self.focus
    }

    pub fn on_submit_button(&self) -> bool {
        self.focus == self.fields.len()
    }

    /// Everything that can take focus, in order; the submit button comes last.
    fn stops(&self) -> Vec<usize> {
        let editable = |(i, field): (usize, &Field)| (!matches!(field.kind, Kind::ReadOnly(_))).then_some(i);
        self.fields
            .iter()
            .enumerate()
            .filter_map(editable)
            .chain([self.fields.len()])
            .collect()
    }

    fn move_focus(&mut self, forward: bool) {
        let stops = self.stops();
        let at = stops.iter().position(|&stop| stop == self.focus).unwrap_or(0);
        let step = if forward { 1 } else { stops.len() - 1 };
        self.focus = stops[(at + step) % stops.len()];
    }

    pub fn on_key(&mut self, key: KeyEvent) -> Outcome {
        if key.modifiers.contains(KeyModifiers::CONTROL) {
            return if key.code == KeyCode::Char('s') {
                Outcome::Submit
            } else {
                Outcome::Pending
            };
        }
        // Alt combinations are terminal shortcuts, not text.
        if key.modifiers.contains(KeyModifiers::ALT) {
            return Outcome::Pending;
        }
        match key.code {
            KeyCode::Esc => return Outcome::Cancel,
            KeyCode::Enter if self.on_submit_button() => return Outcome::Submit,
            KeyCode::Tab | KeyCode::Down | KeyCode::Enter => self.move_focus(true),
            KeyCode::BackTab | KeyCode::Up => self.move_focus(false),
            code => {
                if self
                    .fields
                    .get_mut(self.focus)
                    .is_some_and(|field| field.edit(code))
                {
                    self.error = None;
                }
            }
        }
        Outcome::Pending
    }

    fn kind(&self, key: &str) -> Option<&Kind> {
        self.fields
            .iter()
            .find(|field| field.key == key)
            .map(|field| &field.kind)
    }

    /// The value of a text or read-only field; trimmed unless it is a password.
    pub fn text(&self, key: &str) -> String {
        match self.kind(key) {
            Some(Kind::Text(input)) if input.is_masked() => input.value().to_string(),
            Some(Kind::Text(input)) => input.value().trim().to_string(),
            Some(Kind::ReadOnly(value)) => value.clone(),
            _ => String::new(),
        }
    }

    pub fn checked(&self, key: &str) -> bool {
        matches!(self.kind(key), Some(Kind::Checkbox(true)))
    }

    /// The value of the selected option of a choice field.
    pub fn choice(&self, key: &str) -> String {
        match self.kind(key) {
            Some(Kind::Choice { options, selected }) => options[*selected].0.clone(),
            _ => String::new(),
        }
    }
}

#[cfg(test)]
mod tests;
