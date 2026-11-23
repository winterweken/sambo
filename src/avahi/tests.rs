use super::*;

#[test]
fn service_file_lists_each_time_machine_share_as_a_disk() {
    let xml = generate_service_file(&["backup".to_string(), "tm2".to_string()]);
    assert!(xml.starts_with("<?xml version=\"1.0\" standalone='no'?>"));
    assert!(xml.contains("<type>_adisk._tcp</type>"));
    assert!(xml.contains("    <txt-record>dk0=adVN=backup,adVF=0x82</txt-record>\n"));
    assert!(xml.contains("    <txt-record>dk1=adVN=tm2,adVF=0x82</txt-record>\n"));
    assert!(xml.ends_with("  </service>\n</service-group>\n"));
}

#[test]
fn adding_a_share_does_not_duplicate_it() {
    let existing = vec!["a".to_string(), "b".to_string()];
    assert_eq!(with_share("b", &existing), ["a", "b"]);
    assert_eq!(with_share("c", &existing), ["a", "b", "c"]);
}

#[test]
fn removing_a_share_filters_it_out() {
    let remaining = vec!["a".to_string(), "b".to_string()];
    assert_eq!(without_share("a", &remaining), ["b"]);
}
