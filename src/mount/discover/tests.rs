use super::*;
use std::net::Ipv4Addr;

#[test]
fn showmount_output_lists_exports_with_their_clients() {
    let out = "Exports list on 10.0.0.5:\n/export/data   192.168.1.0/24 10.0.0.0/8\n/export/pub    *\n\n/export/open\n";
    let exports = parse_showmount_output("10.0.0.5", out).unwrap();
    assert_eq!(exports.len(), 3);
    assert_eq!(exports[0].server, "10.0.0.5");
    assert_eq!(exports[0].path, "/export/data");
    assert_eq!(exports[0].clients, ["192.168.1.0/24", "10.0.0.0/8"]);
    assert_eq!(exports[1].clients, ["*"]);
    assert!(exports[2].clients.is_empty());
}

#[test]
fn showmount_output_with_an_unexpected_header_is_an_error() {
    assert!(parse_showmount_output("h", "clnt_create: RPC: Program not registered\n").is_err());
}

#[test]
fn empty_showmount_output_has_no_exports() {
    assert!(parse_showmount_output("h", "").unwrap().is_empty());
}

#[test]
fn a_24_bit_subnet_scans_254_hosts() {
    let hosts = subnet_hosts("192.168.10.0/24").unwrap();
    assert_eq!(hosts.len(), 254);
    assert_eq!(hosts[0], Ipv4Addr::new(192, 168, 10, 1));
    assert_eq!(hosts[253], Ipv4Addr::new(192, 168, 10, 254));
}

#[test]
fn host_bits_in_the_subnet_address_are_ignored() {
    assert_eq!(
        subnet_hosts("192.168.10.77/30").unwrap(),
        [Ipv4Addr::new(192, 168, 10, 77), Ipv4Addr::new(192, 168, 10, 78)]
    );
}

#[test]
fn invalid_or_oversized_subnets_are_errors() {
    assert!(subnet_hosts("not-a-subnet").is_err());
    assert!(subnet_hosts("192.168.1.0").is_err());
    assert!(subnet_hosts("192.168.1.0/33").is_err());
    assert!(subnet_hosts("10.0.0.0/8").unwrap_err().contains("too large"));
}

#[test]
fn subnet_of_an_interface_address() {
    assert_eq!(
        network_cidr(Ipv4Addr::new(192, 168, 10, 77), Ipv4Addr::new(255, 255, 255, 0)),
        "192.168.10.0/24"
    );
    assert_eq!(
        network_cidr(Ipv4Addr::new(10, 1, 2, 3), Ipv4Addr::new(255, 255, 0, 0)),
        "10.1.0.0/16"
    );
}
