use eframe::egui;

fn main() -> eframe::Result {
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_inner_size([1100.0, 700.0])
            .with_title("Omasheet"),
        ..Default::default()
    };

    let app = omasheet::app::app::OmaApp::default();

    eframe::run_native("omasheet", options, Box::new(|_cc| Ok(Box::new(app))))
}
