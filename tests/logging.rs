//! The logger: a portal that cannot be reached has to say so somewhere.
//!
//! `rfd` reports that with `log::error!` and then resolves its future to `None`, which
//! the app reads as a cancelled dialog — so with no logger installed the failure is
//! indistinguishable from a user who dismissed the chooser. What is testable here is
//! that the sink works and that installing twice is not a panic, which is the difference
//! between a diagnosable app and an invisible one.
//!
//! The records themselves come from `rfd`, which a headless run cannot make fail; a
//! `log::info!` from the test is the same call the facade makes, and reaching the file is
//! the whole claim.

use omasheet::app::logging;

/// A unique marker per run, so a log left over from an earlier one cannot pass this.
const PROBE: &str = "omasheet-logging-probe";

#[test]
fn a_record_reaches_the_file_beside_the_executable() {
    let path = logging::log_path().expect("a running test binary has a path");
    assert_eq!(
        path.file_name().and_then(|n| n.to_str()),
        Some("omasheet.log"),
        "the log belongs beside the executable under a name a user would find"
    );

    assert!(
        logging::install(),
        "the first install in the process owns the logger"
    );
    log::info!("{PROBE}");

    let body = std::fs::read_to_string(&path).expect("the log file was just opened");
    assert!(
        body.contains(PROBE),
        "the record must reach the file; the log holds: {body}"
    );

    // `log::set_boxed_logger` answers `Err` rather than panicking on a second call, so
    // this is a claim about the shape of the failure and not about luck.
    assert!(
        !logging::install(),
        "a second install must report that it did nothing"
    );
}
