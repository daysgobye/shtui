//! Headless keybinding and virtualisation tests. `egui::Context::run_ui` runs one UI
//! pass with synthesised input and no window, so these are real tests, not
//! descriptions of intent.
//!
//! One link in the input chain is unreachable here: egui's `Context::run_ui` does not
//! simulate pointer input at all (a plain `ui.button` also fails to report `clicked()`
//! under it), so `response.clicked()` cannot be exercised. The pointer-to-selection
//! mapping around it is covered directly by the `click_to_selection` tests.

use egui::{Event, Key, Modifiers, Pos2, RawInput, Rect, Vec2, pos2, vec2};
use omasheet::app::app::OmaApp;
use omasheet::ui::grid::*;
use omasheet::workbook::cell::{CellRange, CellRef, CellValue};

/// A key-down event for `k`.
fn key(k: Key) -> Vec<Event> {
    vec![Event::Key {
        key: k,
        physical_key: Some(k),
        pressed: true,
        repeat: false,
        modifiers: Modifiers::default(),
    }]
}

/// Press `k` `n` times, one key-down per pass.
///
/// One pass per press, not one pass with `n` events: egui rewrites every same-key
/// press after the first in a single frame to `repeat: true`, which the grid ignores
/// (auto-repeat is a later milestone). Batching them would silently press the key once.
fn press(app: &mut OmaApp, k: Key, n: usize) {
    for _ in 0..n {
        pass(app, key(k));
    }
}

/// A text-input event, as produced by typing.
fn type_text(s: &str) -> Vec<Event> {
    vec![Event::Text(s.to_owned())]
}

/// `Rect::from_min_size` is not `const`, so the screen rect is built by a function.
fn screen() -> Rect {
    Rect::from_min_size(Pos2::ZERO, vec2(1100.0, 700.0))
}

/// Drive one UI pass over `app` with `events`, with no window on screen.
fn pass(app: &mut OmaApp, events: Vec<Event>) {
    pass_with(app, events, Modifiers::default())
}

/// As `pass`, but with explicit modifier state.
///
/// `RawInput.modifiers` is what egui treats as authoritative — the `modifiers` field
/// on an individual `Event::Key` is informational, so relying on it would make every
/// modifier test pass vacuously.
fn pass_with(app: &mut OmaApp, events: Vec<Event>, modifiers: Modifiers) {
    let ctx = egui::Context::default();
    let input = RawInput {
        screen_rect: Some(screen()),
        events,
        modifiers,
        ..Default::default()
    };
    let _ = ctx.run_ui(input, |ui| {
        // `body`, not `ui`: the production `eframe::App::ui` needs an `eframe::Frame`
        // that cannot be constructed outside a running eframe event loop.
        app.body(ui);
    });
}

#[test]
fn arrow_down_moves_the_selection() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowDown));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 1, 0));
}

#[test]
fn arrow_right_moves_the_selection() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowRight));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 0, 1));
}

#[test]
fn arrow_keys_do_not_move_the_selection_past_zero() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowUp));
    pass(&mut app, key(Key::ArrowLeft));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 0, 0));
}

#[test]
fn arrow_keys_clamp_to_the_scrollable_extent() {
    // An empty sheet scrolls over MIN_ROWS x MIN_COLS, so row 199 / column 25 is the
    // last reachable cell. Without the clamp the highlight walks off the end of the
    // content, where it is never painted and cannot be scrolled back to.
    let mut app = OmaApp::default();
    press(&mut app, Key::ArrowDown, 300);
    assert_eq!(app.grid.selection.active, CellRef::new(0, 199, 0));
    press(&mut app, Key::ArrowRight, 100);
    assert_eq!(app.grid.selection.active, CellRef::new(0, 199, 25));
}

#[test]
fn home_moves_to_the_first_column() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowRight));
    pass(&mut app, key(Key::ArrowRight));
    pass(&mut app, key(Key::Home));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 0, 0));
}

#[test]
fn arrow_navigation_moves_the_anchor_with_it() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowDown));
    assert_eq!(app.grid.selection.anchor, CellRef::new(0, 1, 0));
    assert!(app.grid.selection.is_single());
}

#[test]
fn shift_arrow_extends_the_selection_without_moving_the_anchor() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowDown));
    let shift = Modifiers {
        shift: true,
        ..Default::default()
    };
    pass_with(&mut app, key(Key::ArrowDown), shift);
    pass_with(&mut app, key(Key::ArrowRight), shift);
    assert_eq!(app.grid.selection.anchor, CellRef::new(0, 1, 0));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 2, 1));
    assert!(!app.grid.selection.is_single());
    assert_eq!(app.grid.selection.range(), CellRange::new(1, 0, 2, 1));
}

#[test]
fn cell_at_pos_maps_a_pointer_back_to_a_cell() {
    // Content coordinates start at zero: the header gutters sit outside the body, so
    // the body's own top-left is A1.
    assert_eq!(
        cell_at_pos(Pos2::ZERO, Pos2::new(COL_W + 4.0, ROW_H + 4.0)),
        Some((1, 1))
    );
}

#[test]
fn cell_at_pos_returns_nothing_outside_the_body() {
    // Above and left of the origin are the frozen header gutters, not cells.
    assert_eq!(cell_at_pos(Pos2::ZERO, Pos2::new(-10.0, 4.0)), None);
    assert_eq!(cell_at_pos(Pos2::ZERO, Pos2::new(4.0, -10.0)), None);
}

#[test]
fn click_to_selection_maps_the_cell_under_the_pointer() {
    let origin = Pos2::new(HEADER_W, HEADER_H);
    assert_eq!(
        click_to_selection(origin, origin + Vec2::new(COL_W + 5.0, ROW_H + 5.0), 0),
        Some(CellRef::new(0, 1, 1))
    );
}

#[test]
fn click_to_selection_uses_the_sheet_it_is_given() {
    // `cell_at_pos` knows nothing about sheets, so the qualification must come from
    // the caller rather than a hardcoded sheet id.
    assert_eq!(
        click_to_selection(Pos2::ZERO, Pos2::new(4.0, 4.0), 3),
        Some(CellRef::new(3, 0, 0))
    );
}

#[test]
fn click_to_selection_returns_nothing_outside_the_body() {
    assert_eq!(
        click_to_selection(Pos2::ZERO, Pos2::new(-1.0, 5.0), 0),
        None
    );
    assert_eq!(
        click_to_selection(Pos2::ZERO, Pos2::new(5.0, -1.0), 0),
        None
    );
}

#[test]
fn visible_window_is_bounded_by_the_viewport_plus_overscan() {
    // A viewport fits about 31 rows and 11 columns; asking for all 5,000 rows is the
    // bug this arithmetic exists to prevent.
    let w = visible_window(screen(), Pos2::ZERO, 5_000, 26);
    assert!(w.last_row < 100, "last_row was {}", w.last_row);
    assert!(w.cell_count() < 10_000, "cell_count was {}", w.cell_count());
}

#[test]
fn visible_window_clamps_to_the_sheet_extent() {
    // A viewport taller and wider than the sheet must not wrap or overrun the extent.
    let huge = Rect::from_min_size(Pos2::ZERO, vec2(4_000.0, 12_000.0));
    let w = visible_window(huge, Pos2::ZERO, 500, 20);
    assert_eq!(w.last_row, 500);
    assert_eq!(w.last_col, 20);
    // A sheet smaller than the viewport clamps to exactly the sheet, with no overscan.
    let tiny = visible_window(screen(), Pos2::ZERO, 2, 3);
    assert_eq!(
        tiny,
        VisibleWindow {
            first_row: 0,
            last_row: 2,
            first_col: 0,
            last_col: 3
        }
    );
}

#[test]
fn visible_window_scrolls_with_the_origin() {
    // The body sits inside the frozen gutters, so the clip is inset by the header size.
    let clip = Rect::from_min_size(pos2(HEADER_W, HEADER_H), vec2(1000.0, 600.0));
    let unscrolled = visible_window(clip, clip.min, 5_000, 26);
    // Scrolling down by 1,000pt moves the content origin above the viewport top.
    let scrolled = visible_window(clip, clip.min - Vec2::new(0.0, 1_000.0), 5_000, 26);
    assert_eq!(unscrolled.first_row, 0);
    assert!(
        scrolled.first_row > 40,
        "first_row was {}",
        scrolled.first_row
    );
    assert!(scrolled.last_row > unscrolled.last_row);
}

#[test]
fn a_value_written_to_the_model_survives_a_ui_pass() {
    let mut app = OmaApp::default();
    app.wb
        .set_cell(CellRef::new(0, 0, 0), CellValue::Text("rent".into()));
    // If the grid panicked while painting, or read the wrong sheet, this fails.
    pass(&mut app, Vec::new());
    assert_eq!(
        app.wb.value_at(CellRef::new(0, 0, 0)),
        CellValue::Text("rent".into())
    );
}

#[test]
fn a_large_sheet_paints_without_error() {
    // Exercises the virtualisation window with far more cells than fit on screen.
    let mut app = OmaApp::default();
    for row in 0..5_000u32 {
        app.wb
            .set_cell(CellRef::new(0, row, 0), CellValue::Number(row as f64));
    }
    for col in 0..20u32 {
        app.wb
            .set_cell(CellRef::new(0, 0, col), CellValue::Text("x".into()));
    }
    pass(&mut app, Vec::new());
    assert_eq!(app.wb.active().row_count(), 5_000);
}

#[test]
fn a_text_event_is_ignored_while_navigating() {
    // Editing lands in Task 6. This pins the behaviour so that task changes it
    // deliberately rather than discovering it as a surprise.
    let mut app = OmaApp::default();
    pass(&mut app, type_text("hi"));
    assert!(app.grid.edit.is_none());
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Empty);
}

#[test]
fn selection_starts_as_a_single_cell() {
    let state = GridState::new();
    assert!(state.selection.is_single());
    assert_eq!(state.selection.range(), CellRange::new(0, 0, 0, 0));
}
