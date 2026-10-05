//! The formula bar: the application's only text-input surface.
//!
//! One caret-bearing widget for the whole app. Everything a text field has to do —
//! `Backspace`, `Delete`, `Home`/`End`, the arrow keys *within* the text, selecting a
//! range with the mouse — is egui's problem rather than ours, and there is no second
//! editor that could disagree with the first. The grid holds no editor at all: it asks
//! for the keyboard through [`GridState::focus_requested`] and knows nothing else.
//!
//! ## The egui behaviour this rests on
//!
//! All three of these were measured against 0.35.0 rather than read off the docs, and
//! all three are load-bearing:
//!
//! - `return_key(Shift+Enter)` inserts the newline and leaves plain `Enter` *completely*
//!   unhandled. That is what lets `Enter` mean "move down" here: the buffer is
//!   byte-identical after it, so moving cannot cost the user a character.
//! - Focus is not locked by default, so `Tab` surrenders the keyboard instead of typing
//!   a tab character. `TextEdit::code_editor` locks it, so that preset is not used: a
//!   spreadsheet cell must never contain a tab.
//! - A focus request takes effect in the frame it is made, as long as the widget's id is
//!   already known. Requesting focus before laying the field out is therefore what keeps
//!   the keystroke that asked for focus from being swallowed — the alternative loses the
//!   first character of every edit.
//!
//! A multiline `TextEdit` grows to fit its content and *cannot* be shrunk: `desired_rows`
//! and `add_sized` both set a minimum, not a maximum. The cap therefore comes from an
//! enclosing scroll area, which is also what makes the overflow scroll rather than vanish.

use std::borrow::Cow;

use eframe::egui::{
    Id, Key, KeyboardShortcut, Label, Modifiers, Rect, ScrollArea, TextEdit, Ui, UiBuilder, pos2,
};

use crate::ui::grid::{self, HEADER_W};
use crate::workbook::Workbook;
use crate::workbook::cell::{CellRef, CellValue, format_a1};

/// The height of the bar when it holds one line of text. A `TextEdit` is its content plus
/// a four-point margin, so this leaves the field a little air.
pub const MIN_BAR_H: f32 = 24.0;

/// The share of the window the bar may take however long the value is. Past this the field
/// scrolls: a long Lua chunk has to stay readable without eating the grid.
pub const MAX_BAR_FRACTION: f32 = 0.4;

/// Identifies the field across frames. egui derives ids from the `Ui` that owns the
/// widget, not from its rect, so the field keeps its keyboard when the bar changes height
/// — but only if both this salt and the child `Ui` below are fixed.
const FIELD_SALT: &str = "omasheet-formula-bar-field";

/// Which way `Enter` or `Tab` asked the selection to move.
///
/// The bar does not own the selection — the grid does — so it raises the request and the
/// app applies it, which keeps the bar ignorant of how a move is clamped.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Navigate {
    Down,
    Right,
}

/// The formula bar's state.
#[derive(Debug)]
pub struct FormulaBar {
    /// The cell the field is bound to.
    target: CellRef,
    /// The field's text. Owned here rather than read from the model each frame so that a
    /// caret, a selection and a half-typed value all survive a focus change.
    text: String,
    /// What the cell held when this visit began, which is what `Esc` puts back.
    original: String,
    /// Whether the user has typed since the field was loaded. Gates `Esc`, so that `Esc`
    /// on an untouched field is not a change.
    touched: bool,
    /// The height the field needs for its content, carried to the next frame's layout so
    /// the bar can grow. One frame stale, which is invisible.
    height: f32,
    /// The field's widget id, remembered from the last frame it was shown.
    id: Option<Id>,
    /// A focus request made this frame, applied when the field is next laid out.
    focus_pending: bool,
    /// A move `Enter` or `Tab` asked for, waiting for the app to apply it.
    navigate: Option<Navigate>,
}

impl Default for FormulaBar {
    fn default() -> Self {
        Self {
            target: CellRef::new(0, 0, 0),
            text: String::new(),
            original: String::new(),
            touched: false,
            height: MIN_BAR_H,
            id: None,
            focus_pending: false,
            navigate: None,
        }
    }
}

impl FormulaBar {
    pub fn new() -> Self {
        Self::default()
    }

    /// What the field currently shows.
    pub fn text(&self) -> &str {
        &self.text
    }

    /// The cell the field is bound to.
    pub fn target(&self) -> CellRef {
        self.target
    }

    /// Bind the field to `target`, showing what that cell holds.
    pub fn load_from(&mut self, wb: &Workbook, target: CellRef) {
        self.text = wb.value_at(target).to_input_string();
        self.original = self.text.clone();
        self.touched = false;
        self.height = MIN_BAR_H;
        self.target = target;
    }

    /// Ask for the keyboard on the field's behalf.
    ///
    /// Cannot be applied here: the field's id does not exist until it has been shown at
    /// least once. The request is held and applied at the top of the next [`show`], which
    /// is the *same* frame the request was made in — applying it a frame later instead
    /// would swallow the keystroke that asked for focus, and lose the first character of
    /// every edit.
    pub fn request_focus(&mut self) {
        self.focus_pending = true;
    }

    /// Whether the field holds the keyboard.
    ///
    /// Read at the top of a frame, before the field is shown, because
    /// [`crate::app::app::OmaApp::body`] has to know this before drawing the grid: the
    /// grid stands down when the bar owns the keys. `Esc` and `Tab` are cleared by egui
    /// before layout runs, so this reports what the field owned when the keys arrived.
    pub fn focused(&self, ui: &Ui) -> bool {
        self.id.is_some_and(|id| ui.memory(|m| m.has_focus(id)))
    }

    /// The move `Enter` or `Tab` asked for, taken once.
    pub fn take_navigate(&mut self) -> Option<Navigate> {
        self.navigate.take()
    }

    /// The height the bar reserves in the layout.
    ///
    /// `show` clamps this against the window, so the value is one frame stale after a
    /// paste long enough to reach the cap: the layout hands the grid the taller of the two
    /// for that frame and the bar draws short. Not observable.
    pub fn min_height(&self) -> f32 {
        self.height.max(MIN_BAR_H)
    }
}

/// Write `text` into `at`, or clear the cell if it holds nothing.
///
/// The empty case clears rather than stores: `set_cell` would extend the used range and
/// mark the workbook dirty, so backspacing a cell down to nothing would claim unsaved
/// changes where none were made. `clear_cell` is already no-op safe for an absent cell.
fn write(wb: &mut Workbook, at: CellRef, text: &str) {
    match CellValue::from_text(&cell_text(text)) {
        CellValue::Empty => wb.clear_cell(at),
        value => wb.set_cell(at, value),
    }
}

/// The value a buffer becomes, with the characters that cannot survive a round trip
/// dropped.
///
/// A cell value is one line of TSV, so a tab inside one silently becomes a column break
/// on the way through the system clipboard. No keystroke can produce a tab — `Tab` is
/// claimed by the app — but a paste can, and it arrives in the buffer whether or not the
/// model can hold it.
fn cell_text(text: &str) -> Cow<'_, str> {
    if text.contains(['\t', '\r']) {
        Cow::Owned(text.chars().filter(|c| *c != '\t' && *c != '\r').collect())
    } else {
        Cow::Borrowed(text)
    }
}

/// Draw the bar and apply everything the user did in it.
pub fn show(bar: &mut FormulaBar, wb: &mut Workbook, target: CellRef, ui: &mut Ui) {
    // An untouched field is a mirror of the model, so a change made anywhere else — a
    // `Delete` on the grid, a file opened over the top — has to reach it. Once the user
    // has typed, it is theirs: reloading over a half-typed value would throw it away,
    // which is what `Esc` is for. The comparison costs one small `String` per frame
    // against a grid that paints thousands of cells in the same frame.
    if bar.target != target
        || (!bar.touched && bar.original != wb.value_at(target).to_input_string())
    {
        bar.load_from(wb, target);
    }

    // Taken before the field is laid out, because `Enter` and `Tab` are read after it: a
    // keystroke only moves the selection if the field owned the keyboard when it arrived.
    let owned_input = bar.focused(ui);
    let cap = (ui.ctx().input(|i| i.viewport_rect().height()) * MAX_BAR_FRACTION).max(MIN_BAR_H);

    // Read before anything is applied: egui drops the keyboard on `Esc` before layout
    // runs, so this reports what the user pressed, not what the field now owns.
    let escape = ui.input(|i| i.key_pressed(Key::Escape));

    if bar.focus_pending {
        if escape {
            // `Esc` means stop typing, so a request left over from the previous frame —
            // from an `Enter` or a `Tab` that moved the caret — must not take the
            // keyboard back on the very frame the user let it go.
            bar.focus_pending = false;
        } else {
            if let Some(id) = bar.id {
                // Applied before the widget is shown, so this frame's keystroke reaches
                // the field rather than the gap between the two.
                ui.memory_mut(|m| m.request_focus(id));
            }
            // A printable character is the one gesture that means "start from nothing":
            // `F2` and a double click mean "edit what is here", and both arrive on a
            // frame with no text in it. Without this, typing over a selected cell would
            // append at the end of its value instead of replacing it, which is not what
            // typing over a cell means anywhere else.
            if grid::typed_printable(&grid::read_text(ui)) {
                bar.text.clear();
                bar.touched = false;
            }
        }
        bar.focus_pending = false;
    }

    let reserved = bar.min_height().min(cap);
    let mut field = None;
    let text = &mut bar.text;
    ui.horizontal_top(|ui| {
        // The name box is as wide as the row-number gutter, so the two line up down the
        // left edge of the window.
        ui.add_sized(
            [HEADER_W, MIN_BAR_H],
            Label::new(format_a1(target.row, target.col)),
        );
        let rect = ui.available_rect_before_wrap();
        let mut field_ui = ui.new_child(
            UiBuilder::new()
                .id_salt("formula-bar-field")
                .max_rect(Rect::from_min_max(pos2(rect.left(), rect.top()), rect.max)),
        );
        // The cap lives here: a multiline `TextEdit` cannot be given a maximum, so the
        // scroll area is what bounds it, and what makes the overflow scroll.
        ScrollArea::vertical()
            .max_height(reserved)
            .auto_shrink([false, false])
            .show(&mut field_ui, |ui| {
                let out = TextEdit::multiline(text)
                    .id_salt(FIELD_SALT)
                    .desired_width(f32::INFINITY)
                    // A floor, not a ceiling: without it a multiline field reserves four
                    // rows even when it holds one line of text.
                    .desired_rows(1)
                    .return_key(KeyboardShortcut::new(Modifiers::SHIFT, Key::Enter))
                    .show(ui);
                field = Some((
                    out.response.id,
                    out.response.rect.height(),
                    out.response.changed(),
                ));
            });
    });

    let Some((id, wanted, changed)) = field else {
        return;
    };
    bar.id = Some(id);

    if changed {
        bar.touched = true;
        // Every keystroke writes the cell. There is no pending commit, so
        // `Enter` and `Tab` only move, and nothing can be lost by a crash between
        // typing and confirming.
        write(wb, target, &bar.text);
    }
    bar.height = wanted.clamp(MIN_BAR_H, cap);

    // Read after the field, because the widget leaves both keys unhandled by construction —
    // which is the whole reason it is configured that way. Shift is checked explicitly:
    // `Shift+Enter` and `Shift+Tab` are the same physical keys, and `key_pressed` says
    // only that the key went down. Claiming the shifted pair as ours is also what stops
    // `Shift+Tab` reaching the widget, where it would dedent the line instead.
    if owned_input {
        let (enter, tab, shift) = ui.input(|i| {
            (
                i.key_pressed(Key::Enter),
                i.key_pressed(Key::Tab),
                i.modifiers.shift,
            )
        });
        if !shift && (enter || tab) {
            bar.navigate = Some(if tab { Navigate::Right } else { Navigate::Down });
            // Entry continues in the next cell. `Tab` surrendered the keyboard to reach
            // egui's focus movement, so it has to be asked back; `Enter` never gave it
            // up, and asking again is harmless.
            bar.focus_pending = true;
            ui.ctx().request_repaint();
        }
    }

    // Not gated on focus: egui drops the keyboard on `Esc` before layout runs, so a field
    // that *was* focused reads as unfocused by the time this runs.
    if escape && bar.touched {
        bar.text = bar.original.clone();
        write(wb, target, &bar.text);
        bar.touched = false;
        // egui has already let the keyboard go; taking it back would defeat the point of
        // pressing `Esc`.
        bar.focus_pending = false;
        ui.ctx().request_repaint();
    }
}
