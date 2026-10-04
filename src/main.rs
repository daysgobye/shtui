use eframe::egui;

fn main() -> eframe::Result {
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_inner_size([1100.0, 700.0])
            .with_title("Omasheet"),
        ..Default::default()
    };

    // `OmaApp` is still a unit struct, so clippy suggests the literal `OmaApp` instead of
    // `OmaApp::default()`. Later tasks give it fields; constructing through `Default` keeps
    // this call site correct across that change, so silence the lint here.
    #[allow(clippy::default_constructed_unit_structs)]
    let app = omasheet::app::app::OmaApp::default();

    eframe::run_native("omasheet", options, Box::new(|_cc| Ok(Box::new(app))))
}
