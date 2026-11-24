//! Avahi (mDNS) advertisement so Macs discover Time Machine shares on Linux.

use std::fs;
use std::path::Path;

use crate::platform;
use crate::system::has_command;
use crate::Result;

const SERVICES_DIR: &str = "/etc/avahi/services";
const SERVICE_FILE: &str = "/etc/avahi/services/samba.service";

fn is_available() -> bool {
    // macOS advertises through its built-in Bonjour instead.
    !platform::is_macos() && has_command("avahi-daemon")
}

/// Writes the service file advertising `shares`, or removes it when there are none.
/// Does nothing when Avahi is not installed.
pub fn update_time_machine_service(shares: &[String]) -> Result<()> {
    if !is_available() {
        return Ok(());
    }
    if shares.is_empty() {
        if Path::new(SERVICE_FILE).exists() {
            fs::remove_file(SERVICE_FILE).map_err(|e| format!("failed to remove Avahi service file: {e}"))?;
        }
        return Ok(());
    }
    if !Path::new(SERVICES_DIR).exists() {
        return Err(format!(
            "Avahi services directory does not exist: {SERVICES_DIR} (is avahi-daemon installed?)"
        ));
    }
    fs::write(SERVICE_FILE, generate_service_file(shares))
        .map_err(|e| format!("failed to write Avahi service file: {e}"))
}

pub fn generate_service_file(shares: &[String]) -> String {
    let mut xml = String::from(
        r#"<?xml version="1.0" standalone='no'?>
<!DOCTYPE service-group SYSTEM "avahi-service.dtd">
<service-group>
  <name replace-wildcards="yes">%h</name>
  <service>
    <type>_smb._tcp</type>
    <port>445</port>
  </service>
  <service>
    <type>_device-info._tcp</type>
    <port>0</port>
    <txt-record>model=TimeCapsule8,119</txt-record>
  </service>
  <service>
    <type>_adisk._tcp</type>
    <port>9</port>
    <txt-record>sys=waMa=0,adVF=0x100</txt-record>
"#,
    );
    for (i, name) in shares.iter().enumerate() {
        // adVF=0x82: Time Machine volume (0x02), mounted (0x80).
        xml.push_str(&format!(
            "    <txt-record>dk{i}=adVN={name},adVF=0x82</txt-record>\n"
        ));
    }
    xml.push_str("  </service>\n</service-group>\n");
    xml
}

/// `existing` plus `name`, without duplicating it.
pub fn with_share(name: &str, existing: &[String]) -> Vec<String> {
    let mut all = without_share(name, existing);
    all.push(name.to_string());
    all
}

pub fn without_share(name: &str, remaining: &[String]) -> Vec<String> {
    remaining.iter().filter(|n| *n != name).cloned().collect()
}

#[cfg(test)]
mod tests;
