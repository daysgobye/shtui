use eframe::egui::{self, Rect, UiBuilder, pos2};

use crate::ui::formula_bar::{self, FormulaBar, Navigate};
use crate::ui::grid::{self, GridState, Selection, Step};
use crate::workbook::Workbook;
use crate::workbook::sheet::Sheet;

pub struct OmaApp {
    pub wb: Workbook,
    pub grid: GridState,
    pub bar: FormulaBar,
    pub error: Option<String>,
}

impl Default for OmaApp {
    fn default() -> Self {
        Self {
            wb: Workbook::new(),
            grid: GridState::new(),
            bar: FormulaBar::new(),
            error: None,
        }
    }
}

impl OmaApp {
    /// Point every sheet-qualified reference the UI holds at the active sheet.
    ///
    /// The grid's selection carries a `CellRef`, which is sheet-qualified by design so
    /// cross-sheet references need no special case. That makes it go stale the moment the
    /// active sheet changes: a highlight would stop painting because it is compared
    /// against a freshly qualified `CellRef`, and `remove_sheet` re-stamps every surviving
    /// sheet id, so a stale reference can silently come to mean a different sheet. One
    /// call here requalifies it, and the formula bar follows by comparison: it reloads
    /// from whatever cell it is handed, so the two cannot disagree.
    ///
    /// This is called once per frame from [`OmaApp::body`] rather than from each of
    /// `Workbook`'s three mutators, so no route to a changed active sheet can be missed.
    /// The cost is two integer writes. The trade-off it makes: selection position is not
    /// remembered per sheet. When sheet tabs land and per-sheet selection is wanted, this
    /// is the one line to replace — save the old corners keyed by sheet id before calling.
    pub fn retarget_selection_to_active_sheet(&mut self) {
        let sheet = self.wb.active_id();
        let Selection {
            mut active,
            mut anchor,
        } = self.grid.selection;
        active.sheet = sheet;
        anchor.sheet = sheet;
        self.grid.selection = Selection { active, anchor };
    }

    /// Replace the active sheet's contents, preserving its identity so that
    /// selection and scroll position stay meaningful.
    pub fn load_sheet(&mut self, sheet: Sheet) {
        let id = self.wb.active_id();
        self.wb.replace_sheet(id, sheet);
    }

    /// Read a CSV into the active sheet. A parse failure surfaces in `error`
    /// rather than aborting, so a bad file never closes the window.
    pub fn open_csv(&mut self, path: &std::path::Path) {
        match crate::io::csv_read::read_csv_file(path) {
            Ok(sheet) => {
                self.load_sheet(sheet);
                self.error = None;
            }
            Err(e) => self.error = Some(format!("could not read {}: {e}", path.display())),
        }
    }

    /// The UI body. Split from `eframe::App::ui` so headless tests can drive it.
    pub fn body(&mut self, ui: &mut egui::Ui) {
        // Before anything reads the selection this frame, including the grid's own
        // navigation and painting. See `retarget_selection_to_active_sheet`.
        self.retarget_selection_to_active_sheet();
        // No title strip. "Omasheet" said nothing the window manager does not already say,
        // and a heading costs a row of vertical space the grid could use. The modified
        // marker lives in the formula bar's name box instead, which is where a spreadsheet
        // keeps it and where it sits next to the address it refers to.
        if let Some(err) = &self.error {
            ui.colored_label(egui::Color32::from_rgb(220, 90, 90), err);
        }

        // Whether the bar holds the keyboard has to be settled before the grid reads any
        // key: a `Backspace` must be either a deletion in the field or a deletion of the
        // cell, never both.
        let bar_owns_input = self.bar.focused(ui);

        // The two share the window rather than being stacked by whatever order they were
        // added in, because the bar's height is not known until the text in it is: the
        // grid is given the remainder, so a long value shrinks the grid instead of
        // pushing the bar off the bottom of the screen.
        let full = ui.available_rect_before_wrap();
        let bar_top = (full.bottom() - self.bar.min_height()).max(full.top());
        let mut grid_ui = ui.new_child(
            UiBuilder::new()
                .id_salt("grid")
                .max_rect(Rect::from_min_max(full.min, pos2(full.right(), bar_top))),
        );
        grid::show(&mut self.grid, &mut self.wb, bar_owns_input, &mut grid_ui);

        // The grid's gesture is complete; where the focus goes is this layer's business.
        if std::mem::take(&mut self.grid.focus_requested) {
            self.bar.request_focus();
        }
        let bar_rect = Rect::from_min_max(pos2(full.left(), bar_top), full.max);
        let mut bar_ui = ui.new_child(
            UiBuilder::new()
                .id_salt("formula-bar")
                .max_rect(bar_rect)
                .layout(egui::Layout::top_down(egui::Align::LEFT)),
        );
        // Read before the borrow, not inline in the call: `show` takes `&mut wb` and
        // `dirty` reads it.
        let dirty = self.wb.dirty();
        formula_bar::show(
            &mut self.bar,
            &mut self.wb,
            self.grid.selection.active,
            dirty,
            &mut bar_ui,
        );

        // Applied after the bar has been drawn, so a move made this frame is loaded into
        // the field on the next one rather than overwritten by this frame's text.
        if let Some(navigate) = self.bar.take_navigate() {
            let step = match navigate {
                Navigate::Down => Step::Down,
                Navigate::Right => Step::Right,
            };
            grid::step_active(&mut self.grid, &self.wb, step);
            ui.ctx().request_repaint();
        }
    }
}

impl eframe::App for OmaApp {
    fn ui(&mut self, ui: &mut egui::Ui, _frame: &mut eframe::Frame) {
        self.body(ui);
    }
}
