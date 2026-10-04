use eframe::egui;

use crate::ui::grid::{self, GridState, Selection};
use crate::workbook::Workbook;
use crate::workbook::sheet::Sheet;

pub struct OmaApp {
    pub wb: Workbook,
    pub grid: GridState,
    pub error: Option<String>,
}

impl Default for OmaApp {
    fn default() -> Self {
        Self {
            wb: Workbook::new(),
            grid: GridState::new(),
            error: None,
        }
    }
}

impl OmaApp {
    /// Point every sheet-qualified reference the UI holds at the active sheet.
    ///
    /// The grid's selection and any open editor carry `CellRef`s, which are sheet-qualified
    /// by design so cross-sheet references need no special case. That makes them go stale
    /// the moment the active sheet changes: `selection_cells` would clear cells on the
    /// sheet the selection *was* on, a highlight would stop painting because it is
    /// compared against a freshly qualified `CellRef`, and `remove_sheet` re-stamps every
    /// surviving sheet id, so a stale reference can silently come to mean a different
    /// sheet. One call here retargets the active cell, the anchor and the editor together,
    /// so they cannot disagree with the model or with each other.
    ///
    /// This is called once per frame from [`OmaApp::body`] rather than from each of
    /// `Workbook`'s three mutators, so no route to a changed active sheet can be missed.
    /// The cost is three integer writes. The trade-off it makes: selection position is not
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
        if let Some(edit) = self.grid.edit.as_mut() {
            edit.active.sheet = sheet;
        }
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
        ui.horizontal(|ui| {
            ui.heading("Omasheet");
            if self.wb.dirty() {
                ui.label("modified");
            }
        });
        if let Some(err) = &self.error {
            ui.colored_label(egui::Color32::from_rgb(220, 90, 90), err);
        }
        grid::show(&mut self.grid, &mut self.wb, ui);
    }
}

impl eframe::App for OmaApp {
    fn ui(&mut self, ui: &mut egui::Ui, _frame: &mut eframe::Frame) {
        self.body(ui);
    }
}
