//! The spreadsheet grid.
//!
//! Cells are painted manually through the `Painter` rather than composed from egui
//! widgets. A grid shows thousands of cells per frame, and allocating a widget per
//! cell is the difference between smooth scrolling and visible lag. Hit-testing is
//! likewise arithmetic against computed rectangles rather than per-cell interaction.
//!
//! The headers are frozen: their gutters are reserved *outside* the scrolling body,
//! which lives in a child `Ui` sized by `max_rect`. Painting them from the scrolled
//! content origin instead would scroll the column letters and row numbers off screen
//! along with the cells.

use eframe::egui::{
    Align2, Color32, FontId, Pos2, Rect, Sense, Stroke, StrokeKind, UiBuilder, Vec2, Visuals, pos2,
    vec2,
};

use crate::workbook::Workbook;
use crate::workbook::cell::{CellRef, SheetId, col_to_letters};

pub const ROW_H: f32 = 22.0;
pub const COL_W: f32 = 100.0;
pub const HEADER_W: f32 = 48.0;
pub const HEADER_H: f32 = 24.0;
/// Extra rows and columns painted beyond the viewport, so scrolling shows no gaps.
pub const OVERSCAN: i64 = 2;

/// Minimum scrollable extent, so an empty sheet still scrolls.
const MIN_ROWS: u32 = 200;
const MIN_COLS: u32 = 26;

/// The active cell plus the anchor it was selected from.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Selection {
    pub active: CellRef,
    pub anchor: CellRef,
}

impl Default for Selection {
    fn default() -> Self {
        let c = CellRef::new(0, 0, 0);
        Selection {
            active: c,
            anchor: c,
        }
    }
}

impl Selection {
    /// True when the selection is a single cell rather than a rectangle.
    pub fn is_single(&self) -> bool {
        self.active == self.anchor
    }

    /// The rectangle spanned by the anchor and the active cell.
    pub fn range(&self) -> crate::workbook::cell::CellRange {
        crate::workbook::cell::CellRange::from_corners(
            (self.anchor.row, self.anchor.col),
            (self.active.row, self.active.col),
        )
    }
}

/// State of an in-progress edit.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct EditState {
    pub active: CellRef,
    pub buffer: String,
}

#[derive(Clone, Debug, Default)]
pub struct GridState {
    pub selection: Selection,
    pub edit: Option<EditState>,
}

impl GridState {
    pub fn new() -> Self {
        Self::default()
    }
}

/// The top-left of cell `(row, col)` in grid content coordinates.
///
/// Content coordinates start at zero. The header gutters lie outside the scrolling
/// body, so they do not offset the cells, and `cell_min(0, 0)` is the body's own
/// top-left corner.
pub fn cell_min(row: u32, col: u32) -> Pos2 {
    Pos2::new(col as f32 * COL_W, row as f32 * ROW_H)
}

/// The rectangle of cell `(row, col)` in grid content coordinates.
pub fn cell_rect(row: u32, col: u32) -> Rect {
    Rect::from_min_size(cell_min(row, col), vec2(COL_W, ROW_H))
}

/// Map a pointer position to `(row, col)` in grid content coordinates.
///
/// `None` for anything outside the body, including positions above or left of the
/// content origin: those gutters belong to the headers, not to a cell. Returns bare
/// coordinates rather than a [`CellRef`] because this mapping knows nothing about
/// which sheet is active; [`click_to_selection`] applies that qualification.
pub fn cell_at_pos(origin: Pos2, pos: Pos2) -> Option<(u32, u32)> {
    let local = pos - origin;
    if local.x < 0.0 || local.y < 0.0 {
        return None;
    }
    Some((
        (local.y / ROW_H).floor() as u32,
        (local.x / COL_W).floor() as u32,
    ))
}

/// The selection a click at `pos` should produce, or `None` for a click outside the body.
///
/// Split out from [`show`] so the pointer-to-selection mapping is testable without a
/// window. egui's `Context::run_ui` does not simulate pointer input — verified: a
/// plain `ui.button` also fails to report `clicked()` under it — so `response.clicked()`
/// is the one link in this chain that no headless test can reach. Everything else,
/// including the sheet qualification, is covered here.
pub fn click_to_selection(origin: Pos2, pos: Pos2, sheet: SheetId) -> Option<CellRef> {
    cell_at_pos(origin, pos).map(|(row, col)| CellRef::new(sheet, row, col))
}

/// The half-open window of cells to paint, already clamped to the sheet extent.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct VisibleWindow {
    pub first_row: u32,
    pub last_row: u32,
    pub first_col: u32,
    pub last_col: u32,
}

impl VisibleWindow {
    pub fn cell_count(&self) -> u64 {
        let rows = self.last_row.saturating_sub(self.first_row) as u64;
        let cols = self.last_col.saturating_sub(self.first_col) as u64;
        rows * cols
    }
}

/// Which cells the viewport can see, plus `OVERSCAN` margin, clamped to the sheet.
///
/// `clip` is the body's screen rect, `origin` the top-left of the scrolled content
/// within it; the difference between them is the visible window in content
/// coordinates. Pure arithmetic, extracted so virtualisation can be tested without a
/// window — this is the arithmetic that keeps a 5,000-row sheet from costing 5,000
/// painted cells per frame.
pub fn visible_window(clip: Rect, origin: Pos2, rows: u32, cols: u32) -> VisibleWindow {
    let local_min = clip.min - origin;
    let local_max = clip.max - origin;
    let row_of = |y: f32| ((y / ROW_H).floor() as i64).max(0);
    let col_of = |x: f32| ((x / COL_W).floor() as i64).max(0);
    VisibleWindow {
        first_row: (row_of(local_min.y) - OVERSCAN).max(0) as u32,
        last_row: ((row_of(local_max.y) + OVERSCAN).max(0) as u32).min(rows),
        first_col: (col_of(local_min.x) - OVERSCAN).max(0) as u32,
        last_col: ((col_of(local_max.x) + OVERSCAN).max(0) as u32).min(cols),
    }
}

/// Row and column counts worth scrolling: at least a screenful, always covering
/// everything populated.
fn content_extent(wb: &Workbook) -> (u32, u32) {
    let sheet = wb.active();
    (
        sheet.row_count().max(MIN_ROWS),
        sheet.col_count().max(MIN_COLS),
    )
}

/// Apply a movement key to the selection. Returns true when the key was consumed.
///
/// The move is clamped to the scrollable extent: without the clamp the active cell can
/// leave the content rect, where its highlight is never painted and there is no way to
/// scroll to it. Following the selection with the view is a later milestone.
fn handle_key(
    state: &mut GridState,
    wb: &Workbook,
    rows: u32,
    cols: u32,
    k: egui::Key,
    shift: bool,
) -> bool {
    let sheet = wb.active_id();
    let (mut row, mut col) = (state.selection.active.row, state.selection.active.col);
    match k {
        egui::Key::ArrowUp => row = row.saturating_sub(1),
        egui::Key::ArrowDown => row += 1,
        egui::Key::ArrowLeft => col = col.saturating_sub(1),
        egui::Key::ArrowRight => col += 1,
        egui::Key::Home => col = 0,
        _ => return false,
    }
    let last_row = rows.saturating_sub(1);
    let last_col = cols.saturating_sub(1);
    state.selection.active = CellRef::new(sheet, row.min(last_row), col.min(last_col));
    if !shift {
        state.selection.anchor = state.selection.active;
    }
    true
}

/// Collect this frame's key-down events and whether shift is held.
///
/// Text input is deliberately not collected: `Ui::input` takes `FnOnce(&InputState)`, so
/// an owned text buffer would mean cloning every string, and nothing consumes one here
/// yet. The task that adds editing reads `Event::Text` in its own `ui.input` call.
fn read_input(ui: &egui::Ui) -> (Vec<egui::Key>, bool) {
    ui.input(|i| {
        let mut keys = Vec::new();
        for e in &i.events {
            // `repeat` stays filtered: key auto-repeat is a ledgered deferral, and
            // dropping that filter here would change shipped behaviour in this round.
            if let egui::Event::Key {
                key,
                pressed: true,
                repeat: false,
                ..
            } = e
            {
                keys.push(*key);
            }
        }
        (keys, i.modifiers.shift)
    })
}

/// Draw the grid and process navigation.
pub fn show(state: &mut GridState, wb: &mut Workbook, ui: &mut egui::Ui) {
    let (rows, cols) = content_extent(wb);
    let content_size = Vec2::new(cols as f32 * COL_W, rows as f32 * ROW_H);
    let outer = ui.available_rect_before_wrap();
    let visuals = ui.visuals().clone();

    // The scrolling body starts below and to the right of the headers, so the gutters
    // never move and the headers can be painted over them after the fact.
    let body = Rect::from_min_size(
        outer.min + vec2(HEADER_W, HEADER_H),
        vec2(
            (outer.width() - HEADER_W).max(1.0),
            (outer.height() - HEADER_H).max(1.0),
        ),
    );

    // An edit owns the caret; moving the selection underneath it would fight the edit.
    let mut handled = false;
    if state.edit.is_none() {
        let (keys, shift) = read_input(ui);
        for k in &keys {
            if handle_key(state, wb, rows, cols, *k, shift) {
                handled = true;
            }
        }
    }

    let mut body_ui = ui.new_child(UiBuilder::new().max_rect(body));
    let output = egui::ScrollArea::both()
        .max_height(body.height())
        .max_width(body.width())
        .auto_shrink([false, false])
        .show(&mut body_ui, |ui| {
            let (response, painter) = ui.allocate_painter(content_size, Sense::click());
            let origin = response.rect.min;

            // `if let` chains require edition 2024, which this crate uses.
            if let Some(pos) = response
                .interact_pointer_pos()
                .filter(|_| response.clicked())
                && let Some(cell) = click_to_selection(origin, pos, wb.active_id())
            {
                state.selection.active = cell;
                state.selection.anchor = cell;
                handled = true;
            }

            paint_body(state, wb, &painter, &visuals, origin, rows, cols);
        });

    paint_headers(
        ui.painter(),
        &visuals,
        outer,
        body,
        output.state.offset,
        rows,
        cols,
    );

    if handled {
        // Immediate mode repaints on demand; without this the highlight would stay
        // stale until the next unrelated repaint.
        ui.ctx().request_repaint();
    }
}

/// Paint the visible window of cells into the scrolling body.
fn paint_body(
    state: &GridState,
    wb: &Workbook,
    painter: &egui::Painter,
    visuals: &Visuals,
    origin: Pos2,
    rows: u32,
    cols: u32,
) {
    let grid_line = visuals.widgets.noninteractive.bg_stroke.color;
    // The sheet comes from the workbook, not from the selection: after a sheet switch
    // the selection can still name the old sheet for a frame.
    let sheet_id = wb.active_id();
    let sheet = wb.sheet(sheet_id);
    let sel = state.selection.range();

    // The clip rect is the ScrollArea's viewport, which is exactly the window we
    // must paint. Clipping means over-painting past the viewport is harmless.
    let win = visible_window(painter.clip_rect(), origin, rows, cols);

    for row in win.first_row..win.last_row {
        for col in win.first_col..win.last_col {
            let rect = cell_rect(row, col).translate(origin.to_vec2());
            let is_active = state.selection.active == CellRef::new(sheet_id, row, col);
            let in_selection = sel.contains(row, col);
            let cell = sheet.get(row, col);

            let bg = match cell.and_then(|c| c.format.bg_color) {
                Some(colour) => {
                    Color32::from_rgba_unmultiplied(colour.r, colour.g, colour.b, colour.a)
                }
                None if in_selection && !is_active => visuals.selection.bg_fill,
                None if row % 2 == 1 => visuals.widgets.inactive.weak_bg_fill,
                None => Color32::TRANSPARENT,
            };
            if bg != Color32::TRANSPARENT {
                painter.rect_filled(rect, 0.0, bg);
            }

            painter.rect_stroke(rect, 0.0, Stroke::new(1.0, grid_line), StrokeKind::Inside);

            if is_active {
                painter.rect_stroke(
                    rect,
                    0.0,
                    Stroke::new(2.0, visuals.selection.stroke.color),
                    StrokeKind::Middle,
                );
            }

            if let Some(cell) = cell {
                let colour = match cell.format.font_color {
                    Some(c) => Color32::from_rgba_unmultiplied(c.r, c.g, c.b, c.a),
                    None => visuals.text_color(),
                };
                paint_cell_text(painter, rect, cell.formatted(), colour);
            }
        }
    }

    if let Some(edit) = &state.edit {
        let rect = cell_rect(edit.active.row, edit.active.col).translate(origin.to_vec2());
        painter.rect_filled(rect, 0.0, visuals.extreme_bg_color);
        painter.rect_stroke(
            rect,
            0.0,
            Stroke::new(2.0, visuals.selection.stroke.color),
            StrokeKind::Middle,
        );
        paint_cell_text(painter, rect, edit.buffer.clone(), visuals.text_color());
    }
}

/// Paint the column and row headers, and the corner box, in screen space.
///
/// `offset` is the body's scroll offset, so a label's screen position is its content
/// position minus the offset. Labels outside their band are skipped rather than
/// clipped, so a 5,000-row sheet costs one pass over the visible band, not over every
/// row in it.
fn paint_headers(
    painter: &egui::Painter,
    visuals: &Visuals,
    outer: Rect,
    body: Rect,
    offset: Vec2,
    rows: u32,
    cols: u32,
) {
    let bg = visuals.widgets.inactive.bg_fill;
    let colour = visuals.weak_text_color();

    let col_band =
        Rect::from_min_size(pos2(body.left(), outer.top()), vec2(body.width(), HEADER_H));
    painter.rect_filled(col_band, 0.0, bg);
    for col in 0..cols {
        let r = Rect::from_min_size(
            pos2(
                col_band.left() + cell_min(0, col).x - offset.x,
                col_band.top(),
            ),
            vec2(COL_W, HEADER_H),
        );
        if r.right() < col_band.left() || r.left() > col_band.right() {
            continue;
        }
        // `FontId` is not `Copy`, so it is built per label rather than hoisted.
        painter.text(
            r.center(),
            Align2::CENTER_CENTER,
            col_to_letters(col),
            FontId::proportional(12.0),
            colour,
        );
    }

    let row_band = Rect::from_min_size(
        pos2(outer.left(), body.top()),
        vec2(HEADER_W, body.height()),
    );
    painter.rect_filled(row_band, 0.0, bg);
    for row in 0..rows {
        let r = Rect::from_min_size(
            pos2(
                row_band.left(),
                row_band.top() + cell_min(row, 0).y - offset.y,
            ),
            vec2(HEADER_W, ROW_H),
        );
        if r.bottom() < row_band.top() || r.top() > row_band.bottom() {
            continue;
        }
        painter.text(
            r.center(),
            Align2::CENTER_CENTER,
            (row + 1).to_string(),
            FontId::proportional(12.0),
            colour,
        );
    }

    // The corner box where the two headers meet.
    painter.rect_filled(
        Rect::from_min_size(outer.min, vec2(HEADER_W, HEADER_H)),
        0.0,
        bg,
    );
}

/// Paint `text` inside `rect`: vertically centred on the laid-out text, and clipped to
/// the cell so a long value cannot bleed into its neighbours.
fn paint_cell_text(painter: &egui::Painter, rect: Rect, text: String, colour: Color32) {
    if text.is_empty() {
        return;
    }
    let galley = painter.layout_no_wrap(text, FontId::proportional(14.0), colour);
    let pos = Pos2::new(rect.left() + 4.0, rect.center().y - galley.size().y / 2.0);
    painter.with_clip_rect(rect).galley(pos, galley, colour);
}
