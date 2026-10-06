//! File open: `Ctrl+O`, the chosen path, the confirmation, and the window title.
//!
//! The portal dialog cannot be driven from a test — it needs a live session bus and a
//! human to answer it — so every claim here is pinned on the near side of that
//! boundary. `OpenDialog::set_pending` is the seam: a test puts a future where the
//! portal's would go and the app cannot tell the difference. What that leaves
//! unverified is exactly one thing, whether a dialog appears when `Ctrl+O` is pressed,
//! which is verified by hand rather than here.
//!
//! Two more things go unverified, for reasons this repo already accepts. `Context::run_ui`
//! does not simulate clicks, so the confirmation's two *buttons* are never pressed by
//! any test below — only the app methods they call are, which is the decision the
//! buttons exist to make. And the window title is asserted as the string
//! [`omasheet::app::app::OmaApp::window_title`] produces *and* as the viewport command
//! that carries it: `run_ui` returns a `FullOutput` whose
//! `viewport_output[&ViewportId::ROOT]` holds the commands and the repaint delay, so both
//! are observable here.
//!
//! ## The one rule this file has to keep
//!
//! A test that presses `Ctrl+O` **must not pass another frame afterwards.** The chord
//! leaves a real `rfd` future in the app, and the next pass polls it — which asks the
//! portal for a file and puts a live file chooser on the developer's screen in the
//! middle of `cargo test`. That is not hypothetical: it is exactly what happened while
//! this file was being written. So the tests below either press `Ctrl+O` and stop, or
//! press it against a future the test supplied, which the app refuses to replace.

use egui::ViewportOutput;
use egui::{Event, Key, Modifiers, RawInput, Rect, ViewportCommand, pos2, vec2};
use omasheet::app::app::OmaApp;
use omasheet::workbook::cell::{CellRef, CellValue};

const A1: CellRef = CellRef {
    sheet: 0,
    row: 0,
    col: 0,
};

/// A CSV on disk that deletes itself, so a test that opens one leaves nothing behind.
struct Fixture {
    path: std::path::PathBuf,
}

impl Fixture {
    fn new(name: &str, contents: &str) -> Self {
        let path =
            std::env::temp_dir().join(format!("omasheet-open-{}-{name}", std::process::id()));
        std::fs::write(&path, contents).expect("temp fixture should be writable");
        Self { path }
    }

    fn missing(name: &str) -> Self {
        let path =
            std::env::temp_dir().join(format!("omasheet-open-{}-{name}", std::process::id()));
        let _ = std::fs::remove_file(&path);
        Self { path }
    }

    /// The file's own name, which is what the window title is supposed to show.
    fn name(&self) -> String {
        self.path
            .file_name()
            .expect("a fixture is a file in a directory")
            .to_string_lossy()
            .into_owned()
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = std::fs::remove_file(&self.path);
    }
}

fn ctrl() -> Modifiers {
    Modifiers {
        ctrl: true,
        ..Default::default()
    }
}

fn key(k: Key) -> Vec<Event> {
    vec![Event::Key {
        key: k,
        physical_key: Some(k),
        pressed: true,
        repeat: false,
        modifiers: Modifiers::default(),
    }]
}

fn chord(k: Key) -> Vec<Event> {
    vec![Event::Key {
        key: k,
        physical_key: Some(k),
        pressed: true,
        repeat: false,
        modifiers: ctrl(),
    }]
}

fn type_text(s: &str) -> Vec<Event> {
    vec![Event::Text(s.to_owned())]
}

struct Sheet {
    ctx: egui::Context,
    app: OmaApp,
}

impl Sheet {
    fn new() -> Self {
        let mut s = Sheet {
            ctx: egui::Context::default(),
            app: OmaApp::default(),
        };
        s.pass(Vec::new());
        s
    }

    fn pass(&mut self, events: Vec<Event>) -> ViewportOutput {
        self.pass_with(events, Modifiers::default())
    }

    fn pass_with(&mut self, events: Vec<Event>, modifiers: Modifiers) -> ViewportOutput {
        let input = RawInput {
            screen_rect: Some(Rect::from_min_size(pos2(0.0, 0.0), vec2(1100.0, 700.0))),
            events,
            modifiers,
            ..Default::default()
        };
        let Sheet { ctx, app, .. } = self;
        let full = ctx.run_ui(input, |ui| app.body(ui));
        // The root viewport's half of it. `FullOutput::viewport_output` carries what the
        // frame told the window manager — the commands it was sent and when it was asked to
        // redraw — and there is exactly one viewport in a headless run, so the root is the
        // whole of it. Cloned rather than moved so the harness can keep returning
        // `ViewportOutput` from a pass the caller may only read a field of.
        full.viewport_output[&egui::ViewportId::ROOT].clone()
    }

    /// `Ctrl+O`.
    ///
    /// The one gesture in this file that can reach the portal, and it does not: the
    /// future this leaves pending is not polled until the next pass. See the rule in
    /// the file's docs — a test that presses this must not pass again.
    fn ctrl_o(&mut self) -> ViewportOutput {
        self.pass_with(chord(Key::O), ctrl())
    }

    fn value(&self, at: CellRef) -> CellValue {
        self.app.wb.value_at(at)
    }

    /// Pass frames until nothing is asking for another one.
    ///
    /// The grid's scrollbars fade in over `Style::animation_time` — 0.2 s by default, so
    /// twelve passes at egui's predicted 1/60 s — and a running animation asks for a
    /// repaint every frame. That repaint is immediate, and an immediate request outranks a
    /// delayed one, so until the fade finishes `repaint_delay` reads zero and the frame
    /// says nothing about *when* a later request would fire. Stopping at the first settled
    /// frame rather than counting passes keeps this correct if egui changes the timing;
    /// bounded so a version that never settles fails the test instead of hanging it.
    fn settle(&mut self) {
        for _ in 0..60 {
            if self.pass(Vec::new()).repaint_delay == std::time::Duration::MAX {
                return;
            }
        }
        panic!("nothing asked for a repaint within 60 passes, so the frame never settles");
    }
}

/// Make the workbook dirty the way a user does — by typing — rather than by reaching
/// for the flag, so the tests that depend on it are depending on the real route.
fn type_into(s: &mut Sheet, text: &str) {
    for c in text.chars() {
        s.pass(type_text(&c.to_string()));
    }
}

/// Put a future where the portal's would go, so the app takes the same route to a path.
fn choose(s: &mut Sheet, path: &std::path::Path) {
    s.app
        .open_dialog
        .set_pending(std::future::ready(Some(path.to_owned())));
}

/// A dialog that picks a file on its `n`th poll and stays pending until then.
///
/// Polls are counted rather than the answer being `ready`, because a future that
/// resolves at once cannot tell "the app kept it" from "the app threw it away and put
/// something else there": both would be open on the next frame. This one can, because
/// only the original carries the file.
struct OnNthPoll {
    resolve_on: u32,
    polls: u32,
    path: std::path::PathBuf,
}

impl OnNthPoll {
    fn new(resolve_on: u32, path: std::path::PathBuf) -> Self {
        Self {
            resolve_on,
            polls: 0,
            path,
        }
    }
}

impl std::future::Future for OnNthPoll {
    type Output = Option<std::path::PathBuf>;

    fn poll(
        mut self: std::pin::Pin<&mut Self>,
        _cx: &mut std::task::Context<'_>,
    ) -> std::task::Poll<Self::Output> {
        self.polls += 1;
        if self.polls >= self.resolve_on {
            std::task::Poll::Ready(Some(std::mem::take(&mut self.path)))
        } else {
            std::task::Poll::Pending
        }
    }
}

// --- The chord ---

/// `Ctrl+O` reaches the dialog. This is the whole claim: the key is read, and a dialog
/// is asked for.
#[test]
fn ctrl_o_asks_for_a_dialog() {
    let mut s = Sheet::new();
    assert!(!s.app.open_dialog.is_open());
    s.ctrl_o();
    assert!(s.app.open_dialog.is_open());
}

/// `Ctrl+O` is the chord, not the bare key: pressing `O` must not open anything, since
/// `O` is a character a user types into a cell.
#[test]
fn the_bare_key_does_not_ask_for_a_dialog() {
    let mut s = Sheet::new();
    s.pass(key(Key::O));
    assert!(!s.app.open_dialog.is_open());
}

/// A second `Ctrl+O` while a dialog is up must leave the first dialog's future alone.
///
/// The future here resolves on its *second* poll, which is what makes the claim
/// checkable from outside the app: if the second `Ctrl+O` had replaced it, the file
/// would never arrive, because the replacement would be a portal dialog that nothing
/// in a headless run can answer.
#[test]
fn a_second_ctrl_o_leaves_the_open_dialog_alone() {
    let mut s = Sheet::new();
    let file = Fixture::new("survivor.csv", "survivor\n");
    s.app
        .open_dialog
        .set_pending(OnNthPoll::new(2, file.path.clone()));

    s.pass(Vec::new());
    assert!(
        s.app.open_dialog.is_open(),
        "precondition: the dialog is pending"
    );

    s.ctrl_o();
    assert!(s.app.open_dialog.is_open());

    s.pass(Vec::new());
    assert_eq!(
        s.value(A1),
        CellValue::Text("survivor".to_owned()),
        "the future the app was holding must be the one that resolved"
    );
    assert_eq!(s.app.path.as_deref(), Some(file.path.as_path()));
}

/// The chord is read from the keyboard, not from the field. It has to be: the formula
/// bar is the app's only input surface, so a `Ctrl+O` pressed while typing is a chord
/// between two of the app's own surfaces and must not cost a character.
///
/// This stops on the frame the chord lands, because the next one would poll the
/// portal future — see the rule in the file's docs. What that leaves unasserted is
/// that typing *after* a `Ctrl+O` still reaches the field, which nothing here can
/// reach: by then a real dialog is up. Nothing in `formula_bar` reads `Ctrl`, so
/// there is no chord in the field for the app's own shortcut to collide with.
#[test]
fn ctrl_o_does_not_disturb_a_half_typed_value() {
    let mut s = Sheet::new();
    type_into(&mut s, "rent");
    assert_eq!(s.app.bar.text(), "rent");
    s.ctrl_o();
    assert_eq!(
        s.app.bar.text(),
        "rent",
        "Ctrl+O must not have inserted, deleted or reloaded anything"
    );
    assert_eq!(s.value(A1), CellValue::Text("rent".to_owned()));
}

// --- The answer ---

/// The chosen file is loaded and becomes the file the window is showing.
#[test]
fn a_chosen_file_is_loaded_into_the_sheet() {
    let mut s = Sheet::new();
    let file = Fixture::new("chosen.csv", "Name,Value\nRent,1200\n");
    choose(&mut s, &file.path);
    s.pass(Vec::new());

    assert_eq!(
        s.value(CellRef::new(0, 0, 0)),
        CellValue::Text("Name".into())
    );
    assert_eq!(s.value(CellRef::new(0, 1, 1)), CellValue::Number(1200.0));
    assert_eq!(s.app.path.as_deref(), Some(file.path.as_path()));
    assert!(s.app.error.is_none());
}

/// The first row is data, exactly as it is for the reader itself: no header row is
/// consumed, so nothing in the file is dropped on the way in.
#[test]
fn a_chosen_files_first_row_is_data() {
    let mut s = Sheet::new();
    let file = Fixture::new("no-header.csv", "Rent,1200\n");
    choose(&mut s, &file.path);
    s.pass(Vec::new());
    assert_eq!(s.value(A1), CellValue::Text("Rent".into()));
}

/// A file that cannot be read is an error the user can read, not a panic, and it does not
/// become the file that is open: the title keeps naming whatever was open before.
#[test]
fn a_file_that_cannot_be_read_sets_an_error() {
    let mut s = Sheet::new();
    let file = Fixture::missing("gone.csv");
    choose(&mut s, &file.path);
    s.pass(Vec::new());

    let err = s.app.error.expect("a missing file must be reported");
    assert!(err.contains(&file.path.display().to_string()), "{err}");
    assert!(
        s.app.path.is_none(),
        "a file that was never read is not the file that is open"
    );
}

/// A bad file must not also clear what is already on screen.
#[test]
fn a_failed_open_leaves_the_previous_file_in_place() {
    let mut s = Sheet::new();
    let good = Fixture::new("previous.csv", "a\n");
    choose(&mut s, &good.path);
    s.pass(Vec::new());

    let bad = Fixture::missing("bad.csv");
    choose(&mut s, &bad.path);
    s.pass(Vec::new());
    assert!(s.app.error.is_some());
    assert_eq!(s.app.path.as_deref(), Some(good.path.as_path()));
}

/// A dismissed dialog is not a chosen file, so it changes nothing at all.
#[test]
fn a_cancelled_dialog_changes_nothing() {
    let mut s = Sheet::new();
    let file = Fixture::new("cancelled.csv", "a\n");
    choose(&mut s, &file.path);
    s.pass(Vec::new());
    let before = s.app.path.clone();

    s.app.open_dialog.set_pending(std::future::ready(None));
    s.pass(Vec::new());
    assert_eq!(s.app.path, before);
    assert!(s.app.error.is_none());
    assert_eq!(s.value(A1), CellValue::Text("a".into()));
}

// --- The confirmation ---

/// Opening over unsaved work is the app's first destructive act, so it asks. The
/// workbook is untouched and the path is held rather than opened.
#[test]
fn opening_over_unsaved_work_asks_first() {
    let mut s = Sheet::new();
    let file = Fixture::new("dirty.csv", "chosen\n");
    type_into(&mut s, "mine");
    assert!(s.app.wb.dirty(), "precondition: there is unsaved work");

    choose(&mut s, &file.path);
    s.pass(Vec::new());

    assert_eq!(
        s.value(A1),
        CellValue::Text("mine".to_owned()),
        "the chosen file must not have been opened"
    );
    assert!(s.app.path.is_none());
    assert!(s.app.open_dialog.is_staged());
    assert_eq!(s.app.open_dialog.staged(), Some(file.path.as_path()));
}

/// Answering yes opens the staged file and forgets the unsaved work.
#[test]
fn confirming_the_open_loads_the_staged_file() {
    let mut s = Sheet::new();
    let file = Fixture::new("confirmed.csv", "chosen\n");
    type_into(&mut s, "mine");
    choose(&mut s, &file.path);
    s.pass(Vec::new());

    s.app.confirm_open();
    assert_eq!(s.value(A1), CellValue::Text("chosen".into()));
    assert_eq!(s.app.path.as_deref(), Some(file.path.as_path()));
    assert!(!s.app.open_dialog.is_staged());
    assert!(!s.app.open_dialog.is_busy());
}

/// Answering no is the answer that costs nothing: the workbook is as it was, the
/// question is closed, and the open route is free again.
#[test]
fn declining_the_open_keeps_the_workbook() {
    let mut s = Sheet::new();
    let file = Fixture::new("declined.csv", "chosen\n");
    type_into(&mut s, "mine");
    choose(&mut s, &file.path);
    s.pass(Vec::new());

    s.app.decline_open();
    assert_eq!(s.value(A1), CellValue::Text("mine".to_owned()));
    assert!(s.app.path.is_none());
    assert!(!s.app.open_dialog.is_busy());
    assert!(
        s.app.wb.dirty(),
        "declining must not pretend the work was saved"
    );
}

/// A question with nothing staged behind it cannot open anything, so neither answer can
/// discard anything: `take_staged` yields nothing, and the rule that a chosen path is
/// opened without asking or not at all lives in the `dirty` branch of `OmaApp::open`
/// rather than here.
#[test]
fn answering_with_nothing_staged_opens_nothing() {
    let mut s = Sheet::new();
    type_into(&mut s, "mine");
    s.app.confirm_open();
    s.app.decline_open();
    assert_eq!(s.value(A1), CellValue::Text("mine".to_owned()));
    assert!(s.app.path.is_none());
}

/// While a question is on screen the open route is shut, so `Ctrl+O` cannot cover it
/// with a second dialog and leave two questions about one workbook.
#[test]
fn a_confirmation_holds_the_open_route_shut() {
    let mut s = Sheet::new();
    let file = Fixture::new("busy.csv", "chosen\n");
    type_into(&mut s, "mine");
    choose(&mut s, &file.path);
    s.pass(Vec::new());
    assert!(s.app.open_dialog.is_staged());

    s.ctrl_o();
    assert!(
        !s.app.open_dialog.is_open(),
        "a second dialog must not be raised over a question already asked"
    );
    assert_eq!(s.app.open_dialog.staged(), Some(file.path.as_path()));
}

/// An untidy workbook with nothing to lose opens without a question, or the prompt
/// would appear on every open.
#[test]
fn a_clean_workbook_opens_without_asking() {
    let mut s = Sheet::new();
    let file = Fixture::new("clean.csv", "chosen\n");
    choose(&mut s, &file.path);
    s.pass(Vec::new());
    assert_eq!(s.value(A1), CellValue::Text("chosen".into()));
    assert!(!s.app.open_dialog.is_staged());
}

// --- The bar across an open ---

/// Type into A1, open a file over the top, and answer the question.
fn open_over_a_typed_cell(s: &mut Sheet, name: &str) -> Fixture {
    let file = Fixture::new(name, "Name,Value\nRent,1200\n");
    type_into(s, "rent");
    assert_eq!(
        s.app.bar.text(),
        "rent",
        "precondition: the field holds what was typed"
    );
    assert_eq!(s.app.bar.target(), A1, "precondition: bound to A1");

    choose(s, &file.path);
    s.pass(Vec::new());
    assert!(
        s.app.open_dialog.is_staged(),
        "precondition: unsaved work was asked about rather than discarded"
    );
    s.app.confirm_open();
    s.pass(Vec::new());
    file
}

/// The field must be reloaded by the open itself.
///
/// `formula_bar::show` reloads when the `CellRef` it is bound to changes, or when it is
/// untouched — and neither is true across an open. `replace_sheet` keeps the sheet id,
/// the selection keeps its row and column, and the workbook is dirty precisely because
/// the user typed, which is what leaves the field `touched`. So the gate cannot see this
/// event, and the sequence below is four keystrokes from a corrupt file: the grid shows
/// `Name` and the field shows `rent`, and the next `Esc` or character writes `rent`
/// over the header of the file that was just opened.
#[test]
fn opening_a_file_reloads_the_formula_bar() {
    let mut s = Sheet::new();
    let _file = open_over_a_typed_cell(&mut s, "bar-reload.csv");

    assert_eq!(
        s.app.bar.text(),
        "Name",
        "the field must show the cell the open wrote, not what was being typed"
    );
    assert_eq!(
        s.app.bar.target(),
        A1,
        "the open kept the selection, so the field stays on the cell it was on"
    );
    assert!(
        !s.app.wb.dirty(),
        "an opened file matches what is on disk and is not unsaved work"
    );
}

/// `Esc` after an open must not put the old buffer back.
///
/// The bar's `Esc` writes `bar.original` into the bound cell. Before the fix, `original`
/// was still the value the field held *before* the edit that had already been written to
/// the cell — the empty string — so a single keypress cleared the opened file's header
/// and the title flipped to `●` after having claimed to be clean.
#[test]
fn escape_after_an_open_leaves_the_opened_file_alone() {
    let mut s = Sheet::new();
    let _file = open_over_a_typed_cell(&mut s, "bar-escape.csv");

    s.pass(key(Key::Escape));

    assert_eq!(s.app.bar.text(), "Name");
    assert_eq!(
        s.value(A1),
        CellValue::Text("Name".to_owned()),
        "Esc must not restore what was in the field before the file was opened"
    );
    assert!(
        !s.app.wb.dirty(),
        "an Esc that changes nothing must not claim the workbook is edited"
    );
}

/// The other half of the same fault: a keystroke after the open must edit the file's
/// value rather than continue the buffer that was being typed when it arrived.
#[test]
fn typing_after_an_open_edits_the_files_value() {
    let mut s = Sheet::new();
    let _file = open_over_a_typed_cell(&mut s, "bar-typed.csv");

    type_into(&mut s, "!");

    assert_eq!(
        s.value(A1),
        CellValue::Text("Name!".to_owned()),
        "the character has to land on the cell the open put there"
    );
    assert_eq!(
        s.value(CellRef::new(0, 0, 1)),
        CellValue::Text("Value".to_owned()),
        "the character must not have moved the selection and landed in the next column"
    );
}

// --- The title ---

/// Before anything is open the window says what it is.
#[test]
fn the_title_before_a_file_is_open_is_the_app() {
    let s = Sheet::new();
    assert_eq!(s.app.window_title(), "Omasheet");
}

/// Once a file is open the title is its name. The full path would be ninety characters
/// of window manager, and it is still one click away in the dialog the user just came
/// from.
#[test]
fn the_title_is_the_files_name_not_its_path() {
    let mut s = Sheet::new();
    let file = Fixture::new("in-a-deep-directory.csv", "a\n");
    s.app.open_csv(&file.path);
    assert_eq!(s.app.window_title(), file.name());
    assert!(
        !s.app.window_title().contains('/'),
        "the directory must not reach the title bar"
    );
}

/// A change since the last save is marked in the title, where every other application
/// marks it.
#[test]
fn the_title_is_marked_while_the_workbook_is_dirty() {
    let mut s = Sheet::new();
    let file = Fixture::new("dirty-title.csv", "a\n");
    s.app.open_csv(&file.path);
    assert_eq!(s.app.window_title(), file.name(), "precondition: clean");

    type_into(&mut s, "x");
    assert_eq!(s.app.window_title(), format!("\u{25cf} {}", file.name()));
}

/// A workbook with unsaved work and no file behind it is still marked. The dot reports
/// the workbook, not the file.
#[test]
fn the_title_is_marked_before_a_file_is_open_too() {
    let mut s = Sheet::new();
    type_into(&mut s, "x");
    assert_eq!(s.app.window_title(), "\u{25cf} Omasheet");
}

/// A file that has just been opened is not unsaved work. Left marked, every open would
/// show `●` from the instant it landed and the next `Ctrl+O` would ask to discard
/// changes the user never made.
#[test]
fn opening_a_file_leaves_it_unmarked() {
    let mut s = Sheet::new();
    let file = Fixture::new("fresh.csv", "a\n");
    s.app.open_csv(&file.path);
    assert!(
        !s.app.wb.dirty(),
        "a freshly opened file matches what is on disk"
    );
    assert_eq!(s.app.window_title(), file.name());
}

/// A path with no file name in it — the filesystem root, say — must not make the title
/// empty. `Omasheet` is a better answer than a blank title bar.
#[test]
fn a_path_with_no_file_name_falls_back_to_the_app_name() {
    let mut s = Sheet::new();
    s.app.path = Some(std::path::PathBuf::from("/"));
    assert_eq!(s.app.window_title(), "Omasheet");
}

/// The titles this frame sent the window manager, in order.
fn titles_sent(output: &ViewportOutput) -> Vec<&str> {
    output
        .commands
        .iter()
        .filter_map(|c| match c {
            ViewportCommand::Title(t) => Some(t.as_str()),
            _ => None,
        })
        .collect()
}

/// The title is pushed when it changes, and not again while it does not.
///
/// `last_title` is a memo, and a memo rots silently: a window that stops being renamed
/// looks exactly like a window with nothing to rename. The cost of getting this wrong is
/// one `ViewportCommand::Title` per frame to the compositor, sixty a second, to say
/// something that did not change — and the cost of memoising without checking is the title
/// never arriving at all. Both look fine on screen until someone resizes the window.
#[test]
fn the_title_is_pushed_when_it_changes_and_only_then() {
    let mut s = Sheet::new();
    let file = Fixture::new("pushed.csv", "a\n");

    assert!(
        titles_sent(&s.pass(Vec::new())).is_empty(),
        "the first frame already sent `Omasheet`, so an unchanged title must not be sent again"
    );

    choose(&mut s, &file.path);
    assert_eq!(
        titles_sent(&s.pass(Vec::new())),
        vec![file.name()],
        "opening a file changes the title, so the change has to reach the window manager"
    );

    assert!(
        titles_sent(&s.pass(Vec::new())).is_empty(),
        "the title is the same string as last frame, so it must not be sent again"
    );

    // A change the app does not choose the frame for: the keystroke dirties the workbook
    // below the title's branch, so the dirtiness reaches the title on the next frame.
    type_into(&mut s, "x");
    assert_eq!(
        titles_sent(&s.pass(Vec::new())),
        vec![format!("\u{25cf} {}", file.name())],
        "becoming dirty changes the title, and that change has to be sent too"
    );
}

/// The repaint interval exists while a dialog is up and is gone once it is not.
///
/// The interval is the only thing that draws frames while the dialog is open — the
/// future is polled with a no-op waker, so nothing wakes egui when the portal answers.
/// Without it the answer sits in the future until the user happens to move the mouse;
/// with it left in place after the dialog closes the app re-tessellates a grid nobody is
/// watching at 30 Hz for the rest of the session.
#[test]
fn the_polling_interval_exists_only_while_a_dialog_is_up() {
    let mut s = Sheet::new();
    let file = Fixture::new("polled.csv", "a\n");
    // Resolves on the second poll, so the frame that asks for the interval is the last one
    // with a dialog up and the frame after the dialog has gone is the one that must stop
    // asking. Not the portal — a future this test supplies, so nothing here can raise a
    // real dialog.
    s.settle();
    s.app
        .open_dialog
        .set_pending(OnNthPoll::new(2, file.path.clone()));

    let while_open = s.pass(Vec::new()).repaint_delay;
    assert!(
        while_open > std::time::Duration::ZERO,
        "a dialog is up and nothing else will wake egui: {while_open:?}"
    );
    // The app asks for 50 ms and egui takes one predicted frame time off a non-zero
    // `request_repaint_after` (1/60 s by default), so what arrives is below what was asked
    // for. Asserting the ceiling rather than the exact value keeps this test about the
    // interval existing and not about egui's arithmetic, which the comment in `open` is
    // the place to track.
    assert!(
        while_open < std::time::Duration::from_millis(50),
        "the interval must be the one asked for, less egui's predicted frame time: {while_open:?}"
    );
    assert!(s.app.open_dialog.is_open(), "precondition: still pending");

    // Resolving the frame asks for a repaint of its own — there is work a further frame
    // settles — and egui serves that immediate request on the next pass as well, so it
    // takes two frames after the dialog closes before the output can report that nothing
    // is scheduled. What is being pinned is that the interval does not come back: once
    // `is_open` is false nothing asks for a delayed repaint again.
    s.pass(Vec::new());
    s.pass(Vec::new());
    assert_eq!(
        s.pass(Vec::new()).repaint_delay,
        std::time::Duration::MAX,
        "nothing asked for another frame, so the interval must be gone rather than left running"
    );
}
