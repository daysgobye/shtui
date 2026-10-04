//! Headless keybinding tests. `egui::Context::run_ui` runs one UI pass with
//! synthesised input and no window, so these are real tests, not descriptions
//! of intent.

use egui::{Event, Key, Modifiers, Pos2, RawInput, Rect};
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

/// A text-input event, as produced by typing.
fn type_text(s: &str) -> Vec<Event> {
    vec![Event::Text(s.to_owned())]
}

/// Drive one UI pass over `app` with `events`, with no window on screen.
fn pass(app: &mut OmaApp, events: Vec<Event>) {
    let ctx = egui::Context::default();
    let input = RawInput {
        screen_rect: Some(Rect::from_min_size(Pos2::ZERO, egui::vec2(1100.0, 700.0))),
        events,
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
fn cell_at_pos_maps_a_pointer_back_to_a_cell() {
    // A1's top-left in grid content coordinates is (HEADER_W, HEADER_H).
    let origin = Pos2::new(0.0, 0.0);
    assert_eq!(
        cell_at_pos(origin, Pos2::new(HEADER_W + 4.0, HEADER_H + 4.0)),
        Some(CellRef::new(0, 0, 0))
    );
    assert_eq!(
        cell_at_pos(
            origin,
            Pos2::new(HEADER_W + COL_W + 4.0, HEADER_H + ROW_H + 4.0)
        ),
        Some(CellRef::new(0, 1, 1))
    );
}

#[test]
fn cell_at_pos_returns_nothing_over_the_headers() {
    let origin = Pos2::new(0.0, 0.0);
    assert_eq!(cell_at_pos(origin, Pos2::new(5.0, 5.0)), None);
}

#[test]
fn cell_at_pos_returns_nothing_to_the_left_of_the_grid() {
    let origin = Pos2::new(0.0, 0.0);
    assert_eq!(cell_at_pos(origin, Pos2::new(-10.0, HEADER_H + 4.0)), None);
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
