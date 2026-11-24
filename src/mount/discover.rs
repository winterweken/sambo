//! Discovery of NFS servers on the local network.

use std::net::{Ipv4Addr, SocketAddr, TcpStream};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Mutex;
use std::thread;
use std::time::Duration;

use crate::system::run;
use crate::Result;

/// Smallest prefix we are willing to scan (a /16 is 65k hosts).
const MIN_PREFIX: u32 = 16;
const SCAN_THREADS: usize = 50;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NfsExport {
    pub server: String,
    pub path: String,
    /// Allowed clients; may be empty or contain "*".
    pub clients: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NfsServer {
    pub ip: String,
    pub exports: Vec<NfsExport>,
}

/// Parses `showmount -e` output.
pub fn parse_showmount_output(server: &str, output: &str) -> Result<Vec<NfsExport>> {
    let mut lines = output.lines();
    if let Some(header) = lines.next() {
        if !header.starts_with("Exports list") {
            return Err("unexpected showmount output format".into());
        }
    }
    // The output comes from another machine and is shown in a terminal, so
    // control characters (escape sequences included) are dropped.
    let clean = |field: &str| -> String { field.chars().filter(|c| !c.is_control()).collect() };
    Ok(lines
        .filter_map(|line| {
            let mut fields = line
                .split_whitespace()
                .map(clean)
                .filter(|field| !field.is_empty());
            let path = fields.next().filter(|path| path.starts_with('/'))?;
            Some(NfsExport {
                server: server.to_string(),
                path,
                clients: fields.collect(),
            })
        })
        .collect())
}

/// The host addresses of an IPv4 subnet in CIDR notation, without the network
/// and broadcast addresses.
pub fn subnet_hosts(cidr: &str) -> Result<Vec<Ipv4Addr>> {
    let invalid = || format!("invalid subnet: {cidr}");
    let (addr, prefix) = cidr.split_once('/').ok_or_else(invalid)?;
    let addr: Ipv4Addr = addr.parse().map_err(|_| invalid())?;
    let prefix: u32 = prefix.parse().ok().filter(|p| *p <= 32).ok_or_else(invalid)?;
    if prefix < MIN_PREFIX {
        return Err(format!(
            "subnet {cidr} is too large to scan (use /{MIN_PREFIX} or smaller)"
        ));
    }
    let mask = if prefix == 0 { 0 } else { u32::MAX << (32 - prefix) };
    let network = u32::from(addr) & mask;
    let broadcast = network | !mask;
    let all: Vec<Ipv4Addr> = (network..=broadcast).map(Ipv4Addr::from).collect();
    if all.len() > 2 {
        Ok(all[1..all.len() - 1].to_vec())
    } else {
        Ok(all)
    }
}

pub fn network_cidr(ip: Ipv4Addr, mask: Ipv4Addr) -> String {
    let mask = u32::from(mask);
    format!("{}/{}", Ipv4Addr::from(u32::from(ip) & mask), mask.count_ones())
}

/// The subnet of the first non-loopback IPv4 interface.
pub fn local_subnet() -> Result<String> {
    let interfaces = if_addrs::get_if_addrs().map_err(|e| e.to_string())?;
    interfaces
        .into_iter()
        .filter(|iface| !iface.is_loopback())
        .find_map(|iface| match iface.addr {
            if_addrs::IfAddr::V4(v4) => Some(network_cidr(v4.ip, v4.netmask)),
            _ => None,
        })
        .ok_or_else(|| "no suitable network interface found".to_string())
}

pub fn get_nfs_exports(server: &str) -> Result<Vec<NfsExport>> {
    let output = run("showmount", &["-e", server]).map_err(|e| format!("showmount failed: {e}"))?;
    parse_showmount_output(server, &output)
}

/// Scans `subnet` for hosts with the NFS port open and asks each for its exports.
pub fn discover_nfs_servers(subnet: &str, timeout: Duration) -> Result<Vec<NfsServer>> {
    let hosts = subnet_hosts(subnet)?;
    let next = AtomicUsize::new(0);
    let found = Mutex::new(Vec::new());

    thread::scope(|scope| {
        for _ in 0..SCAN_THREADS.min(hosts.len()) {
            scope.spawn(|| {
                while let Some(ip) = hosts.get(next.fetch_add(1, Ordering::Relaxed)) {
                    if TcpStream::connect_timeout(&SocketAddr::from((*ip, 2049)), timeout).is_err() {
                        continue;
                    }
                    let ip = ip.to_string();
                    if let Ok(exports) = get_nfs_exports(&ip) {
                        if !exports.is_empty() {
                            found.lock().unwrap().push(NfsServer { ip, exports });
                        }
                    }
                }
            });
        }
    });

    let mut servers = found.into_inner().unwrap();
    servers.sort_by_key(|s| s.ip.parse::<Ipv4Addr>().ok());
    Ok(servers)
}

#[cfg(test)]
mod tests;
