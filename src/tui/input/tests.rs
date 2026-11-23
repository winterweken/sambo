use super::*;

fn typed(text: &str) -> Input {
    let mut input = Input::default();
    text.chars().for_each(|c| input.insert(c));
    input
}

#[test]
fn typing_appends_and_moves_the_cursor() {
    let input = typed("abc");
    assert_eq!(input.value(), "abc");
    assert_eq!(input.cursor(), 3);
}

#[test]
fn new_starts_with_the_cursor_at_the_end() {
    assert_eq!(Input::new("hello").cursor(), 5);
}

#[test]
fn inserting_in_the_middle() {
    let mut input = typed("ac");
    input.left();
    input.insert('b');
    assert_eq!(input.value(), "abc");
    assert_eq!(input.cursor(), 2);
}

#[test]
fn backspace_removes_the_character_before_the_cursor() {
    let mut input = typed("abc");
    input.left();
    input.backspace();
    assert_eq!(input.value(), "ac");
    assert_eq!(input.cursor(), 1);
    input.home();
    input.backspace();
    assert_eq!(input.value(), "ac");
}

#[test]
fn delete_removes_the_character_under_the_cursor() {
    let mut input = typed("abc");
    input.delete();
    assert_eq!(input.value(), "abc");
    input.home();
    input.delete();
    assert_eq!(input.value(), "bc");
    assert_eq!(input.cursor(), 0);
}

#[test]
fn cursor_stays_within_bounds() {
    let mut input = typed("ab");
    input.right();
    assert_eq!(input.cursor(), 2);
    input.home();
    input.left();
    assert_eq!(input.cursor(), 0);
    input.end();
    assert_eq!(input.cursor(), 2);
}

#[test]
fn multibyte_characters_are_edited_whole() {
    let mut input = typed("né");
    input.insert('e');
    input.left();
    input.backspace();
    assert_eq!(input.value(), "ne");
    assert_eq!(input.cursor(), 1);
}

#[test]
fn masked_input_hides_its_value_when_displayed() {
    let mut input = Input::masked();
    "s3cret".chars().for_each(|c| input.insert(c));
    assert_eq!(input.display(), "••••••");
    assert_eq!(input.value(), "s3cret");
    assert_eq!(typed("plain").display(), "plain");
}
