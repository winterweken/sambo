use super::*;

fn args(list: &[&str]) -> Vec<String> {
    list.iter().map(|s| s.to_string()).collect()
}

const STRINGS: &[&str] = &["name", "path"];
const BOOLS: &[&str] = &["readonly", "browseable"];

#[test]
fn string_flag_takes_the_next_argument() {
    let f = parse(&args(&["-name", "docs", "-path", "/mnt/x"]), STRINGS, BOOLS).unwrap();
    assert_eq!(f.string("name", ""), "docs");
    assert_eq!(f.string("path", ""), "/mnt/x");
}

#[test]
fn double_dash_and_equals_forms_work() {
    let f = parse(&args(&["--name=docs", "-path=/mnt/x"]), STRINGS, BOOLS).unwrap();
    assert_eq!(f.string("name", ""), "docs");
    assert_eq!(f.string("path", ""), "/mnt/x");
}

#[test]
fn bool_flag_is_true_when_present_and_takes_no_argument() {
    let f = parse(&args(&["-readonly", "-name", "docs"]), STRINGS, BOOLS).unwrap();
    assert!(f.boolean("readonly", false));
    assert_eq!(f.string("name", ""), "docs");
}

#[test]
fn bool_flag_accepts_an_explicit_value() {
    let f = parse(&args(&["-browseable=false"]), STRINGS, BOOLS).unwrap();
    assert!(!f.boolean("browseable", true));
}

#[test]
fn defaults_apply_when_a_flag_is_absent() {
    let f = parse(&args(&[]), STRINGS, BOOLS).unwrap();
    assert_eq!(f.string("name", "fallback"), "fallback");
    assert!(f.boolean("browseable", true));
}

#[test]
fn parsing_stops_at_the_first_positional_argument() {
    let f = parse(
        &args(&["-name", "docs", "10.0.0.1", "-path", "x"]),
        STRINGS,
        BOOLS,
    )
    .unwrap();
    assert_eq!(f.positional, args(&["10.0.0.1", "-path", "x"]));
    assert_eq!(f.string("path", ""), "");
}

#[test]
fn unknown_flag_is_an_error() {
    let err = parse(&args(&["-bogus"]), STRINGS, BOOLS).unwrap_err();
    assert_eq!(err, "flag provided but not defined: -bogus");
}

#[test]
fn string_flag_without_a_value_is_an_error() {
    let err = parse(&args(&["-name"]), STRINGS, BOOLS).unwrap_err();
    assert_eq!(err, "flag needs an argument: -name");
}

#[test]
fn invalid_bool_value_is_an_error() {
    assert!(parse(&args(&["-readonly=maybe"]), STRINGS, BOOLS).is_err());
}

#[test]
fn a_bool_flag_that_was_not_given_is_distinguishable_from_false() {
    let f = parse(&args(&["-readonly=false"]), STRINGS, BOOLS).unwrap();
    assert_eq!(f.given("readonly"), Some(false));
    assert_eq!(f.given("browseable"), None);
}
