//! Go-style command-line flags (`-name value`, `--name=value`, `-flag`), kept
//! so existing sambo scripts keep working.

use std::collections::HashMap;

use crate::Result;

#[derive(Debug)]
pub struct Flags {
    values: HashMap<String, String>,
    /// Arguments from the first non-flag onwards.
    pub positional: Vec<String>,
}

impl Flags {
    pub fn string(&self, name: &str, default: &str) -> String {
        self.values
            .get(name)
            .cloned()
            .unwrap_or_else(|| default.to_string())
    }

    pub fn boolean(&self, name: &str, default: bool) -> bool {
        self.given(name).unwrap_or(default)
    }

    /// The value of a boolean flag, or None when it was not on the command line.
    pub fn given(&self, name: &str) -> Option<bool> {
        self.values.get(name).map(|v| v == "true")
    }
}

/// Parses `args` given the names of the flags that take a value (`strings`)
/// and the names of the boolean flags (`bools`).
pub fn parse(args: &[String], strings: &[&str], bools: &[&str]) -> Result<Flags> {
    let mut values = HashMap::new();
    let mut i = 0;
    while i < args.len() {
        let arg = &args[i];
        let Some(body) = arg.strip_prefix("--").or_else(|| arg.strip_prefix('-')) else {
            break;
        };
        if body.is_empty() {
            i += 1;
            break;
        }
        let (name, inline) = match body.split_once('=') {
            Some((name, value)) => (name, Some(value.to_string())),
            None => (body, None),
        };
        if bools.contains(&name) {
            let value = match inline.as_deref() {
                None | Some("true") | Some("1") => "true",
                Some("false") | Some("0") => "false",
                Some(other) => return Err(format!("invalid boolean value \"{other}\" for -{name}")),
            };
            values.insert(name.to_string(), value.to_string());
        } else if strings.contains(&name) {
            let value = match inline {
                Some(value) => value,
                None => {
                    i += 1;
                    args.get(i)
                        .cloned()
                        .ok_or_else(|| format!("flag needs an argument: -{name}"))?
                }
            };
            values.insert(name.to_string(), value);
        } else {
            return Err(format!("flag provided but not defined: -{name}"));
        }
        i += 1;
    }
    Ok(Flags {
        values,
        positional: args[i.min(args.len())..].to_vec(),
    })
}

#[cfg(test)]
mod tests;
