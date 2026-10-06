//! A log, so a portal that cannot be reached says so.
//!
//! `rfd` reports a failure with `log::error!` and then resolves its dialog future to
//! `None`, which [`OpenDialog::poll`] reports as [`OpenOutcome::Cancelled`] — the same
//! answer a user gives by dismissing the dialog. So on a machine with no portal and no
//! zenity, `Ctrl+O` does nothing at all: no chooser, no error label, and no difference
//! between a broken session bus and a keystroke that did not register. The `log` facade
//! is already in the tree, because `rfd` logs through it, but nothing was installing a
//! logger for it, so every one of those messages went nowhere.
//!
//! The sink is a file beside the executable, or stderr when no file could be opened:
//! stderr is what `cargo run` already shows, and it keeps working when the executable
//! lives somewhere read-only. Nothing beyond the facade — a missing portal is a
//! diagnosability problem, not a feature, and the only thing a logger has to do here is
//! make another crate's messages land somewhere.

use std::fs::{File, OpenOptions};
use std::io::{self, Write};
use std::path::PathBuf;
use std::sync::Mutex;
use std::time::{SystemTime, UNIX_EPOCH};

/// The log's file name, beside the executable.
const LOG_NAME: &str = "omasheet.log";

/// Where the log goes, if the executable's own path can be read.
///
/// `None` leaves it on stderr. Beside the executable rather than in a user directory
/// because that is where someone who has just seen this app do nothing will look, and
/// because a path here needs no configuration and no platform-specific directory the
/// `dirs` crate would be pulled in for.
pub fn log_path() -> Option<PathBuf> {
    Some(std::env::current_exe().ok()?.parent()?.join(LOG_NAME))
}

/// Start logging, and report whether this call is the one that did it.
///
/// `false` means a logger was already installed, which is not a failure worth stopping
/// for: [`install`] is called once from `main`, and the answer is only there so the
/// caller can say so rather than leave a silent gap where the log should be.
pub fn install() -> bool {
    let path = log_path();
    let file = path.as_ref().and_then(|p| {
        OpenOptions::new()
            .create(true)
            .append(true)
            .open(p)
            // Reported on stderr rather than swallowed: this is the one case where the
            // fallback matters to whoever is about to read a log, and at this point
            // there is no logger yet to say it with.
            .inspect_err(|e| eprintln!("omasheet: {}: {e}", p.display()))
            .ok()
    });
    if log::set_boxed_logger(Box::new(FileLogger {
        file: Mutex::new(file),
    }))
    .is_ok()
    {
        // `Info` and above: rfd reports failures at `Error`, and nothing in this tree
        // logs below that, so a lower level would be pure noise.
        log::set_max_level(log::LevelFilter::Info);
        true
    } else {
        false
    }
}

/// Appends each record to the file, or to stderr when there is no file.
struct FileLogger {
    /// `Mutex` rather than `RefCell` because `log::set_boxed_logger` wants `Send + Sync`
    /// and rfd logs from its portal thread as well as the main one. The lock is taken
    /// for the length of one `write_all`.
    file: Mutex<Option<File>>,
}

impl log::Log for FileLogger {
    fn enabled(&self, metadata: &log::Metadata<'_>) -> bool {
        metadata.level() <= log::Level::Info
    }

    fn log(&self, record: &log::Record<'_>) {
        if !self.enabled(record.metadata()) {
            return;
        }
        // Seconds since the epoch, not a wall clock. What a reader wants from a log of a
        // desktop app is the order of the lines within one session, and formatting a date
        // would mean a dependency for that.
        let since_epoch = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs())
            .unwrap_or_default();
        let line = format!(
            "[{since_epoch}] {} {}: {}\n",
            record.level(),
            record.target(),
            record.args()
        );
        // Recovered rather than propagated: a panic in whatever logged should not silence
        // the next line, and `log` has nowhere for a poison error to go anyway.
        let mut guard = self
            .file
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        let written = match guard.as_mut() {
            Some(file) => file.write_all(line.as_bytes()),
            None => io::stderr().write_all(line.as_bytes()),
        };
        let _ = written;
    }

    fn flush(&self) {
        let guard = self
            .file
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        if let Some(mut file) = guard.as_ref() {
            let _ = file.flush();
        }
    }
}
