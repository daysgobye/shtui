//! The spreadsheet grid.
//!
//! Cells are painted manually through the `Painter` rather than composed from egui
//! widgets. A grid shows thousands of cells per frame, and allocating a widget per
//! cell is the difference between smooth scrolling and visible lag. Hit-testing is
//! likewise arithmetic against computed rectangles rather than per-cell interaction.

use eframe::egui::{
    self, Align2, Color32, FontId, Pos2, Rect, Sense, Stroke, StrokeKind, Vec2, Visuals,
};

use crate::workbook::Workbook;
use crate::workbook::cell::{CellRef, col_to_letters};

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
pub fn cell_min(row: u32, col: u32) -> Pos2 {
    Pos2::new(HEADER_W + col as f32 * COL_W, HEADER_H + row as f32 * ROW_H)
}

/// The rectangle of cell `(row, col)` in grid content coordinates.
pub fn cell_rect(row: u32, col: u32) -> Rect {
    Rect::from_min_size(cell_min(row, col), Vec2::new(COL_W, ROW_H))
}

/// Map a pointer position to a cell, or `None` when it is over the headers.
pub fn cell_at_pos(origin: Pos2, pos: Pos2) -> Option<CellRef> {
    let local = pos - origin;
    if local.x < HEADER_W || local.y < HEADER_H {
        return None;
    }
    let col = ((local.x - HEADER_W) / COL_W).floor();
    let row = ((local.y - HEADER_H) / ROW_H).floor();
    if row < 0.0 || col < 0.0 {
        return None;
    }
    Some(CellRef::new(0, row as u32, col as u32))
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
fn handle_key(state: &mut GridState, wb: &Workbook, k: egui::Key, shift: bool) -> bool {
    let sheet = wb.active_id();
    let (mut row, mut col) = (state.selection.active.row, state.selection.active.col);
    match k {
        egui::Key::ArrowUp if row > 0 => row -= 1,
        egui::Key::ArrowDown => row += 1,
        egui::Key::ArrowLeft if col > 0 => col -= 1,
        egui::Key::ArrowRight => col += 1,
        egui::Key::Home => col = 0,
        _ => return false,
    }
    state.selection.active = CellRef::new(sheet, row, col);
    if !shift {
        state.selection.anchor = state.selection.active;
    }
    true
}

/// Collect this frame's key-down events and text-input events.
fn read_input(ui: &egui::Ui) -> (Vec<egui::Key>, Vec<String>, bool) {
    ui.input(|i| {
        let mut keys = Vec::new();
        let mut text = Vec::new();
        for e in &i.events {
            match e {
                egui::Event::Key {
                    key,
                    pressed: true,
                    repeat: false,
                    ..
                } => keys.push(*key),
                egui::Event::Text(t) => text.push(t.clone()),
                _ => {}
            }
        }
        (keys, text, i.modifiers.shift)
    })
}

/// Draw the grid and process navigation.
pub fn show(state: &mut GridState, wb: &mut Workbook, ui: &mut egui::Ui) {
    let (rows, cols) = content_extent(wb);
    let content_size = Vec2::new(
        HEADER_W + cols as f32 * COL_W,
        HEADER_H + rows as f32 * ROW_H,
    );
    let viewport = ui.available_size();
    let visuals = ui.visuals().clone();

    let (keys, _text_events, shift) = read_input(ui);
    let mut handled = false;
    for k in &keys {
        if handle_key(state, wb, *k, shift) {
            handled = true;
        }
    }

    egui::ScrollArea::both()
        .max_height(viewport.y)
        .max_width(viewport.x)
        .auto_shrink([false, false])
        .show(ui, |ui| {
            let (response, painter) = ui.allocate_painter(content_size, Sense::click());
            let origin = response.rect.min;

            // `if let` chains require edition 2024, which this crate uses.
            if let Some(pos) = response
                .interact_pointer_pos()
                .filter(|_| response.clicked())
                && let Some(hit) = cell_at_pos(origin, pos)
            {
                let cell = CellRef::new(wb.active_id(), hit.row, hit.col);
                state.selection.active = cell;
                state.selection.anchor = cell;
                handled = true;
            }

            paint(state, wb, &painter, &visuals, origin, rows, cols);
        });

    if handled {
        // Immediate mode repaints on demand; without this the highlight and the
        // edited contents would stay stale until the next unrelated repaint.
        ui.ctx().request_repaint();
    }
}

/// Paint the headers and the visible window of cells.
fn paint(
    state: &GridState,
    wb: &Workbook,
    painter: &egui::Painter,
    visuals: &Visuals,
    origin: Pos2,
    rows: u32,
    cols: u32,
) {
    let header_bg = visuals.widgets.inactive.bg_fill;
    let grid_line = visuals.widgets.noninteractive.bg_stroke.color;
    let sheet_id = state.selection.active.sheet;
    let sheet = wb.sheet(sheet_id);
    let sel = state.selection.range();

    // The clip rect is the ScrollArea's viewport, which is exactly the window we
    // must paint. Clipping means over-painting the headers is harmless.
    let clip = painter.clip_rect();
    let local_min = clip.min - origin;
    let local_max = clip.max - origin;

    let first_row = ((((local_min.y - HEADER_H) / ROW_H).floor() as i64) - OVERSCAN).max(0) as u32;
    let last_row = ((((local_max.y - HEADER_H) / ROW_H).ceil() as i64) + OVERSCAN).max(0) as u32;
    let first_col = ((((local_min.x - HEADER_W) / COL_W).floor() as i64) - OVERSCAN).max(0) as u32;
    let last_col = ((((local_max.x - HEADER_W) / COL_W).ceil() as i64) + OVERSCAN).max(0) as u32;
    let last_row = last_row.min(rows);
    let last_col = last_col.min(cols);

    for row in first_row..last_row {
        for col in first_col..last_col {
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
                let text = cell.formatted();
                if !text.is_empty() {
                    let colour = match cell.format.font_color {
                        Some(c) => Color32::from_rgba_unmultiplied(c.r, c.g, c.b, c.a),
                        None => visuals.text_color(),
                    };
                    let galley = painter.layout_no_wrap(text, FontId::proportional(14.0), colour);
                    painter.galley(
                        Pos2::new(rect.left() + 4.0, rect.center().y - 7.0),
                        galley,
                        colour,
                    );
                }
            }
        }
    }

    // Column headers, left to right.
    let col_header =
        Rect::from_min_size(origin, Vec2::new(HEADER_W + cols as f32 * COL_W, HEADER_H));
    painter.rect_filled(col_header, 0.0, header_bg);
    for col in 0..last_col {
        let r = Rect::from_min_size(
            Pos2::new(origin.x + cell_min(0, col).x, origin.y),
            Vec2::new(COL_W, HEADER_H),
        );
        painter.text(
            r.center(),
            Align2::CENTER_CENTER,
            col_to_letters(col),
            FontId::proportional(12.0),
            visuals.weak_text_color(),
        );
    }

    // Row headers, top to bottom.
    for row in first_row..last_row {
        let r = Rect::from_min_size(
            Pos2::new(origin.x, origin.y + cell_min(row, 0).y),
            Vec2::new(HEADER_W, ROW_H),
        );
        painter.rect_filled(r, 0.0, header_bg);
        painter.text(
            r.center(),
            Align2::CENTER_CENTER,
            (row + 1).to_string(),
            FontId::proportional(12.0),
            visuals.weak_text_color(),
        );
    }

    // The corner box where the two headers meet.
    painter.rect_filled(
        Rect::from_min_size(origin, Vec2::new(HEADER_W, HEADER_H)),
        0.0,
        header_bg,
    );

    if let Some(edit) = &state.edit {
        let rect = cell_rect(edit.active.row, edit.active.col).translate(origin.to_vec2());
        painter.rect_filled(rect, 0.0, visuals.extreme_bg_color);
        painter.rect_stroke(
            rect,
            0.0,
            Stroke::new(2.0, visuals.selection.stroke.color),
            StrokeKind::Middle,
        );
        let colour = visuals.text_color();
        let galley =
            painter.layout_no_wrap(edit.buffer.clone(), FontId::proportional(14.0), colour);
        painter.galley(
            Pos2::new(rect.left() + 4.0, rect.center().y - 7.0),
            galley,
            colour,
        );
    }
}
