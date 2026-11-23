mod avahi;
mod cli;
mod mount;
mod nfs;
mod platform;
mod samba;
mod service;
mod system;
mod tui;
mod user;
mod validate;

/// Errors are human-readable messages, shown as-is by the CLI and the TUI.
pub type Result<T> = std::result::Result<T, String>;

fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    if let Err(e) = cli::execute(&args) {
        eprintln!("Error: {e}");
        std::process::exit(1);
    }
}
