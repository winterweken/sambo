use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};

use super::*;

fn key(code: KeyCode) -> KeyEvent {
    KeyEvent::new(code, KeyModifiers::NONE)
}

fn sample() -> Form {
    Form::new(
        "Sample",
        "Save",
        vec![
            Field::read_only("id", "Id", "42"),
            Field::text("name", "Name", "bob"),
            Field::password("secret", "Secret"),
            Field::checkbox("on", "Enabled", false),
            Field::choice(
                "kind",
                "Kind",
                &[("a", "Alpha"), ("b", "Beta"), ("c", "Gamma")],
                "b",
            ),
        ],
    )
}

fn type_text(form: &mut Form, text: &str) {
    for c in text.chars() {
        form.on_key(key(KeyCode::Char(c)));
    }
}

#[test]
fn focus_starts_on_the_first_editable_field() {
    assert_eq!(sample().focus(), 1);
}

#[test]
fn tab_and_down_move_forward_and_end_on_the_submit_button() {
    let mut form = sample();
    form.on_key(key(KeyCode::Tab));
    assert_eq!(form.focus(), 2);
    form.on_key(key(KeyCode::Down));
    form.on_key(key(KeyCode::Tab));
    form.on_key(key(KeyCode::Tab));
    assert_eq!(form.focus(), 5);
    assert!(form.on_submit_button());
}

#[test]
fn focus_wraps_and_skips_read_only_fields() {
    let mut form = sample();
    form.on_key(key(KeyCode::BackTab));
    assert!(form.on_submit_button());
    form.on_key(key(KeyCode::Tab));
    assert_eq!(form.focus(), 1);
    form.on_key(key(KeyCode::Up));
    assert!(form.on_submit_button());
}

#[test]
fn typing_edits_the_focused_text_field() {
    let mut form = sample();
    type_text(&mut form, "by");
    form.on_key(key(KeyCode::Backspace));
    assert_eq!(form.text("name"), "bobb");
}

#[test]
fn text_is_trimmed_but_passwords_are_not() {
    let mut form = sample();
    type_text(&mut form, "  ");
    form.on_key(key(KeyCode::Tab));
    type_text(&mut form, " p w ");
    assert_eq!(form.text("name"), "bob");
    assert_eq!(form.text("secret"), " p w ");
}

#[test]
fn read_only_values_are_readable() {
    assert_eq!(sample().text("id"), "42");
}

#[test]
fn space_toggles_a_checkbox() {
    let mut form = sample();
    form.on_key(key(KeyCode::Tab));
    form.on_key(key(KeyCode::Tab));
    assert!(!form.checked("on"));
    form.on_key(key(KeyCode::Char(' ')));
    assert!(form.checked("on"));
    form.on_key(key(KeyCode::Char(' ')));
    assert!(!form.checked("on"));
}

#[test]
fn left_and_right_cycle_a_choice() {
    let mut form = sample();
    for _ in 0..3 {
        form.on_key(key(KeyCode::Tab));
    }
    assert_eq!(form.choice("kind"), "b");
    form.on_key(key(KeyCode::Right));
    assert_eq!(form.choice("kind"), "c");
    form.on_key(key(KeyCode::Right));
    assert_eq!(form.choice("kind"), "a");
    form.on_key(key(KeyCode::Left));
    assert_eq!(form.choice("kind"), "c");
    form.on_key(key(KeyCode::Char(' ')));
    assert_eq!(form.choice("kind"), "a");
}

#[test]
fn an_unknown_initial_choice_selects_the_first_option() {
    let form = Form::new(
        "t",
        "ok",
        vec![Field::choice("k", "K", &[("a", "A"), ("b", "B")], "zzz")],
    );
    assert_eq!(form.choice("k"), "a");
}

#[test]
fn enter_moves_to_the_next_field_and_submits_on_the_button() {
    let mut form = sample();
    assert_eq!(form.on_key(key(KeyCode::Enter)), Outcome::Pending);
    assert_eq!(form.focus(), 2);
    form.on_key(key(KeyCode::BackTab));
    form.on_key(key(KeyCode::BackTab));
    assert!(form.on_submit_button());
    assert_eq!(form.on_key(key(KeyCode::Enter)), Outcome::Submit);
}

#[test]
fn ctrl_s_submits_from_any_field() {
    let mut form = sample();
    let outcome = form.on_key(KeyEvent::new(KeyCode::Char('s'), KeyModifiers::CONTROL));
    assert_eq!(outcome, Outcome::Submit);
    assert_eq!(form.text("name"), "bob");
}

#[test]
fn escape_cancels() {
    assert_eq!(sample().on_key(key(KeyCode::Esc)), Outcome::Cancel);
}

#[test]
fn editing_clears_a_previous_error() {
    let mut form = sample();
    form.error = Some("bad".into());
    form.on_key(key(KeyCode::Tab));
    assert!(form.error.is_some());
    type_text(&mut form, "x");
    assert!(form.error.is_none());
}

#[test]
fn alt_modified_keys_are_not_typed() {
    let mut form = sample();
    form.on_key(KeyEvent::new(KeyCode::Char('x'), KeyModifiers::ALT));
    assert_eq!(form.text("name"), "bob");
}
