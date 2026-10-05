//! The formula bar, driven through `OmaApp::body`.
//!
//! Every test here shares one `egui::Context` across passes, which is what makes the
//! interesting claims testable at all: egui's focus lives in the `Context`, so a fresh
//! context per pass means a widget can never be focused on the frame after the one that
//! asked for focus, and no keystroke could ever be shown to reach the field. Sharing one
//! context is also what the running app does — there is one `Context` for the lifetime
//! of the process — so this is the real arrangement rather than a test-only fiction.
//!
//! The pointer is the exception, as everywhere in this repo: `Context::run_ui` does not
//! simulate clicks. "Clicking a cell selects it and focuses the bar" is therefore covered
//! by no test here, and was not verified by hand either — the shared Wayland session this
//! was built on makes synthetic input unsafe. Treat it as unverified.

use egui::{Event, Key, Modifiers, RawInput, Rect, Vec2, pos2, vec2};
use omasheet::app::app::OmaApp;
use omasheet::ui::formula_bar::{MAX_BAR_FRACTION, MIN_BAR_H};
use omasheet::workbook::cell::{CellRef, CellValue};

const A1: CellRef = CellRef {
    sheet: 0,
    row: 0,
    col: 0,
};

/// A key-down event for `k`.
fn key(k: Key) -> Vec<Event> {
    key_with(k, Modifiers::default())
}

/// A key-down event for `k` with `modifiers` held.
///
/// Both are set, and both are needed: `RawInput.modifiers` is what the grid reads, but
/// egui's `TextEdit` matches its configured `return_key` against the modifiers carried on
/// the `Event::Key` itself, so a test that only set the input-level modifiers would find
/// `Shift+Enter` doing nothing.
fn key_with(k: Key, modifiers: Modifiers) -> Vec<Event> {
    vec![Event::Key {
        key: k,
        physical_key: Some(k),
        pressed: true,
        repeat: false,
        modifiers,
    }]
}

/// A text-input event, as produced by typing one character.
fn type_text(s: &str) -> Vec<Event> {
    vec![Event::Text(s.to_owned())]
}

fn shift() -> Modifiers {
    Modifiers {
        shift: true,
        ..Default::default()
    }
}

fn screen() -> Rect {
    Rect::from_min_size(pos2(0.0, 0.0), vec2(1100.0, 700.0))
}

/// The app, the context it runs in, and the size of the window it runs in.
struct Bar {
    ctx: egui::Context,
    app: OmaApp,
    size: Vec2,
}

impl Bar {
    /// An app that has already drawn one frame.
    ///
    /// The bar's widget id does not exist until the field has been shown once, and a
    /// focus request can only be applied to an id that already exists. A real app always
    /// has a frame before the user's first keystroke, so every test starts from the same
    /// place rather than from a state the app can never be in.
    fn new() -> Self {
        Self::with_size(screen().size())
    }

    /// As `new`, but in a window of another size.
    fn with_size(size: Vec2) -> Self {
        let mut bar = Bar {
            ctx: egui::Context::default(),
            app: OmaApp::default(),
            size,
        };
        bar.pass(Vec::new());
        bar
    }

    fn pass(&mut self, events: Vec<Event>) {
        self.pass_with(events, Modifiers::default());
    }

    fn pass_with(&mut self, events: Vec<Event>, modifiers: Modifiers) {
        let input = RawInput {
            screen_rect: Some(Rect::from_min_size(pos2(0.0, 0.0), self.size)),
            events,
            modifiers,
            ..Default::default()
        };
        let Bar { ctx, app, .. } = self;
        // `body`, not `ui`: the production `eframe::App::ui` needs an `eframe::Frame`
        // that cannot be constructed outside a running eframe event loop.
        let _ = ctx.run_ui(input, |ui| app.body(ui));
    }

    /// The cell's value as the formula bar would show it.
    fn field(&self) -> &str {
        self.app.bar.text()
    }

    /// The id egui says holds the keyboard, or `None` if nothing does.
    ///
    /// The field's own id is not published, so the assertions built on this ask whether the
    /// *same* widget still has it, which is what a focus theft looks like from here: egui
    /// moves focus to another widget rather than dropping it.
    fn focused(&self) -> Option<egui::Id> {
        self.ctx.memory(|m| m.focused())
    }

    fn value(&self, at: CellRef) -> CellValue {
        self.app.wb.value_at(at)
    }
}

/// Selecting a cell shows its value in the field, in the frame the selection moves.
#[test]
fn selecting_a_cell_loads_its_value_into_the_field() {
    let mut h = Bar::new();
    h.app
        .wb
        .set_cell(CellRef::new(0, 0, 1), CellValue::Text("rent".to_owned()));
    h.pass(key(Key::ArrowRight));
    assert_eq!(h.app.grid.selection.active, CellRef::new(0, 0, 1));
    assert_eq!(h.field(), "rent");
}

/// A number is shown as typed rather than in its display form, so editing it does not
/// quietly reformat it.
#[test]
fn the_field_shows_a_number_as_it_was_typed() {
    let mut h = Bar::new();
    h.app.wb.set_cell(A1, CellValue::Number(1200.5));
    h.pass(Vec::new());
    assert_eq!(h.field(), "1200.5");
}

/// The load is by identity, not by value: coming back to a cell restores what the user
/// typed there, including text that the grid does not display. `Esc` is in the middle
/// because that is the only way back to navigating — the arrow keys belong to the field
/// while it holds the keyboard.
#[test]
fn revisiting_a_cell_restores_what_was_typed_into_it() {
    let mut h = Bar::new();
    h.pass(type_text("00123"));
    h.pass(key(Key::Tab));
    h.pass(key(Key::Escape));
    h.pass(key(Key::ArrowLeft));
    assert_eq!(h.app.grid.selection.active, A1);
    assert_eq!(h.field(), "00123");
    assert_eq!(h.value(A1), CellValue::Text("00123".to_owned()));
}

/// Typing over a selected cell replaces it rather than appending to it. Every spreadsheet
/// does this, and it is the one thing the caret's default position would otherwise get
/// wrong: the field loads the cell, and the keystroke that focused it starts again.
#[test]
fn typing_over_a_selected_cell_replaces_it() {
    let mut h = Bar::new();
    h.app.wb.set_cell(A1, CellValue::Text("rent".to_owned()));
    h.pass(Vec::new());
    assert_eq!(h.field(), "rent");
    h.pass(type_text("5"));
    assert_eq!(h.field(), "5");
    assert_eq!(h.value(A1), CellValue::Number(5.0));
}

/// The whole point of dropping the in-cell editor: there is no commit step, so the cell
/// holds what was typed on the very keystroke that typed it.
#[test]
fn typing_writes_the_cell_on_the_same_keystroke() {
    let mut h = Bar::new();
    h.pass(type_text("7"));
    assert_eq!(h.value(A1), CellValue::Number(7.0));
    assert_eq!(h.field(), "7");
}

/// Two characters in two frames is two model writes, not one commit at the end.
#[test]
fn every_keystroke_writes_the_cell_again() {
    let mut h = Bar::new();
    h.pass(type_text("h"));
    assert_eq!(h.value(A1), CellValue::Text("h".to_owned()));
    h.pass(type_text("i"));
    assert_eq!(h.value(A1), CellValue::Text("hi".to_owned()));
}

/// Text is stored exactly as typed, through the same rule the CSV reader uses.
#[test]
fn a_typed_leading_zero_stays_text() {
    let mut h = Bar::new();
    for c in "00123".chars() {
        h.pass(type_text(&c.to_string()));
    }
    assert_eq!(h.value(A1), CellValue::Text("00123".to_owned()));
}

/// `Enter` moves, and only moves: the buffer is byte-identical afterwards, which is what
/// makes the cell the caret leaves still hold what was typed into it.
#[test]
fn enter_moves_down_without_touching_the_buffer() {
    let mut h = Bar::new();
    h.pass(type_text("7"));
    h.pass(key(Key::Enter));
    assert_eq!(h.app.grid.selection.active, CellRef::new(0, 1, 0));
    h.pass(Vec::new());
    assert_eq!(
        h.field(),
        "",
        "the field must have reloaded from the new cell"
    );
    assert_eq!(h.value(A1), CellValue::Number(7.0));
}

/// `Tab` moves right and inserts nothing: with focus not locked, egui surrenders focus
/// rather than typing a tab character, which is what keeps a tab out of cell values.
#[test]
fn tab_moves_right_and_inserts_no_tab_character() {
    let mut h = Bar::new();
    h.pass(type_text("7"));
    h.pass(key(Key::Tab));
    assert_eq!(h.app.grid.selection.active, CellRef::new(0, 0, 1));
    h.pass(Vec::new());
    assert_eq!(h.field(), "");
    assert_eq!(h.value(A1), CellValue::Number(7.0));
}

/// The point of moving `Tab` and then re-focusing the field: entry across a row keeps
/// typing in the bar rather than dropping the keystroke on the floor.
#[test]
fn typing_a_tab_b_lands_two_adjacent_cells() {
    let mut h = Bar::new();
    h.pass(type_text("a"));
    h.pass(key(Key::Tab));
    h.pass(type_text("b"));
    assert_eq!(
        h.value(CellRef::new(0, 0, 0)),
        CellValue::Text("a".to_owned())
    );
    assert_eq!(
        h.value(CellRef::new(0, 0, 1)),
        CellValue::Text("b".to_owned())
    );
}

/// `Esc` restores the snapshot taken when the field was first touched in this visit, so
/// a half-typed value never reaches the model.
#[test]
fn escape_restores_the_pre_edit_snapshot() {
    let mut h = Bar::new();
    h.app
        .wb
        .set_cell(A1, CellValue::Text("original".to_owned()));
    h.pass(Vec::new());
    h.pass(type_text("clobbered"));
    assert_eq!(h.value(A1), CellValue::Text("clobbered".to_owned()));
    h.pass(key(Key::Escape));
    assert_eq!(h.value(A1), CellValue::Text("original".to_owned()));
    assert_eq!(h.field(), "original");
}

/// `Esc` with nothing typed is not a change, and must not claim one: opening the field on
/// a populated cell and pressing `Esc` leaves the workbook clean.
#[test]
fn escape_on_an_untouched_field_is_a_no_op() {
    let mut h = Bar::new();
    h.app
        .wb
        .set_cell(A1, CellValue::Text("original".to_owned()));
    h.pass(Vec::new());
    h.app.wb.clear_dirty();
    h.pass(key(Key::Escape));
    assert_eq!(h.value(A1), CellValue::Text("original".to_owned()));
    assert!(!h.app.wb.dirty());
    assert_eq!(h.field(), "original");
}

/// The grid's focus request is discharged in the frame that raises it. Left standing, it
/// would take the keyboard back on the frame after `Esc` — the user asking to stop typing
/// and the app refusing.
#[test]
fn a_focus_request_does_not_outlive_the_frame_that_raised_it() {
    let mut h = Bar::new();
    h.pass(type_text("7"));
    h.pass(key(Key::Escape));
    h.pass(Vec::new());
    h.pass(key(Key::ArrowDown));
    assert_eq!(
        h.app.grid.selection.active,
        CellRef::new(0, 1, 0),
        "the field must not have taken the keyboard back and eaten the arrow key"
    );
}

/// `Esc` drops focus, which is what returns the app to navigating: the next arrow key
/// moves the selection again rather than moving a caret.
#[test]
fn escape_returns_the_app_to_navigating() {
    let mut h = Bar::new();
    h.pass(type_text("7"));
    h.pass(key(Key::Escape));
    h.pass(key(Key::ArrowDown));
    assert_eq!(h.app.grid.selection.active, CellRef::new(0, 1, 0));
}

/// While the field is focused it owns the keyboard. Arrow keys move the caret, and
/// `Backspace` edits the text; neither also navigates or clears the cell, or one keypress
/// would do two things at once.
#[test]
fn the_field_owns_the_keyboard_while_it_is_focused() {
    let mut h = Bar::new();
    h.pass(type_text("ab"));
    h.pass(key(Key::ArrowDown));
    assert_eq!(
        h.app.grid.selection.active, A1,
        "an arrow key inside the field must not also move the selection"
    );
    h.pass(key(Key::Backspace));
    assert_eq!(h.field(), "a");
    assert_eq!(
        h.value(A1),
        CellValue::Text("a".to_owned()),
        "Backspace inside the field must not also clear the cell"
    );
}

/// With the field not focused the grid has the keyboard back, and `Delete` clears the
/// selection as it always did.
#[test]
fn delete_clears_the_cell_while_the_field_is_not_focused() {
    let mut h = Bar::new();
    h.app.wb.set_cell(A1, CellValue::Text("rent".to_owned()));
    h.pass(Vec::new());
    h.app.wb.clear_dirty();
    h.pass(key(Key::Delete));
    assert_eq!(h.value(A1), CellValue::Empty);
    assert!(h.app.wb.dirty());
}

/// `Shift+Enter` inserts a newline, and because the bar now owns the keyboard it does not
/// also move the selection — one chord, one meaning.
#[test]
fn shift_enter_inserts_a_newline_and_does_not_move() {
    let mut h = Bar::new();
    h.pass(type_text("one"));
    h.pass_with(key_with(Key::Enter, shift()), shift());
    h.pass(type_text("two"));
    assert_eq!(h.field(), "one\ntwo");
    assert_eq!(h.value(A1), CellValue::Text("one\ntwo".to_owned()));
    assert_eq!(
        h.app.grid.selection.active, A1,
        "Shift+Enter must not also move the selection"
    );
}

/// The bar grows to fit what it holds, so a multi-line value stays readable.
#[test]
fn a_multi_line_value_grows_the_bar() {
    let mut h = Bar::new();
    assert_eq!(
        h.app.bar.min_height(),
        MIN_BAR_H,
        "precondition: a one-row bar"
    );
    h.pass(type_text("one"));
    h.pass_with(key_with(Key::Enter, shift()), shift());
    h.pass(type_text("two"));
    assert!(
        h.app.bar.min_height() > MIN_BAR_H,
        "a two-line value left the bar at {}pt",
        h.app.bar.min_height()
    );
}

/// Past the cap the bar stops growing and scrolls. The value is not truncated: the whole
/// chunk is still there, it is just no longer all on screen.
#[test]
fn the_bar_grows_to_its_cap_and_no_further() {
    let mut h = Bar::new();
    // The cap is a share of the panel the bar shares with the grid, which fills the window
    // today. Deriving it from the window here is the same number only for as long as that
    // is true — and if it stops being, the half-point assertion below says so rather than
    // quietly measuring the wrong thing.
    let cap = MAX_BAR_FRACTION * screen().height();
    h.pass(type_text("x"));
    for _ in 0..40 {
        h.pass_with(key_with(Key::Enter, shift()), shift());
        h.pass(type_text("x"));
    }
    // Within half a point rather than equal: the cap only *reaches* the window by way of
    // `clamp` handing back a bound verbatim, and that is an implementation detail of a
    // float comparison rather than the claim being made here.
    assert!(
        (h.app.bar.min_height() - cap).abs() < 0.5,
        "the bar stopped at {}pt, not at its cap of {cap}pt",
        h.app.bar.min_height()
    );
    assert_eq!(
        h.field().lines().count(),
        41,
        "the value must be kept whole, not truncated to what fits"
    );
}

/// `Tab` is claimed by the app rather than by the widget, which is why no cell value can
/// contain a tab character from the keyboard. Paste is the other way in, and the field
/// drops it there too: a tab in a cell value would silently corrupt the TSV the system
/// clipboard round-trips through.
#[test]
fn a_pasted_tab_never_reaches_a_cell_value() {
    let mut h = Bar::new();
    h.pass(type_text("x"));
    h.pass(vec![Event::Paste("a\tb".to_owned())]);
    assert_eq!(h.value(A1), CellValue::Text("xab".to_owned()));
}

/// A cell cleared from the bar shows as an empty field, not as the text `Empty`.
#[test]
fn emptying_the_field_clears_the_cell_rather_than_storing_an_empty_one() {
    let mut h = Bar::new();
    h.app.wb.set_cell(A1, CellValue::Text("rent".to_owned()));
    h.pass(Vec::new());
    h.pass(key(Key::F2));
    h.pass_with(key(Key::Backspace), Modifiers::default());
    h.pass(key(Key::Backspace));
    h.pass(key(Key::Backspace));
    h.pass(key(Key::Backspace));
    assert_eq!(h.value(A1), CellValue::Empty);
    assert!(
        h.app.wb.cell(A1).is_none(),
        "no phantom cell should be left behind"
    );
    assert!(
        h.app.wb.active().used().is_none(),
        "the used range must shrink back with the last cell"
    );
}

/// `F2` is the other way in: it focuses the bar with the cell already loaded, which is how
/// an existing value gets edited rather than replaced.
#[test]
fn f2_focuses_the_field_seeded_with_the_current_value() {
    let mut h = Bar::new();
    h.app.wb.set_cell(A1, CellValue::Text("rent".to_owned()));
    h.pass(Vec::new());
    h.pass(key(Key::F2));
    h.pass(type_text("!"));
    assert_eq!(h.field(), "rent!");
    assert_eq!(h.value(A1), CellValue::Text("rent!".to_owned()));
}

/// An arrow key must move the caret, not the keyboard.
///
/// The window is one keystroke wide and it is real. egui hands over focus with its
/// *default* event filter, which surrenders focus on the arrow keys, and a field cannot
/// install a filter of its own until it has held focus on an earlier frame — so on the
/// frame after a focus request the field is focused but unclaimed, and egui reads the
/// arrow as a `FocusDirection` at the top of that frame and hands focus to whatever
/// widget it finds in that direction at the bottom. `Tab` opens the same window, since it
/// surrenders focus by design. `Enter` is safe: it is read after the field has been shown,
/// by which time the field's own filter is in place.
///
/// The window here is 4,000 x 6,000, and that is the point rather than a quirk. The grid's
/// focusable rect is the size of the whole sheet rather than of the window, so in an
/// ordinary window it swallows the field whole, the directional search finds nothing, and
/// the bug hides — which is exactly why this has to be pinned where the search has
/// somewhere to go.
#[test]
fn an_arrow_key_does_not_take_the_keyboard_out_of_the_field() {
    let mut h = Bar::with_size(vec2(4_000.0, 6_000.0));
    h.pass(type_text("a"));
    let field = h.focused();
    assert!(
        field.is_some(),
        "precondition: typing on the grid focused the bar"
    );

    // The frame focus arrives on, and the frame after it: the window is these two.
    h.pass(key(Key::ArrowUp));
    assert_eq!(
        h.focused(),
        field,
        "the arrow key moved the keyboard instead of the caret"
    );
    h.pass(key(Key::ArrowUp));
    assert_eq!(
        h.focused(),
        field,
        "and the next one found a widget to hand it to"
    );

    // The keystroke that would have been swallowed if focus had gone is what proves the
    // caret moved instead: two `ArrowUp`s put it at the front of the value.
    h.pass(type_text("x"));
    assert_eq!(h.field(), "xa");
}

/// `F2`-then-`Enter` on an untouched cell must leave the workbook clean. Nothing was typed,
/// so nothing was written: with every keystroke reaching the model live there is no commit
/// step here at all, so the "modified" marker has nothing to switch on.
///
/// These two live here rather than in `tests/grid_keys.rs`, where they started, because the
/// field is what they are about. `F2` is only visible as a gesture when focus survives into
/// the next frame, and focus lives in the `Context` — a per-pass context, which is what
/// `tests/grid_keys.rs` drives the app with, throws that away.
#[test]
fn f2_then_enter_on_an_untouched_cell_leaves_the_workbook_clean() {
    let mut h = Bar::new();
    h.pass(key(Key::F2));
    h.pass(key(Key::Enter));
    assert!(h.app.wb.cell(A1).is_none());
    assert!(h.app.wb.active().used().is_none());
    assert!(!h.app.wb.dirty());
}

/// The same two keys over a populated cell leave it exactly as it was. Under the old
/// in-cell editor this rewrote the text it had just seeded the buffer with, marking the
/// workbook dirty for a gesture that changed nothing; the pin survives because that is
/// still the easy mistake to make.
#[test]
fn f2_then_enter_on_a_populated_cell_changes_nothing() {
    let mut h = Bar::new();
    h.app.wb.set_cell(A1, CellValue::Text("rent".to_owned()));
    h.pass(Vec::new());
    h.app.wb.clear_dirty();
    h.pass(key(Key::F2));
    h.pass(key(Key::Enter));
    assert_eq!(h.value(A1), CellValue::Text("rent".to_owned()));
    assert!(h.app.wb.active().used().is_some());
    assert!(!h.app.wb.dirty());
}
