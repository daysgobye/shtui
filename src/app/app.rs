use eframe::egui;

use crate::workbook::Workbook;

#[derive(Default)]
pub struct OmaApp {
    pub wb: Workbook,
    pub error: Option<String>,
}

impl OmaApp {
    /// Replace the active sheet's contents with `sheet`, preserving its identity
    /// so that selection and scroll position stay meaningful.
    pub fn load_sheet(&mut self, sheet: crate::workbook::sheet::Sheet) {
        let id = self.wb.active_id();
        self.wb.replace_sheet(id, sheet);
    }

    /// Read a CSV into the active sheet. Parse failures surface in `error`
    /// rather than aborting, so a bad file never closes the window.
    pub fn open_csv(&mut self, path: &std::path::Path) {
        match crate::io::csv_read::read_csv_file(path) {
            Ok(sheet) => {
                self.load_sheet(sheet);
                self.error = None;
            }
            Err(e) => self.error = Some(format!("could not read {}: {e:?}", path.display())),
        }
    }
}

impl eframe::App for OmaApp {
    fn ui(&mut self, ui: &mut egui::Ui, _frame: &mut eframe::Frame) {
        ui.heading("Omasheet");
        ui.label(format!("{} sheets", self.wb.sheet_count()));
        if let Some(err) = &self.error {
            ui.colored_label(egui::Color32::from_rgb(220, 90, 90), err);
        }
    }
}
