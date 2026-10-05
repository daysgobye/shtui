//! `OpenDialog`: the seam between a `Ctrl+O` and a path.
//!
//! The portal itself cannot be driven from a test — it needs a live session bus and a
//! human to answer it — so everything here is tested on the far side of that boundary:
//! the bookkeeping that decides whether a dialog is open, what happens to a second
//! `Ctrl+O` while one is already up, and what a resolved future is reported as. The one
//! thing no test here proves is that a dialog appears, which is verified by hand.

use std::future::Future;
use std::path::PathBuf;
use std::pin::Pin;
use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::task::{Context, Poll};

use omasheet::app::open_dialog::{FILTERS, OpenDialog, OpenOutcome};

/// How many times a [`Never`] has been polled.
type Polls = Arc<AtomicUsize>;

/// A dialog that never resolves, counting its polls so a test can tell which future is
/// being polled.
struct Never {
    polls: Polls,
}

impl Future for Never {
    type Output = Option<PathBuf>;

    fn poll(self: Pin<&mut Self>, _cx: &mut Context<'_>) -> Poll<Self::Output> {
        self.polls.fetch_add(1, Ordering::SeqCst);
        Poll::Pending
    }
}

/// A [`Never`] plus the counter its polls land in.
fn never() -> (Never, Polls) {
    let polls = Arc::new(AtomicUsize::new(0));
    (
        Never {
            polls: polls.clone(),
        },
        polls,
    )
}

fn path(s: &str) -> PathBuf {
    PathBuf::from(s)
}

/// Nothing has been spawned, so there is nothing to report. This is the state the app is
/// in on every frame that is not the frame a `Ctrl+O` landed on, and returning an
/// outcome here instead would mean the app acted on a dialog that never existed.
#[test]
fn poll_before_anything_is_spawned_reports_nothing() {
    let mut d = OpenDialog::new();
    assert_eq!(d.poll(), None);
    assert_eq!(d.poll(), None);
    assert!(!d.is_open());
}

/// A spawned dialog is open until its future resolves, and polling it is what resolves
/// it — the app polls once per frame rather than blocking, so "open" has to be a state
/// the type can report in between.
#[test]
fn a_spawned_dialog_stays_open_while_its_future_is_pending() {
    let mut d = OpenDialog::new();
    let (fut, polls) = never();
    d.set_pending(fut);
    assert!(d.is_open());
    assert_eq!(d.poll(), None);
    assert_eq!(d.poll(), None);
    assert!(d.is_open());
    assert_eq!(polls.load(Ordering::SeqCst), 2);
}

/// The second `Ctrl+O` must not orphan the first dialog's future. Dropping it would
/// leave the portal's dialog on screen with nothing left to receive its answer, so the
/// app would sit there believing no dialog is open while the user is looking at one.
#[test]
fn spawning_twice_keeps_the_first_dialog() {
    let mut d = OpenDialog::new();
    let (first, first_polls) = never();
    let (second, second_polls) = never();
    d.set_pending(first);
    d.set_pending(second);
    assert_eq!(d.poll(), None);
    assert_eq!(
        first_polls.load(Ordering::SeqCst),
        1,
        "the first dialog's future must still be the one being polled"
    );
    assert_eq!(
        second_polls.load(Ordering::SeqCst),
        0,
        "the second future must have been dropped rather than stored"
    );
}

/// `spawn` refuses the same way `set_pending` does, and `spawn` is the only route the
/// app has.
#[test]
fn spawn_is_refused_while_a_dialog_is_open() {
    let mut d = OpenDialog::new();
    let (first, first_polls) = never();
    d.set_pending(first);
    // No portal on a headless run, so this may or may not open a real dialog; either
    // way it must not disturb the one already pending, which is the claim.
    d.spawn();
    assert!(d.is_open());
    assert_eq!(d.poll(), None);
    assert_eq!(first_polls.load(Ordering::SeqCst), 1);
}

/// The chosen path is reported once, and the slot is emptied on the way out so the app
/// cannot load the same file twice from one dialog.
#[test]
fn a_resolved_dialog_reports_the_chosen_path_and_closes() {
    let mut d = OpenDialog::new();
    d.set_pending(std::future::ready(Some(path("/tmp/budget.csv"))));
    assert!(d.is_open());
    assert_eq!(d.poll(), Some(OpenOutcome::Chosen(path("/tmp/budget.csv"))));
    assert!(!d.is_open());
    assert_eq!(d.poll(), None);
}

/// A cancelled dialog is a different answer from an open one, and the app has to be able
/// to tell them apart without nesting `Option`s.
#[test]
fn a_cancelled_dialog_reports_cancelled() {
    let mut d = OpenDialog::new();
    d.set_pending(std::future::ready(None));
    assert_eq!(d.poll(), Some(OpenOutcome::Cancelled));
    assert!(!d.is_open());
}

/// A staged path is held, not loaded. This is the whole of the confirmation path: the
/// chosen path stops here until the app asks for it, so no route exists that discards a
/// dirty workbook without going through the question first.
#[test]
fn a_staged_path_is_held_until_the_app_takes_it() {
    let mut d = OpenDialog::new();
    assert!(!d.is_staged());
    d.stage(path("/tmp/budget.csv"));
    assert!(d.is_staged());
    assert!(d.is_busy(), "a staged path must block a second dialog");
    assert_eq!(d.staged(), Some(path("/tmp/budget.csv")).as_deref());
    assert_eq!(d.take_staged(), Some(path("/tmp/budget.csv")));
    assert!(!d.is_staged());
    assert!(!d.is_busy());
}

/// Dropping the staged path is the "keep editing" answer, and it must leave nothing
/// behind for a later frame to act on.
#[test]
fn declining_the_staged_path_leaves_nothing_staged() {
    let mut d = OpenDialog::new();
    d.stage(path("/tmp/budget.csv"));
    assert_eq!(d.take_staged(), Some(path("/tmp/budget.csv")));
    assert_eq!(d.poll(), None);
    assert!(!d.is_busy());
}

/// Only formats the reader can actually open are offered. Offering `xlsx` today would
/// let the user pick a file whose only possible outcome is an error message.
#[test]
fn only_readable_formats_are_offered() {
    let offered: Vec<&str> = FILTERS
        .iter()
        .flat_map(|(_, exts)| exts.iter().copied())
        .collect();
    assert_eq!(offered, ["csv", "tsv"]);
    assert!(
        !offered.contains(&"xlsx"),
        "xlsx reading is a later milestone; the filter must not be here yet"
    );
}

/// Every filter names its own type, so the dialog is not a list of extensions with no
/// label on them.
#[test]
fn every_filter_is_named() {
    for (name, exts) in FILTERS {
        assert!(
            !name.is_empty(),
            "a filter with no name shows as a blank entry"
        );
        assert!(!exts.is_empty(), "filter {name} matches nothing");
    }
}
