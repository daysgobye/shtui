use eframe::egui;

fn main() -> eframe::Result {
    // Before anything can log. rfd reports a portal it cannot reach through `log` and
    // then answers `None`, which reads as a cancelled dialog — so without this, a
    // `Ctrl+O` on a machine with no portal leaves nothing behind to look at.
    if !omasheet::app::logging::install() {
        // Said rather than dropped: `false` means something else is already the logger,
        // so this app's records are going wherever that is, and if it is nothing the
        // silent gap is the whole problem the logger was added to fix.
        eprintln!(
            "omasheet: logging is not installed by this process; log records may go elsewhere"
        );
    }

    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_inner_size([1100.0, 700.0])
            .with_title("Omasheet"),
        ..Default::default()
    };

    let app = omasheet::app::app::OmaApp::default();

    eframe::run_native("omasheet", options, Box::new(|_cc| Ok(Box::new(app))))
}
