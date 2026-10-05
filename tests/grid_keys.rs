//! Headless keybinding and virtualisation tests. `egui::Context::run_ui` runs one UI
//! pass with synthesised input and no window, so these are real tests, not
//! descriptions of intent.
//!
//! One link in the input chain is unreachable here: egui's `Context::run_ui` does not
//! simulate pointer input at all (a plain `ui.button` also fails to report `clicked()`
//! under it), so `response.clicked()` and `response.double_clicked()` cannot be exercised.
//! The pointer-to-selection mapping around it is covered directly by the
//! `click_to_selection` tests, but the wiring those two sit on — a click reaching the
//! selection, and a double click handing the keyboard to the formula bar — is **unverified
//! by any test and not verified by hand either**. The shared Wayland session this was
//! built on makes synthetic input unsafe, so it was not exercised in a real window.
//!
//! Typing is not here either: it goes through one widget now, and everything about it
//! lives in `tests/formula_bar.rs`.

use std::time::{Duration, Instant};

use egui::{Event, Key, Modifiers, Pos2, RawInput, Rect, Vec2, pos2, vec2};
use omasheet::app::app::OmaApp;
use omasheet::io::csv_read::read_csv;
use omasheet::ui::grid;
use omasheet::ui::grid::*;
use omasheet::workbook::Workbook;
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
    pass_on(app, &ctx, events, modifiers);
}

/// Drive the grid on its own, with no `OmaApp::body` around it.
///
/// `body` consumes `focus_requested` every frame, which is the wiring rather than the
/// behaviour: the flag's whole job is to be set in here and read out there, so a test that
/// went through `body` could only ever see it false.
fn grid_pass(state: &mut GridState, wb: &mut Workbook, events: Vec<Event>) {
    grid_pass_locked(state, wb, events, false)
}

/// As `grid_pass`, but saying the keyboard belongs to the formula bar.
fn grid_pass_locked(
    state: &mut GridState,
    wb: &mut Workbook,
    events: Vec<Event>,
    input_locked: bool,
) {
    let ctx = egui::Context::default();
    let input = RawInput {
        screen_rect: Some(screen()),
        events,
        modifiers: Modifiers::default(),
        ..Default::default()
    };
    let _ = ctx.run_ui(input, |ui| {
        grid::show(state, wb, input_locked, ui);
    });
}

/// As `pass_with`, but against a context the caller owns, so consecutive passes share one.
///
/// A fresh `Context` per pass builds its font atlas and texture on the first frame. That is
/// start-up cost, and no real app pays it twice, so a *timed* pass must reuse a context
/// that has already had one. That is the *only* difference, and it is not a small one for
/// the formula bar: focus lives in the `Context`, so a pass on a fresh one can never see a
/// widget focused by the pass before it, and a gesture that hands over focus — `F2`, a
/// printable character, `Tab` — has no visible effect at all. A test here that needs to
/// see the bar must not use `pass_with`; it belongs in `tests/formula_bar.rs`, which shares
/// one context for exactly that reason.
fn pass_on(app: &mut OmaApp, ctx: &egui::Context, events: Vec<Event>, modifiers: Modifiers) {
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

// --- The spec's performance gate, split in two. ---

/// One 16 ms frame, the budget the design document sets.
const FRAME_BUDGET: Duration = Duration::from_millis(16);

/// The **load-and-paint** half of the design document's performance gate, which promised
/// that the slice would not ship until a 10,000-cell CSV loads and a full pass completes
/// inside one 16 ms frame. The gate was deferred whole because the promise bundled a
/// *recalc* half into it, and recalc cannot be tested before formulas exist.
///
/// The **recalc** half is the Milestone 2 sibling assertion and lands with the formula
/// plan: the same 10,000-cell fixture, one pass, *full recalculation of every cell*,
/// asserted under the same 16 ms budget. Nothing in this test substitutes for it — there is
/// no recalc to measure yet, and the load below is one-off file-open cost rather than
/// per-frame work, which is precisely why the two halves had to be split.
///
/// All three timings are printed, not just the gated one, because the point of shipping
/// the gate is knowing the headroom rather than only the pass/fail. Run with `--nocapture`.
/// The gate is on the warm pass: that is the per-frame cost a frame budget is about.
#[test]
fn a_ten_thousand_cell_csv_paints_within_one_frame() {
    // 100 x 100 = 10,000 populated cells. The first row is text and the rest numbers, so
    // the pass paints both kinds and exercises number inference on the way in.
    let mut text = String::with_capacity(100 * 100 * 4);
    for row in 0..100u32 {
        for col in 0..100u32 {
            if col > 0 {
                text.push(',');
            }
            if row == 0 {
                text.push_str(&format!("header{col}"));
            } else {
                text.push_str(&(row * 100 + col).to_string());
            }
        }
        text.push('\n');
    }

    let mut app = OmaApp::default();
    let load_start = Instant::now();
    let sheet = read_csv(&text).expect("the generated fixture should parse");
    app.load_sheet(sheet);
    let load_ms = millis(load_start.elapsed());

    // Without this the gate could pass on an accidentally tiny fixture, which would make the
    // timing below meaningless rather than merely fast.
    assert_eq!(
        app.wb.active().iter_cells().count(),
        10_000,
        "the fixture must really hold 10,000 populated cells"
    );

    // Cold: context construction plus first-frame font atlas, i.e. process start-up. Printed
    // but not asserted: no frame in a real app pays it, and it varies about 2x run to run
    // (3.6-8.0 ms observed here) because font rasterisation dominates it. Asserting on it
    // would make the gate flaky without testing anything the frame budget is about.
    let cold_start = Instant::now();
    pass(&mut app, Vec::new());
    let cold_ms = millis(cold_start.elapsed());

    // Warm: the per-frame cost the 16 ms budget is actually about, on a context that has
    // already been through a frame — as every frame after the first in a real app is.
    let ctx = egui::Context::default();
    pass_on(&mut app, &ctx, Vec::new(), Modifiers::default());
    let warm_start = Instant::now();
    pass_on(&mut app, &ctx, Vec::new(), Modifiers::default());
    let warm_ms = millis(warm_start.elapsed());

    eprintln!(
        "10,000-cell fixture: load {load_ms:.3} ms | cold paint {cold_ms:.3} ms | \
         warm paint {warm_ms:.3} ms | budget 16 ms",
    );

    assert!(
        warm_ms < FRAME_BUDGET.as_secs_f64() * 1000.0,
        "a warm paint pass over 10,000 populated cells took {warm_ms:.3} ms, \
         over the 16 ms frame budget"
    );
}

/// The measured milliseconds, at the precision the report quotes.
fn millis(d: Duration) -> f64 {
    d.as_secs_f64() * 1000.0
}

/// Control-only text must not hand the keyboard to the formula bar. egui-winit already
/// filters it at the source, but `Event::Text`'s own contract only asks an integration to
/// withhold text for Enter, and a backend that leaks it must not steal focus.
#[test]
fn a_control_only_text_event_does_not_ask_for_focus() {
    let (mut state, mut wb) = (GridState::new(), Workbook::new());
    grid_pass(&mut state, &mut wb, type_text("\t"));
    assert!(!state.focus_requested);
}

#[test]
fn typing_on_the_grid_asks_for_focus() {
    let (mut state, mut wb) = (GridState::new(), Workbook::new());
    grid_pass(&mut state, &mut wb, type_text("hi"));
    assert!(
        state.focus_requested,
        "a printable character asks for focus"
    );
}

/// The flag is a request, not a mode, so a frame with nothing in it must not raise one.
/// Nothing here clears it — the app does, in the same frame — so this is the grid's half
/// of the contract: it raises a request only for a gesture that means one.
#[test]
fn a_frame_with_nothing_in_it_does_not_ask_for_focus() {
    let (mut state, mut wb) = (GridState::new(), Workbook::new());
    grid_pass(&mut state, &mut wb, Vec::new());
    assert!(!state.focus_requested);
    grid_pass(&mut state, &mut wb, key(Key::ArrowDown));
    assert!(
        !state.focus_requested,
        "navigating is not a request for the keyboard"
    );
}

/// The second way in, and the one that means "edit what is here" rather than "start from
/// nothing". The difference is invisible in the grid, which is why the bar works it out
/// from the frame's own text rather than being told.
#[test]
fn f2_on_the_grid_asks_for_focus() {
    let (mut state, mut wb) = (GridState::new(), Workbook::new());
    grid_pass(&mut state, &mut wb, key(Key::F2));
    assert!(state.focus_requested);
}

/// Typing asks for focus rather than moving the selection, and does not also clear the
/// cell the way `Delete` does: one keystroke, one meaning.
#[test]
fn typing_on_the_grid_neither_navigates_nor_clears() {
    let (mut state, mut wb) = (GridState::new(), Workbook::new());
    let a1 = CellRef::new(0, 0, 0);
    wb.set_cell(a1, CellValue::Text("rent".into()));
    wb.clear_dirty();
    grid_pass(&mut state, &mut wb, type_text("7"));
    assert_eq!(state.selection.active, a1);
    assert_eq!(wb.value_at(a1), CellValue::Text("rent".into()));
    assert!(!wb.dirty());
}

/// `input_locked` says the formula bar holds the keyboard. The grid must then read nothing
/// at all, or one `Backspace` both deletes a character from the field and clears the cell
/// underneath it.
#[test]
fn a_locked_grid_reads_no_keys() {
    let (mut state, mut wb) = (GridState::new(), Workbook::new());
    let a1 = CellRef::new(0, 0, 0);
    wb.set_cell(a1, CellValue::Text("rent".into()));
    grid_pass_locked(
        &mut state,
        &mut wb,
        [
            key(Key::Delete),
            key(Key::ArrowDown),
            key(Key::F2),
            type_text("x"),
        ]
        .concat(),
        true,
    );
    assert_eq!(state.selection.active, a1);
    assert!(!state.focus_requested);
    assert_eq!(wb.value_at(a1), CellValue::Text("rent".into()));
}

// There is no `Shift+Tab` test here. One was, and it asserted nothing: with the field out
// of reach (see `pass_with` above) `grid::show` matched neither its `Delete`/`Backspace`
// arm nor `handle_key`, which declines `Tab` outright, so the test would have passed
// against a build with no formula bar in it at all. The claim it was named for is real —
// the shifted pair is claimed by the app so egui cannot dedent the line — but it is only
// observable with the field present, and lives with the field in `tests/formula_bar.rs`.

#[test]
fn selection_starts_as_a_single_cell() {
    let state = GridState::new();
    assert!(state.selection.is_single());
    assert_eq!(state.selection.range(), CellRange::new(0, 0, 0, 0));
}

#[test]
fn delete_clears_the_active_cell_and_marks_dirty() {
    let mut app = OmaApp::default();
    app.wb
        .set_cell(CellRef::new(0, 0, 0), CellValue::Text("rent".into()));
    app.wb.clear_dirty();
    pass(&mut app, key(Key::Delete));
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Empty);
    assert!(app.wb.dirty());
}

// `parse_input` used to live here as `ui::grid::parse_input` and was tested through this
// file. It is now `CellValue::from_text`, in the model, and is tested in `tests/cell.rs`
// alongside the rule itself. Everything about typing into a cell moved to
// `tests/formula_bar.rs`, which drives the one widget that types now.

/// `Backspace` clears the selection while the formula bar is not holding the keyboard,
/// like `Delete`. While it *is* holding the keyboard the same key deletes a character
/// instead, which `tests/formula_bar.rs` covers.
#[test]
fn backspace_clears_the_active_cell_and_marks_dirty() {
    let mut app = OmaApp::default();
    app.wb
        .set_cell(CellRef::new(0, 0, 0), CellValue::Text("rent".into()));
    app.wb.clear_dirty();
    pass(&mut app, key(Key::Backspace));
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Empty);
    assert!(app.wb.dirty());
}

// --- Cross-sheet selection. Unreachable from today's UI, because there are no sheet tabs
// yet, but `Workbook` can already change the active sheet — so the invariant is pinned
// here, before the tabs arrive to break it. ---

#[test]
fn switching_the_active_sheet_moves_the_selection_with_it() {
    let mut app = OmaApp::default();
    assert!(app.wb.add_sheet("Two"));
    assert_eq!(app.wb.active_id(), 1);
    pass(&mut app, Vec::new());
    assert_eq!(app.grid.selection.active, CellRef::new(1, 0, 0));
    assert_eq!(app.grid.selection.anchor, CellRef::new(1, 0, 0));
}

/// The defect this exists for: before, `selection_cells` acted on the sheet the selection
/// *was* on, so `Delete` after a sheet switch erased the other sheet's cell. The old
/// sheet's data must survive.
#[test]
fn delete_after_a_sheet_switch_clears_the_new_active_sheet_only() {
    let mut app = OmaApp::default();
    app.wb
        .set_cell(CellRef::new(0, 0, 0), CellValue::Text("first".into()));
    assert!(app.wb.add_sheet("Two"));
    app.wb
        .set_cell(CellRef::new(1, 0, 0), CellValue::Text("second".into()));
    app.wb.clear_dirty();

    pass(&mut app, key(Key::Delete));

    assert_eq!(
        app.wb.value_at(CellRef::new(1, 0, 0)),
        CellValue::Empty,
        "Delete must clear the sheet on screen"
    );
    assert_eq!(
        app.wb.value_at(CellRef::new(0, 0, 0)),
        CellValue::Text("first".into()),
        "the other sheet's data must survive"
    );
}

/// `remove_sheet` re-stamps every remaining sheet id, so a `CellRef` held in the selection
/// or in the formula bar's target silently changes which sheet it names. Re-qualifying is
/// what stops that from becoming a write to the wrong sheet.
#[test]
fn removing_a_sheet_requalifies_a_selection_naming_a_later_sheet() {
    let mut app = OmaApp::default();
    assert!(app.wb.add_sheet("Two"));
    assert!(app.wb.add_sheet("Three"));
    // Put the selection on the sheet that is about to be removed.
    pass(&mut app, key(Key::ArrowDown));
    app.retarget_selection_to_active_sheet();
    assert_eq!(app.grid.selection.active.sheet, 2);

    assert!(app.wb.remove_sheet(0));
    assert_eq!(app.wb.active_id(), 0);
    assert_eq!(app.wb.sheet_name(0), "Two", "ids were re-stamped");

    pass(&mut app, Vec::new());
    assert_eq!(
        app.grid.selection.active,
        CellRef::new(0, 1, 0),
        "the selection must name the active sheet, not the old id, and keep its row"
    );
    assert_eq!(
        app.bar.target().sheet,
        0,
        "the formula bar must not write into a re-stamped sheet id"
    );
}

/// A shift-extended selection has two corners, and both are sheet-qualified. Retargeting
/// only the active one would leave `Selection::range` straddling two sheets.
#[test]
fn retargeting_requalifies_the_anchor_as_well_as_the_active_cell() {
    let mut app = OmaApp::default();
    let shift = Modifiers {
        shift: true,
        ..Default::default()
    };
    // Build a rectangle on the second sheet, where the corners genuinely differ.
    assert!(app.wb.add_sheet("Two"));
    pass(&mut app, key(Key::ArrowDown));
    pass_with(&mut app, key(Key::ArrowDown), shift);
    assert!(!app.grid.selection.is_single(), "precondition: a rectangle");
    assert_eq!(app.grid.selection.anchor, CellRef::new(1, 1, 0));
    assert_eq!(app.grid.selection.active, CellRef::new(1, 2, 0));

    app.wb.set_active(0);
    app.retarget_selection_to_active_sheet();

    assert_eq!(
        app.grid.selection.anchor,
        CellRef::new(0, 1, 0),
        "the anchor must be requalified too, not left naming the old sheet"
    );
    assert_eq!(app.grid.selection.active, CellRef::new(0, 2, 0));
    assert_eq!(
        app.grid.selection.range(),
        CellRange::new(1, 0, 2, 0),
        "the rectangle stays the same rectangle"
    );
}

// --- Pointer actions. `Response::clicked` and `double_clicked` cannot be simulated by
// `Context::run_ui`, so these cover the decision and nothing else. The wiring from a
// `Response` to that decision is covered by no test here, and was not checked by hand
// either: the shared Wayland session this was built on makes synthetic input unsafe, so no
// click was ever driven through a real window. Treat it as unverified. ---

#[test]
fn a_single_click_selects_the_cell_under_the_pointer() {
    assert_eq!(
        click_action(pos2(0.0, 0.0), pos2(150.0, 10.0), 0, true, false),
        CellAction::Select(CellRef::new(0, 0, 1))
    );
}

/// A double click selects like any other click. What it *also* means — hand the keyboard
/// over — cannot travel through this decision, because there is nothing here to receive
/// it: the field is somewhere else entirely. That half is raised by the grid from
/// `Response::double_clicked` and is the one part of the pointer chain no headless test
/// can reach.
#[test]
fn a_double_click_selects_the_cell_under_the_pointer() {
    assert_eq!(
        click_action(pos2(0.0, 0.0), pos2(150.0, 10.0), 0, false, true),
        CellAction::Select(CellRef::new(0, 0, 1))
    );
}

/// egui sets `CLICKED` on the frame of a double click too, so both flags can be true at
/// once. Both have to answer the same way, or the second click of a pair would land
/// somewhere else from the first.
#[test]
fn a_double_click_selects_the_same_cell_the_click_it_also_reports_does() {
    assert_eq!(
        click_action(pos2(0.0, 0.0), pos2(150.0, 10.0), 0, true, true),
        CellAction::Select(CellRef::new(0, 0, 1))
    );
}

#[test]
fn a_press_that_is_not_a_click_selects_nothing() {
    assert_eq!(
        click_action(pos2(0.0, 0.0), pos2(150.0, 10.0), 0, false, false),
        CellAction::None
    );
}

#[test]
fn a_click_outside_the_body_selects_nothing() {
    // Above the content origin: in the row-number gutter, which belongs to the header.
    assert_eq!(
        click_action(pos2(0.0, 0.0), pos2(150.0, -5.0), 0, true, true),
        CellAction::None
    );
}

/// Perceived brightness, 0-255. Good enough to compare two greys with each other.
fn luma(c: egui::Color32) -> f32 {
    0.2126 * c.r() as f32 + 0.7152 * c.g() as f32 + 0.0722 * c.b() as f32
}

#[test]
fn the_zebra_stripe_is_visible_against_the_row_beneath_it() {
    for (name, visuals) in [
        ("dark", egui::Visuals::dark()),
        ("light", egui::Visuals::light()),
    ] {
        let (stripe, _) = zebra_and_grid_colours(&visuals);
        let base = visuals.panel_fill;
        // Absolute difference, not "brighter": the stripe moves towards the text colour,
        // which means it darkens a light theme and lightens a dark one.
        let lift = (luma(stripe) - luma(base)).abs();
        assert!(
            lift >= 4.0,
            "{name}: stripe {stripe:?} is only {lift:.1} from the row beneath it {base:?}"
        );
    }
}

#[test]
fn grid_lines_are_clearly_stronger_than_the_zebra_stripe() {
    for (name, visuals) in [
        ("dark", egui::Visuals::dark()),
        ("light", egui::Visuals::light()),
    ] {
        let (stripe, grid) = zebra_and_grid_colours(&visuals);
        let gap = (luma(grid) - luma(stripe)).abs();
        // This is the pair the original complaint was about: borrowed from two unrelated
        // theme slots that happened to be the same grey.
        assert!(
            gap >= 6.0,
            "{name}: grid {grid:?} is only {gap:.1} from the stripe {stripe:?}"
        );
    }
}

#[test]
fn the_stripe_darkens_a_light_theme_rather_than_going_black() {
    // Regression guard. Deriving the stripe from `text_color` alone, which is what this
    // did first, gives near-black on a white background.
    let visuals = egui::Visuals::light();
    let (stripe, _) = zebra_and_grid_colours(&visuals);
    assert!(
        luma(stripe) > 128.0,
        "light theme stripe {stripe:?} went nearly black"
    );
}
