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
//! lives somewhere read-only. The file is created by the first record rather than by
//! [`install`], so a launch with nothing to report leaves nothing behind — an empty log
//! appearing beside the executable on every first run reads as evidence that something
//! failed, and there is no rotation, so a file that always exists only ever grows.
//! Nothing beyond the facade — a missing portal is a diagnosability problem, not a
//! feature, and the only thing a logger has to do here is make another crate's messages
//! land somewhere.

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
/// caller can say so rather than leave a silent gap where the log should be. Nothing is
/// opened here — see [`Sink`], which creates the file on the first record.
pub fn install() -> bool {
    let sink = match log_path() {
        Some(path) => Sink::Pending(path),
        // No path to log to, so stderr from the first record. Reported by nobody: there
        // is no log to report it in, and the records themselves are on stderr already.
        None => Sink::Stderr,
    };
    if log::set_boxed_logger(Box::new(FileLogger {
        sink: Mutex::new(sink),
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

/// Where records go, before the first one has decided.
enum Sink {
    /// A file that does not exist yet. See the module docs for why it is not created
    /// eagerly.
    Pending(PathBuf),
    /// The file, open.
    File(File),
    /// stderr, because there was no file to write to or the file could not be opened.
    Stderr,
}

impl Sink {
    /// Move to [`Sink::File`] on the first record, and stay there.
    ///
    /// The failure is remembered rather than retried, so a read-only directory is
    /// reported once instead of on every record for the rest of the session.
    fn open(&mut self) {
        if !matches!(self, Sink::Pending(_)) {
            return;
        }
        let pending = std::mem::replace(self, Sink::Stderr);
        if let Sink::Pending(path) = pending {
            *self = match open_log_file(&path) {
                Ok(file) => Sink::File(file),
                Err(e) => {
                    // Reported on stderr, which is where the records are going instead, and
                    // there is no logger yet to say it with.
                    eprintln!("omasheet: {}: {e}", path.display());
                    Sink::Stderr
                }
            };
        }
    }

    /// Append one line, and return the result so a caller holding no logger can ignore it
    /// knowingly rather than by accident.
    fn write_line(&mut self, line: &str) -> io::Result<()> {
        self.open();
        match self {
            Sink::File(file) => file.write_all(line.as_bytes()),
            _ => io::stderr().write_all(line.as_bytes()),
        }
    }
}

/// Open the log file for appending, creating it if this is the first run that needed one.
fn open_log_file(path: &std::path::Path) -> io::Result<File> {
    OpenOptions::new().create(true).append(true).open(path)
}

/// Appends each record to the file, or to stderr when there is no file.
struct FileLogger {
    /// `Mutex` rather than `RefCell` because `log::set_boxed_logger` wants `Send + Sync`
    /// and rfd logs from its portal thread as well as the main one. The lock is taken
    /// for the length of one `write_all`.
    sink: Mutex<Sink>,
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
            .sink
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        let _ = guard.write_line(&line);
    }

    fn flush(&self) {
        let mut guard = self
            .sink
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        // Deliberately not `Sink::open`: flushing must not create the file, or every
        // launch would leave one behind again.
        if let Sink::File(file) = &mut *guard {
            let _ = file.flush();
        }
    }
}
