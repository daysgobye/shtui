//! The open dialog, as a future this app owns.
//!
//! `rfd` offers two ways to ask for a file and only one of them is usable here.
//! `FileDialog::pick_file` blocks the thread it is called on for as long as the dialog
//! is up — on the main thread that means a frozen window and a window manager that
//! thinks the app has hung. `AsyncFileDialog::pick_file` hands back a future instead,
//! and this module is the whole of the difference between that and an app that can open
//! a file: it holds the future, polls it once per frame from
//! [`crate::app::app::OmaApp::body`], and reports what came back.
//!
//! ## Why a no-op waker
//!
//! Polling a future needs a [`std::task::Waker`], and there are two honest choices here.
//! The waker could be egui's — held, and made to call `request_repaint` when woken —
//! which is the tidiest design in the abstract. It is the wrong one for this future:
//! `rfd`'s portal backend completes through a one-shot channel whose sender is dropped
//! on a worker thread, so the wake arrives from off the main thread at a moment egui is
//! not in a frame, and getting that to mean "repaint now" means holding an egui context
//! inside the dialog and reasoning about thread affinity to save a handful of wakeups
//! per open. The no-op waker costs one frame of latency that nobody can see, because
//! [`OpenDialog::poll`] is called every frame anyway and the caller asks for a repaint
//! while the dialog is open. The wake is then stored by the channel, invoked, and
//! discarded — harmless, because nothing depends on it.
//!
//! ## What is deliberately not here
//!
//! Whether a chosen path may be opened at all is not this module's decision. It holds
//! the path in `staged` until [`OpenDialog::take_staged`] is called, which is what gives
//! the app somewhere to put the question asked before unsaved work is discarded.
//!
//! There is also no timeout and no way to cancel. The portal backend's worker blocks in
//! `dbus_connection_read_write` with a `-1` timeout — meaning forever, in
//! `xdg_desktop_portal/portal/mod.rs` — so a portal that never answers leaves
//! `pending` set: [`OpenDialog::is_busy`] stays true and every later `Ctrl+O` is
//! refused, while the app keeps drawing frames the whole time because `OmaApp::open`
//! asks for them while `is_open`. The user is left with a window that accepts `Ctrl+O`
//! and does nothing and a workbook they cannot save, and their way out is killing the
//! process. Neither a deadline nor a cancel route exists here because neither is
//! reachable through rfd's future — dropping the future would drop the worker thread
//! with it, which is the cancel route, and it is not offered from the UI. This is
//! recorded rather than fixed: it needs either a new dependency on the portal crates
//! rfd wraps or an `abort` handle of our own, and until then it is the failure to watch
//! for on a machine with a half-installed desktop portal.

use std::future::Future;
use std::path::PathBuf;
use std::pin::Pin;
use std::task::{Context, Poll, Wake, Waker};

/// The file types the open dialog offers, as `(name, extensions)`.
///
/// Only what [`crate::io::csv_read`] can actually read. `xlsx` arrives with the reader
/// for it; offering the filter before then would let a user pick a file whose only
/// possible outcome is an error, which is worse than not offering it at all — the
/// dialog would be advertising a capability the app does not have.
pub const FILTERS: &[(&str, &[&str])] = &[
    ("Comma-separated values", &["csv"]),
    ("Tab-separated values", &["tsv"]),
];

/// How a dialog ended.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum OpenOutcome {
    /// The user picked this file.
    Chosen(PathBuf),
    /// The user dismissed the dialog without picking anything.
    Cancelled,
}

/// A waker that does nothing. See the module docs for why that is the right shape here
/// rather than a waker that drives egui.
struct NoopWaker;

impl Wake for NoopWaker {
    fn wake(self: std::sync::Arc<Self>) {}
}

fn noop_waker() -> Waker {
    Waker::from(std::sync::Arc::new(NoopWaker))
}

/// One open dialog, if one is up.
type Pending = Pin<Box<dyn Future<Output = Option<PathBuf>>>>;

/// The app's open dialog.
///
/// Holds at most one dialog. A second `Ctrl+O` while one is up is refused rather than
/// served, because replacing the stored future would drop it while the portal's dialog
/// was still on screen — the user would be looking at a dialog that could no longer
/// report an answer.
#[derive(Default)]
pub struct OpenDialog {
    pending: Option<Pending>,
    /// A path the user chose that the app has not decided to open yet. See
    /// [`OpenDialog::stage`].
    staged: Option<PathBuf>,
}

impl OpenDialog {
    pub fn new() -> Self {
        Self::default()
    }

    /// Whether a dialog is up right now.
    pub fn is_open(&self) -> bool {
        self.pending.is_some()
    }

    /// Whether a chosen path is waiting on the app's decision.
    pub fn is_staged(&self) -> bool {
        self.staged.is_some()
    }

    /// Whether the open route is occupied — by a live dialog or by a path waiting to be
    /// confirmed. A second `Ctrl+O` is only worth acting on when this is false.
    ///
    /// The staged case matters as much as the live one: a confirmation on screen that a
    /// second dialog could cover up would leave two questions about the same workbook
    /// with no way to answer the first.
    pub fn is_busy(&self) -> bool {
        self.is_open() || self.is_staged()
    }

    /// Ask the platform for a file. Does nothing if a dialog is already up.
    pub fn spawn(&mut self) {
        if self.is_open() {
            return;
        }
        let dialog = FILTERS
            .iter()
            .fold(rfd::AsyncFileDialog::new(), |d, (name, exts)| {
                d.add_filter(*name, exts)
            })
            .set_title("Open");
        // The future rather than the call: `AsyncFileDialog::pick_file` only starts the
        // portal request when it is first polled, so nothing has left the process yet at
        // this point, and `spawn` costs nothing.
        self.pending = Some(Box::pin(async move {
            dialog
                .pick_file()
                .await
                .map(|handle| handle.path().to_owned())
        }));
    }

    /// Put an arbitrary future in the pending slot, in place of [`OpenDialog::spawn`].
    ///
    /// The portal cannot be driven from a test, so this is the seam the tests drive the
    /// app through: everything from `Ctrl+O` to a loaded sheet is reachable without a
    /// session bus and a human. It refuses a second future for the same reason
    /// `spawn` does, so the tests exercise the real rule rather than a laxer one.
    pub fn set_pending(&mut self, future: impl Future<Output = Option<PathBuf>> + 'static) {
        if self.is_open() {
            return;
        }
        self.pending = Some(Box::pin(future));
    }

    /// Poll the dialog once. `None` while it is still up.
    ///
    /// `Cancelled` is an answer rather than an absence, and the app can tell it from
    /// `None` without nesting `Option`s — which is the whole reason this returns an
    /// enum instead of the `Option<Option<PathBuf>>` the future itself produces.
    pub fn poll(&mut self) -> Option<OpenOutcome> {
        // Taken out for the duration of the poll and put back if it did not resolve:
        // a future cannot be polled through a shared borrow and it must not be dropped
        // by the early return that a resolved poll takes.
        let mut pending = self.pending.take()?;
        let waker = noop_waker();
        let mut cx = Context::from_waker(&waker);
        match pending.as_mut().poll(&mut cx) {
            Poll::Ready(chosen) => Some(chosen.map_or(OpenOutcome::Cancelled, OpenOutcome::Chosen)),
            Poll::Pending => {
                self.pending = Some(pending);
                None
            }
        }
    }

    /// Hold a chosen path without opening it.
    ///
    /// The app stages rather than opens so that the one destructive thing it can do —
    /// replacing a dirty workbook — is always a decision it made on a later frame, in
    /// front of the user, rather than a side effect of a keypress.
    pub fn stage(&mut self, path: PathBuf) {
        self.staged = Some(path);
    }

    /// The path waiting to be opened, if there is one.
    pub fn staged(&self) -> Option<&std::path::Path> {
        self.staged.as_deref()
    }

    /// Take the staged path, if there is one. The only route by which a chosen path
    /// reaches the reader.
    pub fn take_staged(&mut self) -> Option<PathBuf> {
        self.staged.take()
    }
}
