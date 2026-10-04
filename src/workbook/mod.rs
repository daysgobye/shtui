pub mod cell;
pub mod sheet;
// The design document fixes this file's name as `workbook/workbook.rs`, which clippy
// reads as a module named like its parent. The layout is deliberate; silence the lint
// rather than rename a file the spec specifies.
#[allow(clippy::module_inception)]
pub mod workbook;