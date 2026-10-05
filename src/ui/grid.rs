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
//!
//! The grid holds no editor. Text is typed in the formula bar, so this module's whole
//! involvement in editing is [`GridState::focus_requested`]: a gesture that means "the
//! user wants to type" asks for the keyboard, and the app decides what that means. A
//! second editor here could disagree with the first one about what a cell contains, which
//! is the defect this design removes.

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

/// How far the zebra stripe moves from the background towards the text colour.
///
/// Enough to carry the eye across a row without the stripe becoming a band of its own.
const STRIPE_STRENGTH: f32 = 0.055;

/// How far a grid line moves the same way. Kept clearly above the stripe so a striped
/// row never reads as one thick edge.
const GRID_STRENGTH: f32 = 0.16;

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

/// The grid's state.
#[derive(Clone, Debug, Default)]
pub struct GridState {
    pub selection: Selection,
    /// Set when a gesture means "the user wants to type here": a printable character,
    /// `F2`, or a double click. The grid consumes the gesture and asks for the keyboard;
    /// it does not know what will receive it. [`crate::app::app::OmaApp::body`] clears the
    /// flag by handing it to the formula bar.
    pub focus_requested: bool,
}

impl GridState {
    pub fn new() -> Self {
        Self::default()
    }
}

/// One step of cell movement.
///
/// Shared by the grid's arrow keys and the formula bar's `Enter` and `Tab`, so both clamp
/// identically: a commit-move that could walk off the content rect would strand the
/// highlight where it is never painted and cannot be scrolled back to.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Step {
    Up,
    Down,
    Left,
    Right,
}

/// The cell one step from `from`, clamped to the scrollable extent.
fn step_from(from: CellRef, rows: u32, cols: u32, step: Step) -> CellRef {
    let (row, col) = match step {
        Step::Up => (from.row.saturating_sub(1), from.col),
        Step::Down => (from.row + 1, from.col),
        Step::Left => (from.row, from.col.saturating_sub(1)),
        Step::Right => (from.row, from.col + 1),
    };
    CellRef::new(
        from.sheet,
        row.min(rows.saturating_sub(1)),
        col.min(cols.saturating_sub(1)),
    )
}

/// Move the active cell one `step`, taking the anchor with it.
///
/// The bar's `Enter` and `Tab` arrive as this rather than as a key, because the bar does
/// not know how a move is clamped — only the grid does.
pub fn step_active(state: &mut GridState, wb: &Workbook, step: Step) {
    let (rows, cols) = content_extent(wb);
    let next = step_from(state.selection.active, rows, cols, step);
    state.selection.active = next;
    state.selection.anchor = next;
}

/// Linear blend of two opaque colours. `t` of 0 returns `a`, 1 returns `b`.
fn mix(a: Color32, b: Color32, t: f32) -> Color32 {
    let channel = |x: u8, y: u8| (x as f32 + (y as f32 - x as f32) * t).round() as u8;
    Color32::from_rgb(
        channel(a.r(), b.r()),
        channel(a.g(), b.g()),
        channel(a.b(), b.b()),
    )
}

/// The zebra stripe and grid line colours for a theme.
///
/// Both are the background moved a *fixed fraction towards the text colour*, so the
/// stripe darkens a light theme and lightens a dark one — always away from the
/// background, never towards black. Deriving them from the text colour alone, which is
/// what this did first, produces a near-black stripe on a white background.
///
/// Exposed so the relationship can be asserted without a window: separability in *any*
/// theme is the claim, and it is worth testing rather than eyeballing on one machine.
pub fn zebra_and_grid_colours(visuals: &Visuals) -> (Color32, Color32) {
    let base = visuals.panel_fill;
    let text = visuals.text_color();
    (
        mix(base, text, STRIPE_STRENGTH),
        mix(base, text, GRID_STRENGTH),
    )
}
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
/// The move is clamped to the scrollable extent, and `Shift` extends the selection instead
/// of moving the anchor. Following the selection with the view is a later milestone.
fn handle_key(state: &mut GridState, wb: &Workbook, k: egui::Key, shift: bool) -> bool {
    let (rows, cols) = content_extent(wb);
    let active = state.selection.active;
    let next = match k {
        egui::Key::ArrowUp => step_from(active, rows, cols, Step::Up),
        egui::Key::ArrowDown => step_from(active, rows, cols, Step::Down),
        egui::Key::ArrowLeft => step_from(active, rows, cols, Step::Left),
        egui::Key::ArrowRight => step_from(active, rows, cols, Step::Right),
        egui::Key::Home => CellRef::new(active.sheet, active.row, 0),
        _ => return false,
    };
    state.selection.active = next;
    if !shift {
        state.selection.anchor = next;
    }
    true
}

/// Collect this frame's key-down events and whether shift is held.
///
/// Text input is collected separately by [`read_text`]: `Ui::input` takes
/// `FnOnce(&InputState)`, so one closure can build one owned collection, and folding the
/// second into this tuple would couple every caller to the question of what starts an
/// edit.
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

/// Whether this frame carries text that means "start typing".
///
/// Control-only text does not count: egui-winit already filters it at the source, but
/// `Event::Text`'s own contract only asks an integration to withhold text for Enter — a
/// backend that leaks it must not hand the keyboard to the formula bar.
///
/// Shared with the bar, which needs the same answer for the same frame to tell a
/// keystroke that starts a fresh value from one that edits the one already there. Two
/// copies of that rule could disagree about which of two frames is which.
pub fn typed_printable(text: &[String]) -> bool {
    text.iter().any(|t| !t.chars().all(char::is_control))
}

/// Collect this frame's text-input events.
pub fn read_text(ui: &egui::Ui) -> Vec<String> {
    ui.input(|i| {
        i.events
            .iter()
            .filter_map(|e| match e {
                egui::Event::Text(t) => Some(t.clone()),
                _ => None,
            })
            .collect()
    })
}

/// What a pointer event in the grid body means.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum CellAction {
    /// Nothing: no button was pressed, or the press landed outside the body.
    None,
    /// A click: select `cell`.
    Select(CellRef),
}

/// Decide what a pointer event at `pos` in the grid body means.
///
/// Split out of the `ScrollArea` closure so the decision is testable without a window.
/// `Context::run_ui` does not simulate pointer input at all, so `Response::clicked` and
/// `Response::double_clicked` are the link in this chain that no headless test can reach
/// — they arrive here as plain booleans instead of being read off a `Response`.
/// [`click_to_selection`] covers the remaining, pointer-mapping half.
///
/// A double click selects like any other click. It used to mean "open an editor here", and
/// now means "and hand the keyboard over" — a request the grid raises through
/// [`GridState::focus_requested`] rather than an action it can take itself, because there
/// is no editor left here to open.
pub fn click_action(
    origin: Pos2,
    pos: Pos2,
    sheet: SheetId,
    clicked: bool,
    double_clicked: bool,
) -> CellAction {
    if !(clicked || double_clicked) {
        return CellAction::None;
    }
    // A double click also reports `clicked()`, so either answer selects the same cell.
    click_to_selection(origin, pos, sheet).map_or(CellAction::None, CellAction::Select)
}

/// Every cell in the selection rectangle, in row-major order.
fn selection_cells(state: &GridState) -> Vec<CellRef> {
    let sheet = state.selection.active.sheet;
    let r = state.selection.range();
    let mut out = Vec::new();
    for row in r.min_row..=r.max_row {
        for col in r.min_col..=r.max_col {
            out.push(CellRef::new(sheet, row, col));
        }
    }
    out
}

/// Draw the grid and process navigation.
///
/// `input_locked` says the keyboard belongs to the formula bar. The grid then reads
/// nothing: one `Backspace` must not both delete a character from the field and clear the
/// cell underneath it, and only the widget holding focus can be trusted to consume a key
/// exactly once.
pub fn show(state: &mut GridState, wb: &mut Workbook, input_locked: bool, ui: &mut egui::Ui) {
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

    let mut handled = false;
    if !input_locked {
        let (keys, shift) = read_input(ui);
        let text = read_text(ui);
        if keys.contains(&egui::Key::F2) {
            // Two ways in, and both mean the same thing to the grid: `F2` to edit what is
            // already there, a printable character to start from nothing. Either way the
            // grid's own job is over, so navigation does not also run — moving the
            // selection out from under a caret that is on its way would leave the field
            // bound to a cell that is no longer the selected one.
            state.focus_requested = true;
            handled = true;
        } else if typed_printable(&text) {
            state.focus_requested = true;
            handled = true;
        } else {
            for k in &keys {
                match k {
                    egui::Key::Delete | egui::Key::Backspace => {
                        for cell in selection_cells(state) {
                            wb.clear_cell(cell);
                        }
                        handled = true;
                    }
                    _ => {
                        if handle_key(state, wb, *k, shift) {
                            handled = true;
                        }
                    }
                }
            }
        }
    }

    let mut body_ui = ui.new_child(UiBuilder::new().id_salt("grid-body").max_rect(body));
    let output = egui::ScrollArea::both()
        .max_height(body.height())
        .max_width(body.width())
        .auto_shrink([false, false])
        .show(&mut body_ui, |ui| {
            let (response, painter) = ui.allocate_painter(content_size, Sense::click());
            let origin = response.rect.min;

            let pos = response.interact_pointer_pos();
            let action = pos.map_or(CellAction::None, |pos| {
                click_action(
                    origin,
                    pos,
                    wb.active_id(),
                    response.clicked(),
                    response.double_clicked(),
                )
            });
            match action {
                CellAction::None => {}
                CellAction::Select(cell) => {
                    state.selection.active = cell;
                    state.selection.anchor = cell;
                    handled = true;
                }
            }
            if response.double_clicked() {
                // A double click asks for the keyboard. There is nothing to open here —
                // the field is somewhere else entirely — so the request is all it can
                // mean, and the app decides where it goes.
                state.focus_requested = true;
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
    // The zebra stripe and the grid lines were each borrowed from a different theme slot,
    // and in egui's own palette those two greys sit close enough together that the stripe
    // and the lines read as one blur. Deriving both from the text colour at a fixed
    // fraction makes the relationship explicit — the line is three times the stripe's
    // strength — and holds in light and dark alike, rather than only in this theme.
    let (stripe_colour, grid_line) = zebra_and_grid_colours(visuals);
    let base_colour = visuals.panel_fill;
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
                None if row % 2 == 1 => stripe_colour,
                // Every row is painted, not just the striped ones. The stripe is computed
                // relative to the background, so leaving the unstriped rows transparent
                // would mix the stripe against whatever happened to be behind the window.
                None => base_colour,
            };
            painter.rect_filled(rect, 0.0, bg);

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
}

/// Paint the column and row headers, and the corner box, in screen space.
///
/// `offset` is the body's scroll offset, so a label's screen position is its content
/// position minus the offset. The loops run over every row and every column, so
/// *iteration* is not bounded by the viewport — what is bounded is painting: a label
/// whose band falls outside the gutter is skipped rather than drawn or clipped. A
/// 5,000-row sheet therefore costs 5,000 cheap bounds tests per frame to paint about 30
/// labels. Making the iteration bounded too would mean binary-searching the first and
/// last visible index, which is left alone until the loop cost is actually measured.
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
