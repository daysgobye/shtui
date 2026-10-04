use eframe::egui;

pub struct OmaApp;

impl Default for OmaApp {
    fn default() -> Self {
        Self
    }
}

impl eframe::App for OmaApp {
    fn ui(&mut self, ui: &mut egui::Ui, _frame: &mut eframe::Frame) {
        ui.centered_and_justified(|ui| {
            ui.heading("Omasheet");
            ui.label("skeleton");
        });
    }
}