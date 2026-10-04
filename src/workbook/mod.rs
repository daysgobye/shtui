pub mod cell;
pub mod sheet;
// Private now that callers have a path to the type: `omasheet::workbook::Workbook`.
// The file name stays as the design document specifies, so the lint stays silenced.
#[allow(clippy::module_inception)]
mod workbook;
pub use workbook::Workbook;
