use eframe::egui;

use crate::ui::grid::{self, GridState};
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
