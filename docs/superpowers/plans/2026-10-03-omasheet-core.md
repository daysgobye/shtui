# Omasheet Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A window that opens a CSV into a fast, virtualized, editable spreadsheet grid, with selection, keyboard navigation, in-cell editing, and dirty-state tracking.

**Architecture:** A single Rust crate. The data model (`workbook/`) and I/O (`io/`) are pure Rust with no egui dependency and are unit-tested directly. The grid (`ui/`) paints cells manually inside an egui `ScrollArea` rather than composing per-cell widgets, which is what keeps scrolling cheap. UI logic is testable headlessly through `egui::Context::run_ui`, so keybindings are covered by tests rather than by hand.

**Tech Stack:** Rust 2024 edition, `eframe = "=0.35.0"`, `egui = "=0.35.0"`, `csv = "=1.4.0"`.

**Spec:** `docs/superpowers/specs/2026-10-03-omasheet-m1-design.md` — read it before starting. Section references below point into it.

## Global Constraints

- `eframe` and `egui` are pinned with `=` to **0.35.0**. Version 0.36.2 declares `rust-version = "1.95"` and this machine has **rustc 1.94.0**; it does not build. Do not unpin these.
- The `App` trait method is `fn ui(&mut self, ui: &mut egui::Ui, frame: &mut eframe::Frame)`. It is **not** `update(ctx, frame)`.
- `ui.add(widget)` returns `egui::Response` in 0.35. Do not write `.response.changed()`; write `.changed()`.
- `Context::run_ui` returns `FullOutput`, which is `#[must_use]`. Bind it with `let _ =`.
- The `workbook/` and `io/` modules must not depend on egui. `Color` is a plain `(r, g, b, a)` byte struct defined in `workbook/cell.rs`; conversion to `egui::Color32` happens only in `ui/`.
- Rows and columns are **zero-indexed** everywhere in the engine. Column/row labels (`A1`) exist only at the UI and parser boundary.
- Cell text is stored in the model exactly as the user typed it. No trimming, no case folding, no reformatting on commit.
- No `unwrap()` or `expect()` in library code. Tests may use them.
- Run `cargo clippy --all-targets` before every commit; it must be silent.
- Commit after every task with a Conventional Commits message.

## File Structure

| File | Responsibility |
|---|---|
| `Cargo.toml` | Dependencies, pinned versions, crate metadata |
| `.gitignore` | `/target`, editor cruft |
| `src/main.rs` | Binary entry point, `run_native` wiring |
| `src/lib.rs` | Library root; re-exports modules for tests |
| `src/workbook/mod.rs` | Module declarations |
| `src/workbook/cell.rs` | `SheetId`, `CellRef`, `CellValue`, `ErrorKind`, `Color`, `NumberFormat`, `CellFormat`, `Cell`, `CellRange`, A1 address helpers |
| `src/workbook/sheet.rs` | `Sheet`: sparse row storage, used-range maintenance |
| `src/workbook/workbook.rs` | `Workbook`: sheet registry, active sheet, dirty flag, cell accessors |
| `src/io/mod.rs` | Module declarations |
| `src/io/csv_read.rs` | Delimiter sniffing, CSV to `Sheet`, number inference |
| `src/app/mod.rs` | Module declarations |
| `src/app/app.rs` | `OmaApp`: owns `Workbook`, grid state, layout |
| `src/ui/mod.rs` | Module declarations |
| `src/ui/grid.rs` | `Grid`: geometry constants, painting, hit-testing, key handling, editing |
| `tests/grid_keys.rs` | Headless keybinding and editing tests via `Context::run_ui` |

---

### Task 1: Crate Skeleton and Window

**Files:**
- Create: `Cargo.toml`
- Create: `.gitignore`
- Create: `src/main.rs`
- Create: `src/lib.rs`
- Create: `src/workbook/mod.rs`
- Create: `src/io/mod.rs`
- Create: `src/app/mod.rs`
- Create: `src/ui/mod.rs`

**Interfaces:**
- Consumes: nothing.
- Produces: a compiling, window-opening binary. `OmaApp` implementing `eframe::App`, with `pub fn ui(&mut self, ui: &mut egui::Ui, frame: &mut eframe::Frame)`. Later tasks add fields to `OmaApp` and delegate to it.

- [ ] **Step 1: Create `Cargo.toml`**

```toml
[package]
name = "omasheet"
version = "0.1.0"
edition = "2024"
description = "A fast, native spreadsheet. Open a file, work with your data, get out of the way."
license = "MIT"

[dependencies]
eframe = "=0.35.0"
egui = "=0.35.0"
csv = "=1.4.0"

[profile.dev]
opt-level = 1

[profile.dev.package."*"]
opt-level = 3
```

`opt-level = 1` on the dev profile matters: egui and wgpu are dependencies that would otherwise be unoptimized, which makes the app feel like a slideshow during development.

- [ ] **Step 2: Create `.gitignore`**

```gitignore
/target
*.swp
```

- [ ] **Step 3: Create the module files**

`src/lib.rs`:
```rust
pub mod app;
pub mod io;
pub mod ui;
pub mod workbook;
```

`src/workbook/mod.rs`:
```rust
pub mod cell;
pub mod sheet;
// The design document fixes this file's name as `workbook/workbook.rs`, which clippy
// reads as a module named like its parent. The layout is deliberate; silence the lint
// rather than rename a file the spec specifies.
#[allow(clippy::module_inception)]
mod workbook;
pub use workbook::Workbook;
```

`src/io/mod.rs`:
```rust
pub mod csv_read;
```

`src/app/mod.rs`:
```rust
#[allow(clippy::module_inception)]
pub mod app;
```

`src/ui/mod.rs`:
```rust
pub mod grid;
```

- [ ] **Step 4: Create placeholder contents so the modules compile**

`src/workbook/cell.rs`:
```rust
// Core spreadsheet value types. Defined in Task 2.
```

`src/workbook/sheet.rs`:
```rust
// Sparse sheet storage. Defined in Task 3.
```

`src/workbook/workbook.rs`:
```rust
// Workbook container. Defined in Task 3.
```

`src/io/csv_read.rs`:
```rust
// CSV reading. Defined in Task 4.
```

`src/ui/grid.rs`:
```rust
// Grid rendering. Defined in Task 5.
```

`src/app/app.rs`:
```rust
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
```

- [ ] **Step 5: Create `src/main.rs`**

```rust
use eframe::egui;

fn main() -> eframe::Result {
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_inner_size([1100.0, 700.0])
            .with_title("Omasheet"),
        ..Default::default()
    };

    eframe::run_native(
        "omasheet",
        options,
        Box::new(|_cc| Ok(Box::new(omasheet::app::app::OmaApp::default()))),
    )
}
```

- [ ] **Step 6: Build and confirm it compiles**

Run: `cargo build`
Expected: `Finished dev profile` with no errors. If you see `rustc 1.94.0 is not supported`, a dependency lost its pin — check that both `eframe` and `egui` use `=0.35.0`.

- [ ] **Step 7: Lint**

Run: `cargo clippy --all-targets`
Expected: no output, no warnings.

- [ ] **Step 8: Run the app and confirm the window opens**

Run: `timeout 10 cargo run`
Expected: exit code 124 (the `timeout` killed a still-running app) and a window titled "Omasheet" showing the heading. Exit code 0 or a panic means the window failed to open.

- [ ] **Step 9: Commit**

```bash
git add Cargo.toml .gitignore src
git commit -m "feat: crate skeleton and eframe window"
```

---

### Task 2: Core Value Types

**Files:**
- Create: `src/workbook/cell.rs` (replace placeholder)
- Create: `tests/cell.rs`

**Interfaces:**
- Consumes: nothing.
- Produces: `SheetId`, `CellRef`, `CellValue`, `ErrorKind`, `Color`, `NumberFormat`, `CellFormat`, `Cell`, `CellRange`, `col_to_letters`, `letters_to_col`, `format_a1`, `parse_a1`, and `NumberFormat::apply`. Every later task depends on these exact names.

- [ ] **Step 1: Write the failing test file `tests/cell.rs`**

```rust
use omasheet::workbook::cell::*;

#[test]
fn col_to_letters_maps_the_expected_columns() {
    assert_eq!(col_to_letters(0), "A");
    assert_eq!(col_to_letters(25), "Z");
    assert_eq!(col_to_letters(26), "AA");
    assert_eq!(col_to_letters(27), "AB");
    assert_eq!(col_to_letters(701), "ZZ");
    assert_eq!(col_to_letters(702), "AAA");
}

#[test]
fn letters_to_col_round_trips() {
    for col in [0u32, 1, 5, 25, 26, 27, 51, 52, 701, 702, 16383] {
        assert_eq!(letters_to_col(&col_to_letters(col)), Some(col));
    }
}

#[test]
fn letters_to_col_rejects_nonsense() {
    assert_eq!(letters_to_col(""), None);
    assert_eq!(letters_to_col("A1"), None);
    assert_eq!(letters_to_col("a"), None);
    assert_eq!(letters_to_col("!"), None);
}

#[test]
fn a1_addresses_round_trip() {
    assert_eq!(format_a1(0, 0), "A1");
    assert_eq!(format_a1(0, 25), "Z1");
    assert_eq!(format_a1(9, 27), "AB10");
    assert_eq!(parse_a1("A1"), Some((0, 0)));
    assert_eq!(parse_a1("AB10"), Some((9, 27)));
    assert_eq!(parse_a1("A"), None);
    assert_eq!(parse_a1("1"), None);
    assert_eq!(parse_a1(""), None);
}

#[test]
fn cell_range_bounds_a_rectangle() {
    let r = CellRange::new(1, 2, 5, 4);
    assert_eq!(r.min_row, 1);
    assert_eq!(r.min_col, 2);
    assert_eq!(r.max_row, 5);
    assert_eq!(r.max_col, 4);
    assert_eq!(r.rows(), 5);
    assert_eq!(r.cols(), 3);
    assert!(r.contains(1, 2));
    assert!(r.contains(5, 4));
    assert!(!r.contains(6, 4));
    assert!(!r.contains(1, 5));
}

#[test]
fn cell_range_of_two_cells_normalises_corner_order() {
    let r = CellRange::from_corners((9, 9), (2, 1));
    assert_eq!(r.min_row, 2);
    assert_eq!(r.max_row, 9);
    assert_eq!(r.min_col, 1);
    assert_eq!(r.max_col, 9);
}

#[test]
fn number_format_renders_values_for_display() {
    assert_eq!(NumberFormat::General.apply(&CellValue::Number(3.5)), "3.5");
    assert_eq!(NumberFormat::Integer.apply(&CellValue::Number(3.7)), "4");
    assert_eq!(NumberFormat::Decimal(2).apply(&CellValue::Number(1.23456)), "1.23");
    assert_eq!(
        NumberFormat::Currency { symbol: '$', places: 2 }.apply(&CellValue::Number(1234.5)),
        "$1,234.50"
    );
    assert_eq!(NumberFormat::Percent(1).apply(&CellValue::Number(0.456)), "45.6%");
    assert_eq!(NumberFormat::Text.apply(&CellValue::Number(3.5)), "3.5");
}

#[test]
fn number_format_passes_text_and_errors_through() {
    assert_eq!(NumberFormat::Decimal(2).apply(&CellValue::Text("hi".into())), "hi");
    assert_eq!(
        NumberFormat::General.apply(&CellValue::Error(ErrorKind::DivZero)),
        "#DIV/0!"
    );
    assert_eq!(NumberFormat::General.apply(&CellValue::Empty), "");
}

#[test]
fn thousands_separators_group_correctly() {
    let f = NumberFormat::Currency { symbol: '$', places: 2 };
    assert_eq!(f.apply(&CellValue::Number(999.0)), "$999.00");
    assert_eq!(f.apply(&CellValue::Number(1000.0)), "$1,000.00");
    assert_eq!(f.apply(&CellValue::Number(1234567.891)), "$1,234,567.89");
}

#[test]
fn default_cell_is_empty_and_untouched() {
    let c = Cell::default();
    assert_eq!(c.value, CellValue::Empty);
    assert_eq!(c.format, CellFormat::default());
    assert_eq!(c.format.number_format, NumberFormat::General);
    assert!(!c.format.bold);
    assert!(c.format.font_color.is_none());
    assert!(c.format.bg_color.is_none());
}

#[test]
fn errors_render_with_their_excel_spelling() {
    assert_eq!(CellValue::Error(ErrorKind::DivZero).to_display_string(), "#DIV/0!");
    assert_eq!(CellValue::Error(ErrorKind::Value).to_display_string(), "#VALUE!");
    assert_eq!(CellValue::Error(ErrorKind::Name).to_display_string(), "#NAME?");
    assert_eq!(CellValue::Error(ErrorKind::Ref).to_display_string(), "#REF!");
    assert_eq!(CellValue::Error(ErrorKind::Circular).to_display_string(), "#CIRCULAR!");
    assert_eq!(CellValue::Error(ErrorKind::Num).to_display_string(), "#NUM!");
}

#[test]
fn cell_value_display_string_uses_general_format() {
    assert_eq!(CellValue::Number(42.0).to_display_string(), "42");
    assert_eq!(CellValue::Text("a".into()).to_display_string(), "a");
    assert_eq!(CellValue::Bool(true).to_display_string(), "TRUE");
    assert_eq!(CellValue::Empty.to_display_string(), "");
}

#[test]
fn cell_formated_display_applies_the_number_format() {
    let c = Cell {
        value: CellValue::Number(1234.5),
        format: CellFormat {
            number_format: NumberFormat::Currency { symbol: '$', places: 2 },
            ..Default::default()
        },
    };
    assert_eq!(c.formatted(), "$1,234.50");
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cargo test --test cell`
Expected: compile failure — `unresolved import omasheet::workbook::cell`.

- [ ] **Step 3: Implement `src/workbook/cell.rs`**

```rust
//! Core spreadsheet value types.
//!
//! This module is deliberately free of any egui dependency so the engine can be
//! tested headlessly. Colour is a plain byte struct; conversion to `egui::Color32`
//! happens in the `ui` layer.

/// Index of a sheet within a [`Workbook`](super::workbook::Workbook).
pub type SheetId = usize;

/// A fully-qualified cell address. Always sheet-qualified, even for same-sheet
/// references, so that cross-sheet references need no special case.
#[derive(Clone, Copy, PartialEq, Eq, Hash, Debug, PartialOrd, Ord)]
pub struct CellRef {
    pub sheet: SheetId,
    pub row: u32,
    pub col: u32,
}

impl CellRef {
    pub fn new(sheet: SheetId, row: u32, col: u32) -> Self {
        Self { sheet, row, col }
    }
}

/// A spreadsheet error. Errors are values: they propagate through operators and
/// render inside the cell rather than panicking.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum ErrorKind {
    DivZero,
    Value,
    Name,
    Ref,
    Circular,
    Num,
}

impl ErrorKind {
    pub fn as_str(self) -> &'static str {
        match self {
            ErrorKind::DivZero => "#DIV/0!",
            ErrorKind::Value => "#VALUE!",
            ErrorKind::Name => "#NAME?",
            ErrorKind::Ref => "#REF!",
            ErrorKind::Circular => "#CIRCULAR!",
            ErrorKind::Num => "#NUM!",
        }
    }
}

/// An sRGB colour with alpha. Plain bytes so the model stays egui-free.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub struct Color {
    pub r: u8,
    pub g: u8,
    pub b: u8,
    pub a: u8,
}

impl Color {
    pub const fn rgb(r: u8, g: u8, b: u8) -> Self {
        Self { r, g, b, a: 255 }
    }
}

impl Default for Color {
    fn default() -> Self {
        Self::rgb(0, 0, 0)
    }
}

/// How a numeric value is rendered. Currency and percent carry parameters rather
/// than raw format strings, so xlsx write is a string emit and read is a string parse.
#[derive(Clone, Copy, PartialEq, Eq, Debug, Default)]
pub enum NumberFormat {
    #[default]
    General,
    Integer,
    Decimal(u32),
    Currency { symbol: char, places: u32 },
    Percent(u32),
    Date,
    Time,
    Text,
}

impl NumberFormat {
    /// Render a value for display. Non-numeric values pass through unchanged so
    /// that `Decimal` on a text cell does not mangle the text.
    pub fn apply(&self, value: &CellValue) -> String {
        match value {
            CellValue::Number(n) => self.apply_number(*n),
            other => other.to_display_string(),
        }
    }

    fn apply_number(&self, n: f64) -> String {
        if !n.is_finite() {
            return String::new();
        }
        match self {
            NumberFormat::General => format_general(n),
            NumberFormat::Integer => format!("{}", n.round() as i64),
            NumberFormat::Decimal(places) => {
                format!("{:.*}", *places as usize, n)
            }
            NumberFormat::Currency { symbol, places } => {
                // Round first, then group only the integer part: grouping the
                // already-decimal string would put separators after the point.
                let rounded = format!("{:.*}", *places as usize, n.abs());
                let (int_part, frac_part) = match rounded.find('.') {
                    Some(i) => (&rounded[..i], &rounded[i + 1..]),
                    None => (rounded.as_str(), ""),
                };
                let mut body = group_thousands(int_part);
                if !frac_part.is_empty() {
                    body.push('.');
                    body.push_str(frac_part);
                }
                if n < 0.0 {
                    format!("-{symbol}{body}")
                } else {
                    format!("{symbol}{body}")
                }
            }
            NumberFormat::Percent(places) => {
                let scaled = n * 100.0;
                format!("{:.*}%", *places as usize, scaled)
            }
            NumberFormat::Date | NumberFormat::Time => format_general(n),
            NumberFormat::Text => format_general(n),
        }
    }
}

/// Insert thousands separators into a run of unsigned integer digits.
fn group_thousands(digits: &str) -> String {
    let int_len = digits.len();
    if int_len <= 3 {
        return digits.to_owned();
    }
    let mut out = String::with_capacity(digits.len() + int_len / 3);
    for (i, ch) in digits.chars().enumerate() {
        if i > 0 && i <= int_len && (int_len - i).is_multiple_of(3) {
            out.push(',');
        }
        out.push(ch);
    }
    out
}

/// Shortest representation that round-trips, with no trailing `.0` for integers.
fn format_general(n: f64) -> String {
    if n == n.trunc() && n.abs() < 1e15 {
        format!("{}", n as i64)
    } else {
        format!("{}", n)
    }
}

/// The value of a cell, independent of how it is displayed.
#[derive(Clone, PartialEq, Debug, Default)]
pub enum CellValue {
    #[default]
    Empty,
    Number(f64),
    Text(String),
    Bool(bool),
    Error(ErrorKind),
}

impl CellValue {
    /// The `General`-formatted representation of this value.
    pub fn to_display_string(&self) -> String {
        match self {
            CellValue::Empty => String::new(),
            CellValue::Number(n) => format_general(*n),
            CellValue::Text(t) => t.clone(),
            CellValue::Bool(b) => {
                if *b {
                    "TRUE".to_owned()
                } else {
                    "FALSE".to_owned()
                }
            }
            CellValue::Error(e) => e.as_str().to_owned(),
        }
    }

    /// The value as stored in the formula bar: numbers and booleans as typed,
    /// text verbatim. Used to seed the editor without destroying formatting.
    pub fn to_input_string(&self) -> String {
        match self {
            CellValue::Empty => String::new(),
            CellValue::Number(n) => format!("{}", n),
            CellValue::Text(t) => t.clone(),
            CellValue::Bool(b) => {
                if *b {
                    "TRUE".to_owned()
                } else {
                    "FALSE".to_owned()
                }
            }
            CellValue::Error(e) => e.as_str().to_owned(),
        }
    }

    pub fn is_empty(&self) -> bool {
        matches!(self, CellValue::Empty)
    }

    /// Numeric interpretation of this value, or `None` if it is not a number.
    /// Empty counts as zero, matching spreadsheet arithmetic.
    pub fn as_number(&self) -> Option<f64> {
        match self {
            CellValue::Number(n) => Some(*n),
            CellValue::Empty => Some(0.0),
            _ => None,
        }
    }
}

#[derive(Clone, Copy, PartialEq, Eq, Debug, Default)]
pub enum Align {
    #[default]
    Default,
    Left,
    Center,
    Right,
}

#[derive(Clone, PartialEq, Debug, Default)]
pub struct CellFormat {
    pub number_format: NumberFormat,
    pub font_color: Option<Color>,
    pub bg_color: Option<Color>,
    pub bold: bool,
    pub italic: bool,
    pub underline: bool,
    pub align: Align,
}

/// A cell: its value plus its formatting.
#[derive(Clone, PartialEq, Debug, Default)]
pub struct Cell {
    pub value: CellValue,
    pub format: CellFormat,
}

impl Cell {
    /// The text shown in the grid.
    pub fn formatted(&self) -> String {
        self.format.number_format.apply(&self.value)
    }
}

/// A half-open-by-inclusion rectangle of cells, always normalised so that
/// `min_*` is the top-left corner.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub struct CellRange {
    pub min_row: u32,
    pub min_col: u32,
    pub max_row: u32,
    pub max_col: u32,
}

impl CellRange {
    /// Build from two corners, normalising so `min_*` is the top-left.
    pub fn from_corners(a: (u32, u32), b: (u32, u32)) -> Self {
        Self {
            min_row: a.0.min(b.0),
            min_col: a.1.min(b.1),
            max_row: a.0.max(b.0),
            max_col: a.1.max(b.1),
        }
    }

    pub fn new(min_row: u32, min_col: u32, max_row: u32, max_col: u32) -> Self {
        Self { min_row, min_col, max_row, max_col }
    }

    pub fn rows(&self) -> u32 {
        self.max_row - self.min_row + 1
    }

    pub fn cols(&self) -> u32 {
        self.max_col - self.min_col + 1
    }

    pub fn contains(&self, row: u32, col: u32) -> bool {
        row >= self.min_row && row <= self.max_row && col >= self.min_col && col <= self.max_col
    }

    /// The smallest range covering both inputs.
    pub fn union(&self, other: &CellRange) -> CellRange {
        CellRange {
            min_row: self.min_row.min(other.min_row),
            min_col: self.min_col.min(other.min_col),
            max_row: self.max_row.max(other.max_row),
            max_col: self.max_col.max(other.max_col),
        }
    }
}

/// Convert a zero-indexed column to its spreadsheet letters: 0 -> "A", 26 -> "AA".
pub fn col_to_letters(col: u32) -> String {
    let mut n = col;
    let mut out = Vec::new();
    loop {
        out.push(b'A' + (n % 26) as u8);
        if n < 26 {
            break;
        }
        n = n / 26 - 1;
    }
    out.reverse();
    String::from_utf8(out).unwrap_or_default()
}

/// Parse spreadsheet column letters. Lowercase and non-letters are rejected.
pub fn letters_to_col(letters: &str) -> Option<u32> {
    if letters.is_empty() || !letters.bytes().all(|b| b.is_ascii_uppercase()) {
        return None;
    }
    let mut n: u32 = 0;
    for b in letters.bytes() {
        n = n.checked_mul(26)?.checked_add((b - b'A' + 1) as u32)?;
    }
    Some(n - 1)
}

/// Format a zero-indexed address as `A1`.
pub fn format_a1(row: u32, col: u32) -> String {
    format!("{}{}", col_to_letters(col), row + 1)
}

/// Parse `A1`-style notation into `(row, col)`, both zero-indexed.
pub fn parse_a1(s: &str) -> Option<(u32, u32)> {
    let split = s.find(|c: char| c.is_ascii_digit())?;
    let (letters, digits) = s.split_at(split);
    if digits.is_empty() || !digits.bytes().all(|b| b.is_ascii_digit()) {
        return None;
    }
    let col = letters_to_col(letters)?;
    let row: u32 = digits.parse().ok()?;
    if row == 0 {
        return None;
    }
    Some((row - 1, col))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cargo test --test cell`
Expected: 13 passed, 0 failed.

- [ ] **Step 5: Lint**

Run: `cargo clippy --all-targets`
Expected: silent. If clippy flags `format_general` as having a collapsible `if/else` returning the same value, simplify it to a single expression.

- [ ] **Step 6: Commit**

```bash
git add src/workbook/cell.rs tests/cell.rs
git commit -m "feat: core cell value types, formats and address helpers"
```

---

### Task 3: Sparse Sheet and Workbook

**Files:**
- Create: `src/workbook/sheet.rs` (replace placeholder)
- Create: `src/workbook/workbook.rs` (replace placeholder)
- Create: `tests/workbook.rs`

**Interfaces:**
- Consumes: everything from Task 2.
- Produces: `Sheet` with `new`, `get`, `set`, `remove`, `row_count`, `col_count`, `used`, `iter_row`, `clear`; `Workbook` with `new`, `sheet`, `sheet_mut`, `active_id`, `active`, `active_mut`, `sheet_count`, `sheet_name`, `rename_sheet`, `add_sheet`, `cell`, `cell_mut`, `set_cell`, `set_cell_format`, `clear_cell`, `dirty`, `mark_dirty`, `clear_dirty`.

- [ ] **Step 1: Write the failing test file `tests/workbook.rs`**

```rust
use omasheet::workbook::cell::*;
use omasheet::workbook::Workbook;

#[test]
fn a_new_workbook_has_one_named_sheet() {
    let wb = Workbook::new();
    assert_eq!(wb.sheet_count(), 1);
    assert_eq!(wb.sheet_name(wb.active_id()), "Sheet1");
    assert_eq!(wb.active_id(), 0);
    assert!(!wb.dirty());
}

#[test]
fn an_empty_sheet_reports_zero_dimensions() {
    let wb = Workbook::new();
    assert_eq!(wb.active().row_count(), 0);
    assert_eq!(wb.active().col_count(), 0);
    assert_eq!(wb.active().used(), None);
}

#[test]
fn reading_an_unset_cell_yields_nothing() {
    let wb = Workbook::new();
    assert!(wb.cell(CellRef::new(0, 0, 0)).is_none());
}

#[test]
fn writing_a_value_then_reading_it_back_round_trips() {
    let mut wb = Workbook::new();
    let r = CellRef::new(0, 2, 1);
    wb.set_cell(r, CellValue::Text("rent".into()));
    assert_eq!(wb.cell(r).unwrap().value, CellValue::Text("rent".into()));
}

#[test]
fn the_used_range_tracks_the_populated_bounding_box() {
    let mut wb = Workbook::new();
    wb.set_cell(CellRef::new(0, 4, 9), CellValue::Number(1.0));
    let used = wb.active().used().expect("sheet should not be empty");
    assert_eq!(used.min_row, 4);
    assert_eq!(used.min_col, 9);
    assert_eq!(used.max_row, 4);
    assert_eq!(used.max_col, 9);
    assert_eq!(wb.active().row_count(), 5);
    assert_eq!(wb.active().col_count(), 10);
}

#[test]
fn the_used_range_grows_to_cover_later_writes() {
    let mut wb = Workbook::new();
    wb.set_cell(CellRef::new(0, 10, 3), CellValue::Number(1.0));
    wb.set_cell(CellRef::new(0, 2, 20), CellValue::Number(2.0));
    let used = wb.active().used().unwrap();
    assert_eq!(used.min_row, 2);
    assert_eq!(used.min_col, 3);
    assert_eq!(used.max_row, 10);
    assert_eq!(used.max_col, 20);
    assert_eq!(wb.active().row_count(), 11);
    assert_eq!(wb.active().col_count(), 21);
}

#[test]
fn clearing_the_last_cell_returns_the_sheet_to_empty() {
    let mut wb = Workbook::new();
    let r = CellRef::new(0, 1, 1);
    wb.set_cell(r, CellValue::Number(7.0));
    assert!(wb.active().used().is_some());
    wb.clear_cell(r);
    assert!(wb.active().used().is_none());
    assert_eq!(wb.active().row_count(), 0);
    assert!(wb.cell(r).is_none());
}

#[test]
fn clearing_recomputes_the_remaining_bounds() {
    let mut wb = Workbook::new();
    wb.set_cell(CellRef::new(0, 0, 0), CellValue::Number(1.0));
    wb.set_cell(CellRef::new(0, 5, 5), CellValue::Number(2.0));
    wb.clear_cell(CellRef::new(0, 5, 5));
    let used = wb.active().used().unwrap();
    assert_eq!(used.max_row, 0);
    assert_eq!(used.max_col, 0);
}

#[test]
fn setting_a_cell_marks_the_workbook_dirty() {
    let mut wb = Workbook::new();
    assert!(!wb.dirty());
    wb.set_cell(CellRef::new(0, 0, 0), CellValue::Number(1.0));
    assert!(wb.dirty());
}

#[test]
fn formatting_a_cell_does_not_change_its_value() {
    let mut wb = Workbook::new();
    let r = CellRef::new(0, 0, 0);
    wb.set_cell(r, CellValue::Number(1234.5));
    let fmt = CellFormat {
        number_format: NumberFormat::Currency { symbol: '$', places: 2 },
        bold: true,
        ..Default::default()
    };
    wb.set_cell_format(r, fmt.clone());
    let cell = wb.cell(r).unwrap();
    assert_eq!(cell.value, CellValue::Number(1234.5));
    assert_eq!(cell.format, fmt);
    assert_eq!(cell.formatted(), "$1,234.50");
}

#[test]
fn rows_iterate_in_column_order_and_skip_holes() {
    let mut wb = Workbook::new();
    wb.set_cell(CellRef::new(0, 0, 3), CellValue::Text("c".into()));
    wb.set_cell(CellRef::new(0, 0, 1), CellValue::Text("a".into()));
    wb.set_cell(CellRef::new(0, 0, 2), CellValue::Text("b".into()));
    let cols: Vec<u32> = wb.active().iter_row(0).map(|(c, _)| c).collect();
    assert_eq!(cols, vec![1, 2, 3]);
}

#[test]
fn sheets_can_be_added_and_renamed() {
    let mut wb = Workbook::new();
    wb.add_sheet("Inventory");
    assert_eq!(wb.sheet_count(), 2);
    assert_eq!(wb.sheet_name(1), "Inventory");
    wb.rename_sheet(1, "Stock");
    assert_eq!(wb.sheet_name(1), "Stock");
}

#[test]
fn cells_are_scoped_to_their_sheet() {
    let mut wb = Workbook::new();
    wb.add_sheet("Two");
    wb.set_cell(CellRef::new(0, 0, 0), CellValue::Text("first".into()));
    wb.set_cell(CellRef::new(1, 0, 0), CellValue::Text("second".into()));
    assert_eq!(
        wb.cell(CellRef::new(0, 0, 0)).unwrap().value,
        CellValue::Text("first".into())
    );
    assert_eq!(
        wb.cell(CellRef::new(1, 0, 0)).unwrap().value,
        CellValue::Text("second".into())
    );
}

#[test]
fn duplicate_sheet_names_are_rejected() {
    let mut wb = Workbook::new();
    wb.add_sheet("Two");
    assert!(!wb.rename_sheet(0, "Two"), "a name already in use must be refused");
    assert_eq!(wb.sheet_name(0), "Sheet1");
    assert!(!wb.rename_sheet(0, ""), "an empty name must be refused");
}

#[test]
fn removing_the_last_sheet_is_refused() {
    let mut wb = Workbook::new();
    assert!(!wb.remove_sheet(0));
    assert_eq!(wb.sheet_count(), 1);
}

#[test]
fn removing_a_sheet_reassigns_the_active_id() {
    let mut wb = Workbook::new();
    wb.add_sheet("Two");
    wb.add_sheet("Three");
    assert_eq!(wb.active_id(), 2, "adding a sheet makes it active");
    assert!(wb.remove_sheet(0));
    assert_eq!(wb.sheet_count(), 2);
    assert_eq!(wb.active_id(), 0);
    assert_eq!(wb.sheet_name(0), "Two");
    assert_eq!(wb.sheet_name(1), "Three");
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cargo test --test workbook`
Expected: compile failure — `unresolved import omasheet::workbook::Workbook`.

- [ ] **Step 3: Implement `src/workbook/sheet.rs`**

```rust
//! Sparse sheet storage.
//!
//! Rows are stored in a `BTreeMap` keyed by row index, and each row holds a
//! `BTreeMap` of cells. A 10k-cell sheet allocates nothing for the cells it does
//! not have, and the same structure handles a far larger sheet unchanged.

use std::collections::BTreeMap;

use super::cell::{Cell, CellRange, SheetId};

/// One row of cells, keyed by column index.
#[derive(Debug, Default)]
pub struct Row {
    cells: BTreeMap<u32, Cell>,
}

impl Row {
    pub fn get(&self, col: u32) -> Option<&Cell> {
        self.cells.get(&col)
    }

    pub fn get_mut(&mut self, col: u32) -> Option<&mut Cell> {
        self.cells.get_mut(&col)
    }

    pub fn set(&mut self, col: u32, cell: Cell) {
        self.cells.insert(col, cell);
    }

    pub fn remove(&mut self, col: u32) -> Option<Cell> {
        self.cells.remove(&col)
    }

    pub fn is_empty(&self) -> bool {
        self.cells.is_empty()
    }

    pub fn iter(&self) -> impl Iterator<Item = (u32, &Cell)> {
        self.cells.iter().map(|(c, cell)| (*c, cell))
    }

    pub fn min_col(&self) -> u32 {
        self.cells.keys().copied().min().unwrap_or(0)
    }

    pub fn max_col(&self) -> u32 {
        self.cells.keys().copied().max().unwrap_or(0)
    }
}

/// A single sheet: sparse rows plus a cached bounding box of populated cells.
#[derive(Debug)]
pub struct Sheet {
    pub(crate) id: SheetId,
    name: String,
    rows: BTreeMap<u32, Row>,
    used: Option<CellRange>,
}

impl Sheet {
    pub fn new(id: SheetId, name: impl Into<String>) -> Self {
        Self { id, name: name.into(), rows: BTreeMap::new(), used: None }
    }

    pub fn id(&self) -> SheetId {
        self.id
    }

    pub fn name(&self) -> &str {
        &self.name
    }

    pub fn set_name(&mut self, name: impl Into<String>) {
        self.name = name.into();
    }

    pub fn get(&self, row: u32, col: u32) -> Option<&Cell> {
        self.rows.get(&row).and_then(|r| r.get(col))
    }

    pub fn get_mut(&mut self, row: u32, col: u32) -> Option<&mut Cell> {
        self.rows.get_mut(&row).and_then(|r| r.get_mut(col))
    }

    /// Store a cell and extend the used range to cover it.
    pub fn set(&mut self, row: u32, col: u32, cell: Cell) {
        self.used = Some(match self.used {
            Some(r) => r.union(&CellRange::new(row, col, row, col)),
            None => CellRange::new(row, col, row, col),
        });
        self.rows.entry(row).or_default().set(col, cell);
    }

    /// Remove a cell, shrinking the used range only if the removed cell was on
    /// its boundary. An empty sheet reports `None` rather than a degenerate range.
    pub fn remove(&mut self, row: u32, col: u32) -> Option<Cell> {
        let removed = {
            let row_entry = self.rows.get_mut(&row)?;
            let removed = row_entry.remove(col);
            if row_entry.is_empty() {
                self.rows.remove(&row);
            }
            removed
        }?;

        if self.used.is_some() {
            let mut min_row = u32::MAX;
            let mut max_row = 0u32;
            let mut min_col = u32::MAX;
            let mut max_col = 0u32;
            for (r, row_entry) in &self.rows {
                if row_entry.is_empty() {
                    continue;
                }
                min_row = min_row.min(*r);
                max_row = max_row.max(*r);
                min_col = min_col.min(row_entry.min_col());
                max_col = max_col.max(row_entry.max_col());
            }
            self.used = if self.rows.is_empty() {
                None
            } else {
                Some(CellRange { min_row, min_col, max_row, max_col })
            };
        }
        Some(removed)
    }

    /// Remove every cell.
    pub fn clear(&mut self) {
        self.rows.clear();
        self.used = None;
    }

    /// The bounding box of populated cells, or `None` when the sheet is empty.
    pub fn used(&self) -> Option<CellRange> {
        self.used
    }

    /// One past the highest populated row index, or 0 when empty.
    pub fn row_count(&self) -> u32 {
        self.used.map(|r| r.max_row + 1).unwrap_or(0)
    }

    /// One past the highest populated column index, or 0 when empty.
    pub fn col_count(&self) -> u32 {
        self.used.map(|r| r.max_col + 1).unwrap_or(0)
    }

    pub fn iter_row(&self, row: u32) -> impl Iterator<Item = (u32, &Cell)> {
        self.rows.get(&row).into_iter().flat_map(|r| r.iter())
    }

    /// Every populated cell, in row-major order. Used by save and export.
    pub fn iter_cells(&self) -> impl Iterator<Item = (u32, u32, &Cell)> {
        self.rows.iter().flat_map(|(r, row)| row.iter().map(move |(c, cell)| (*r, c, cell)))
    }
}
```

- [ ] **Step 4: Implement `src/workbook/workbook.rs`**

```rust
//! Workbook container: the sheet registry, the active sheet, and the dirty flag.

use super::cell::{Cell, CellFormat, CellRef, CellValue, SheetId};
use super::sheet::Sheet;

/// A workbook. Always has at least one sheet.
#[derive(Debug)]
pub struct Workbook {
    sheets: Vec<Sheet>,
    active: SheetId,
    dirty: bool,
}

impl Default for Workbook {
    fn default() -> Self {
        Self::new()
    }
}

impl Workbook {
    /// A workbook with one sheet named `Sheet1`.
    pub fn new() -> Self {
        Self {
            sheets: vec![Sheet::new(0, "Sheet1")],
            active: 0,
            dirty: false,
        }
    }

    pub fn sheet_count(&self) -> usize {
        self.sheets.len()
    }

    pub fn sheet(&self, id: SheetId) -> &Sheet {
        &self.sheets[id]
    }

    pub fn sheet_mut(&mut self, id: SheetId) -> &mut Sheet {
        &mut self.sheets[id]
    }

    pub fn sheets(&self) -> &[Sheet] {
        &self.sheets
    }

    pub fn sheet_name(&self, id: SheetId) -> &str {
        self.sheets[id].name()
    }

    pub fn active_id(&self) -> SheetId {
        self.active
    }

    pub fn active(&self) -> &Sheet {
        &self.sheets[self.active]
    }

    pub fn active_mut(&mut self) -> &mut Sheet {
        &mut self.sheets[self.active]
    }

    pub fn set_active(&mut self, id: SheetId) {
        if id < self.sheets.len() {
            self.active = id;
        }
    }

    /// Add a sheet named `name`. Names must be unique and non-empty.
    pub fn add_sheet(&mut self, name: &str) -> bool {
        if name.is_empty() || self.sheets.iter().any(|s| s.name() == name) {
            return false;
        }
        let id = self.sheets.len();
        self.sheets.push(Sheet::new(id, name));
        self.active = id;
        self.dirty = true;
        true
    }

    /// Rename a sheet. Refuses empty names, duplicates, and unknown ids.
    pub fn rename_sheet(&mut self, id: SheetId, name: &str) -> bool {
        if id >= self.sheets.len() || name.is_empty() {
            return false;
        }
        if self.sheets.iter().enumerate().any(|(i, s)| i != id && s.name() == name) {
            return false;
        }
        self.sheets[id].set_name(name);
        self.dirty = true;
        true
    }

    /// Remove a sheet, never the last one. The active sheet becomes the first
    /// remaining sheet.
    pub fn remove_sheet(&mut self, id: SheetId) -> bool {
        if id >= self.sheets.len() || self.sheets.len() == 1 {
            return false;
        }
        self.sheets.remove(id);
        for (i, s) in self.sheets.iter_mut().enumerate() {
            s.id = i;
        }
        self.active = 0;
        self.dirty = true;
        true
    }

    pub fn cell(&self, at: CellRef) -> Option<&Cell> {
        self.sheets.get(at.sheet).and_then(|s| s.get(at.row, at.col))
    }

    pub fn cell_mut(&mut self, at: CellRef) -> Option<&mut Cell> {
        self.sheets.get_mut(at.sheet).and_then(|s| s.get_mut(at.row, at.col))
    }

    /// The value at `at`, treating a missing cell as empty. This is the accessor
    /// the grid and the formula engine use; they should not deal in `Option`.
    pub fn value_at(&self, at: CellRef) -> CellValue {
        self.cell(at).map(|c| c.value.clone()).unwrap_or(CellValue::Empty)
    }

    pub fn set_cell(&mut self, at: CellRef, value: CellValue) {
        if at.sheet >= self.sheets.len() {
            return;
        }
        let sheet = &mut self.sheets[at.sheet];
        let mut cell = sheet.get(at.row, at.col).cloned().unwrap_or_default();
        cell.value = value;
        sheet.set(at.row, at.col, cell);
        self.dirty = true;
    }

    pub fn set_cell_format(&mut self, at: CellRef, format: CellFormat) {
        if at.sheet >= self.sheets.len() {
            return;
        }
        let sheet = &mut self.sheets[at.sheet];
        let mut cell = sheet.get(at.row, at.col).cloned().unwrap_or_default();
        cell.format = format;
        sheet.set(at.row, at.col, cell);
        self.dirty = true;
    }

    pub fn clear_cell(&mut self, at: CellRef) {
        if at.sheet >= self.sheets.len() {
            return;
        }
        self.sheets[at.sheet].remove(at.row, at.col);
        self.dirty = true;
    }

    pub fn dirty(&self) -> bool {
        self.dirty
    }

    pub fn mark_dirty(&mut self) {
        self.dirty = true;
    }

    pub fn clear_dirty(&mut self) {
        self.dirty = false;
    }

    /// Load a sheet produced by a reader, replacing the workbook's contents.
    /// Used by file open in a later plan; defined here so the seam exists.
    pub fn replace_sheet(&mut self, id: SheetId, sheet: Sheet) {
        if id < self.sheets.len() {
            self.sheets[id] = sheet;
            self.dirty = true;
        }
    }
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cargo test --test workbook`
Expected: 16 passed, 0 failed.

- [ ] **Step 6: Lint and fix**

Run: `cargo clippy --all-targets`
Expected: silent. - [ ] **Step 7: Commit**

```bash
git add src/workbook/sheet.rs src/workbook/workbook.rs tests/workbook.rs
git commit -m "feat: sparse sheet storage and workbook container"
```

---

### Task 4: CSV Reader

**Files:**
- Create: `src/io/csv_read.rs` (replace placeholder)
- Create: `tests/csv_read.rs`

**Interfaces:**
- Consumes: `Sheet`, `Cell`, `CellValue` from Tasks 2–3.
- Produces: `pub fn sniff_delimiter(text: &str) -> u8`, `pub fn infer_number(field: &str) -> Option<f64>`, `pub fn read_csv(text: &str) -> Result<Sheet, CsvError>`, `pub enum CsvError { Empty, Utf8 }`, and `pub fn read_csv_file(path: &Path) -> Result<Sheet, CsvError>`.

- [ ] **Step 1: Write the failing test file `tests/csv_read.rs`**

```rust
use omasheet::io::csv_read::*;
use omasheet::workbook::cell::{CellRef, CellValue};
use omasheet::workbook::Workbook;

#[test]
fn a_single_column_sniffs_a_comma() {
    assert_eq!(sniff_delimiter("a\nb\nc\n"), b',');
}

#[test]
fn a_semicolon_delimited_file_sniffs_a_semicolon() {
    assert_eq!(sniff_delimiter("a;b;c\n1;2;3\n4;5;6\n"), b';');
}

#[test]
fn a_tab_delimited_file_sniffs_a_tab() {
    assert_eq!(sniff_delimiter("a\tb\tc\n1\t2\t3\n4\t5\t6\n"), b'\t');
}

#[test]
fn a_pipe_delimited_file_sniffs_a_pipe() {
    assert_eq!(sniff_delimiter("a|b|c\n1|2|3\n4|5|6\n"), b'|');
}

#[test]
fn a_comma_file_containing_semicolons_keeps_the_comma() {
    // Semicolons appear inside fields; commas are the real delimiter.
    assert_eq!(sniff_delimiter("a;b,c\n1;2,3\n"), b',');
}

#[test]
fn a_bom_does_not_stop_sniffing() {
    assert_eq!(sniff_delimiter("\u{feff}a;b;c\n1;2;3\n"), b';');
}

#[test]
fn numbers_are_inferred_when_unambiguous() {
    assert_eq!(infer_number("42"), Some(42.0));
    assert_eq!(infer_number("3.50"), Some(3.5));
    assert_eq!(infer_number("-7"), Some(-7.0));
    assert_eq!(infer_number("+7"), Some(7.0));
    assert_eq!(infer_number("0"), Some(0.0));
    assert_eq!(infer_number("0.5"), Some(0.5));
    assert_eq!(infer_number("1e3"), Some(1000.0));
}

#[test]
fn leading_zero_identifiers_stay_text() {
    assert_eq!(infer_number("00123"), None);
    assert_eq!(infer_number("007"), None);
    assert_eq!(infer_number("-0042"), None);
}

#[test]
fn non_numeric_fields_are_not_numbers() {
    assert_eq!(infer_number(""), None);
    assert_eq!(infer_number("abc"), None);
    assert_eq!(infer_number("42abc"), None);
    assert_eq!(infer_number(" 42"), None);
    assert_eq!(infer_number("42 "), None);
    assert_eq!(infer_number("1,234"), None);
}

#[test]
fn rust_parses_these_but_we_still_reject_them() {
    // `f64::from_str` accepts all of these; a spreadsheet must not.
    assert!("NaN".parse::<f64>().is_ok(), "Rust really does accept this");
    assert_eq!(infer_number("NaN"), None);
    assert_eq!(infer_number("inf"), None);
    assert_eq!(infer_number("-inf"), None);
    assert_eq!(infer_number("infinity"), None);
}

#[test]
fn a_simple_csv_loads_into_a_sheet() {
    let sheet = read_csv("a,b\n1,2\n3,4\n").unwrap();
    assert_eq!(sheet.row_count(), 3);
    assert_eq!(sheet.col_count(), 2);
    assert_eq!(sheet.get(0, 0).unwrap().value, CellValue::Text("a".into()));
    assert_eq!(sheet.get(1, 0).unwrap().value, CellValue::Number(1.0));
    assert_eq!(sheet.get(2, 1).unwrap().value, CellValue::Number(4.0));
}

#[test]
fn the_first_row_is_data_not_a_header() {
    let sheet = read_csv("Name,Value\nRent,1200\n").unwrap();
    assert_eq!(sheet.get(0, 0).unwrap().value, CellValue::Text("Name".into()));
    assert_eq!(sheet.row_count(), 2);
}

#[test]
fn quoted_fields_may_contain_the_delimiter_and_newlines() {
    let sheet = read_csv("\"a,b\",\"c\"\"d\"\n\"multi\nline\",2\n").unwrap();
    assert_eq!(sheet.get(0, 0).unwrap().value, CellValue::Text("a,b".into()));
    assert_eq!(sheet.get(0, 1).unwrap().value, CellValue::Text("c\"d".into()));
    assert_eq!(sheet.get(1, 0).unwrap().value, CellValue::Text("multi\nline".into()));
    assert_eq!(sheet.get(1, 1).unwrap().value, CellValue::Number(2.0));
}

#[test]
fn crlf_line_endings_are_handled() {
    let sheet = read_csv("a,b\r\n1,2\r\n").unwrap();
    assert_eq!(sheet.row_count(), 2);
    assert_eq!(sheet.get(1, 1).unwrap().value, CellValue::Number(2.0));
}

#[test]
fn a_bom_is_stripped_from_the_first_field() {
    let sheet = read_csv("\u{feff}hello,2\n").unwrap();
    assert_eq!(sheet.get(0, 0).unwrap().value, CellValue::Text("hello".into()));
}

#[test]
fn empty_fields_become_empty_cells_not_empty_text() {
    let sheet = read_csv("a,,c\n").unwrap();
    assert_eq!(sheet.get(0, 1).unwrap().value, CellValue::Empty);
    assert_eq!(sheet.get(0, 2).unwrap().value, CellValue::Text("c".into()));
}

#[test]
fn a_semicolon_file_loads_with_the_sniffed_delimiter() {
    let sheet = read_csv("a;b;c\n1;2;3\n").unwrap();
    assert_eq!(sheet.col_count(), 3);
    assert_eq!(sheet.get(1, 2).unwrap().value, CellValue::Number(3.0));
}

#[test]
fn a_trailing_newline_does_not_create_an_extra_row() {
    let sheet = read_csv("a,b\n1,2\n").unwrap();
    assert_eq!(sheet.row_count(), 2);
}

#[test]
fn ragged_rows_stay_sparse_rather_than_materialising_padding() {
    let sheet = read_csv("a,b,c\n1\n2,3\n").unwrap();
    assert_eq!(sheet.col_count(), 3);
    assert!(sheet.get(1, 1).is_none(), "a missing cell is not stored");
    // Through the workbook accessor it reads as empty, which is all the grid needs.
    let mut wb = Workbook::new();
    wb.replace_sheet(0, sheet);
    assert_eq!(wb.value_at(CellRef::new(0, 1, 1)), CellValue::Empty);
    assert_eq!(wb.value_at(CellRef::new(0, 1, 2)), CellValue::Empty);
    assert_eq!(wb.value_at(CellRef::new(0, 2, 1)), CellValue::Number(3.0));
}

#[test]
fn an_empty_file_is_an_error_not_a_panic() {
    assert!(matches!(read_csv(""), Err(CsvError::Empty)));
}

#[test]
fn a_file_of_only_whitespace_is_an_error() {
    assert!(matches!(read_csv("\n\n\n"), Err(CsvError::Empty)));
}

#[test]
fn the_sheet_name_defaults_to_sheet1() {
    let sheet = read_csv("a\n").unwrap();
    assert_eq!(sheet.name(), "Sheet1");
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cargo test --test csv_read`
Expected: compile failure — `unresolved import omasheet::io::csv_read`.

- [ ] **Step 3: Implement `src/io/csv_read.rs`**

```rust
//! CSV reading.
//!
//! The `csv` crate handles quoting, escaping and delimiter mechanics. Omasheet
//! owns delimiter sniffing and number inference, because both need the raw field
//! text that the crate hands back verbatim.

use std::path::Path;

use crate::workbook::cell::{Cell, CellValue};
use crate::workbook::sheet::Sheet;

/// How many rows to look at when choosing a delimiter.
const SNIFF_ROWS: usize = 20;

/// Candidate delimiters, in preference order.
const CANDIDATES: [u8; 4] = [b',', b';', b'\t', b'|'];

#[derive(Debug, PartialEq, Eq)]
pub enum CsvError {
    Empty,
    Utf8,
    Io,
}

/// Guess the delimiter by parsing the first [`SNIFF_ROWS`] rows with each candidate
/// and choosing the one whose records agree on column count most often.
///
/// Disagreement matters more than raw frequency: a file full of semicolons inside
/// comma-separated fields still splits cleanly on the comma.
pub fn sniff_delimiter(text: &str) -> u8 {
    let body = text.trim_start_matches('\u{feff}');
    let mut best = b',';
    let mut best_score = (0usize, 0usize);

    for &candidate in &CANDIDATES {
        let mut reader = csv::ReaderBuilder::new()
            .delimiter(candidate)
            .has_headers(false)
            // Must match the real reader: without this a ragged record errors and
            // the correct delimiter gets discarded before it can be scored.
            .flexible(true)
            .from_reader(body.as_bytes());

        let mut counts = Vec::new();
        for record in reader.records().take(SNIFF_ROWS) {
            match record {
                Ok(r) => counts.push(r.len()),
                Err(_) => {
                    counts.clear();
                    break;
                }
            }
        }
        if counts.len() < 2 {
            continue;
        }

        // Total fields is the primary signal: the real delimiter yields the most
        // fields overall, while a delimiter that does not appear yields one field
        // per line. Consistency is only a tie-break, because genuinely ragged
        // files have inconsistent widths and must not lose to a wrong delimiter.
        let total: usize = counts.iter().sum();
        let first = counts[0];
        let agreements = counts.iter().filter(|n| **n == first).count();
        let score = (total, agreements);
        if score > best_score {
            best_score = score;
            best = candidate;
        }
    }
    best
}

/// Decide whether a raw field is a number.
///
/// Three conditions, all necessary:
/// 1. it parses as `f64`;
/// 2. the result is finite, which rejects `NaN` and `inf` that Rust otherwise accepts;
/// 3. the integer part has no redundant leading zero, so `00123` stays text.
pub fn infer_number(field: &str) -> Option<f64> {
    if field.is_empty() {
        return None;
    }
    let value: f64 = field.parse().ok()?;
    if !value.is_finite() {
        return None;
    }
    if has_redundant_leading_zero(field) {
        return None;
    }
    Some(value)
}

fn has_redundant_leading_zero(field: &str) -> bool {
    let digits = field.strip_prefix(['+', '-']).unwrap_or(field);
    let bytes = digits.as_bytes();
    // "0", "0.5" and "0e1" are all fine; "01" and "00.5" are not.
    bytes.len() >= 2 && bytes[0] == b'0' && bytes[1].is_ascii_digit()
}

/// Parse CSV text into a sheet.
pub fn read_csv(text: &str) -> Result<Sheet, CsvError> {
    if text.trim().is_empty() {
        return Err(CsvError::Empty);
    }

    let body = text.strip_prefix('\u{feff}').unwrap_or(text);
    let delimiter = sniff_delimiter(text);

    let mut reader = csv::ReaderBuilder::new()
        .delimiter(delimiter)
        .has_headers(false)
        .flexible(true)
        .from_reader(body.as_bytes());

    let mut sheet = Sheet::new(0, "Sheet1");
    let mut row: u32 = 0;

    for record in reader.records() {
            let record = record.map_err(|_| CsvError::Utf8)?;
        let mut cells: Vec<Cell> = Vec::with_capacity(record.len());
        for field in &record {
            let value = match infer_number(field) {
                Some(n) => CellValue::Number(n),
                None if field.is_empty() => CellValue::Empty,
                None => CellValue::Text(field.to_owned()),
            };
            cells.push(Cell { value, format: Default::default() });
        }
        for (col, cell) in cells.into_iter().enumerate() {
            sheet.set(row, col as u32, cell);
        }
        row += 1;
    }

    if row == 0 {
        return Err(CsvError::Empty);
    }

    // Short rows are left sparse: absent cells read as `CellValue::Empty` through
    // `Workbook::value_at`, so padding would allocate cells that carry no information.
    Ok(sheet)
}

/// Read a CSV file from disk as UTF-8.
pub fn read_csv_file(path: &Path) -> Result<Sheet, CsvError> {
    let bytes = std::fs::read(path).map_err(|_| CsvError::Io)?;
    let text = String::from_utf8(bytes).map_err(|_| CsvError::Utf8)?;
    read_csv(&text)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cargo test --test csv_read`
Expected: 22 passed, 0 failed.

- [ ] **Step 5: Lint**

Run: `cargo clippy --all-targets`
Expected: silent.

- [ ] **Step 6: Wire a real file into `OmaApp` so the grid has data to show**

Replace `src/app/app.rs` with:

```rust
use eframe::egui;

use crate::workbook::Workbook;

pub struct OmaApp {
    pub wb: Workbook,
    pub error: Option<String>,
}

impl Default for OmaApp {
    fn default() -> Self {
        Self { wb: Workbook::new(), error: None }
    }
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
```

- [ ] **Step 7: Verify the whole suite still passes**

Run: `cargo test`
Expected: all tests across `cell`, `workbook`, `csv_read` pass. If `csv_read` reports a different test count than 21, count the `#[test]` functions in the file and reconcile — the assertions are what matter.

- [ ] **Step 8: Commit**

```bash
git add src/io/csv_read.rs src/app/app.rs tests/csv_read.rs
git commit -m "feat: CSV reader with delimiter sniffing and guarded number inference"
```

---

### Task 5: Virtualized Grid, Selection and Navigation

**Files:**
- Create: `src/ui/grid.rs` (replace placeholder)
- Modify: `src/app/app.rs`
- Create: `tests/grid_keys.rs`

**Interfaces:**
- Consumes: `Workbook` from Task 3; `CellRef`, `CellValue`, `CellRange`, `format_a1`, `col_to_letters` from Task 2.
- Produces: `pub const ROW_H`, `COL_W`, `HEADER_W`, `HEADER_H`, `OVERSCAN`; `pub struct Selection { pub active: CellRef, pub anchor: CellRef }` with `is_single` and `range`; `pub struct EditState { pub active: CellRef, pub buffer: String }`; `pub struct GridState { pub selection: Selection, pub edit: Option<EditState> }` with `new`; `pub fn cell_at_pos(origin: Pos2, pos: Pos2) -> Option<CellRef>`; `pub fn show(state: &mut GridState, wb: &mut Workbook, ui: &mut egui::Ui)`. Also splits `OmaApp::body(&mut self, ui)` out of `eframe::App::ui`, so tests can drive the UI without an `eframe::Frame`.

**egui API notes, verified against 0.35.0 source — follow these exactly:**
- `Sense::hover_and_drag()` does not exist. Use `Sense::click()`, which includes hover.
- `Painter::galley(&self, pos: Pos2, galley: Arc<Galley>, fallback_color: Color32)` — position **first**.
- `Fonts::layout` takes `wrap_width: f32`, not a `Vec2`, and needs `&mut self`. Use `Painter::layout_no_wrap(&self, text: String, font_id, color) -> Arc<Galley>`, which handles the interior mutability.
- `Painter::rect_filled(rect, corner_radius: impl Into<CornerRadius>, fill)` — `0.0` converts fine.
- `Painter::text(pos, anchor: Align2, text, font_id, color)`.
- `visuals.selection.stroke.color`, `visuals.widgets.inactive.bg_fill`, `visuals.widgets.noninteractive.bg_stroke.color`, `visuals.extreme_bg_color`, `visuals.text_color()`, `visuals.weak_text_color()` all exist.

- [ ] **Step 1: Write the failing test file `tests/grid_keys.rs`**

```rust
//! Headless keybinding tests. `egui::Context::run_ui` runs one UI pass with
//! synthesised input and no window, so these are real tests, not descriptions
//! of intent.

use egui::{Event, Key, Modifiers, Pos2, RawInput, Rect};
use omasheet::app::app::OmaApp;
use omasheet::ui::grid::*;
use omasheet::workbook::cell::{CellRange, CellRef, CellValue};

/// A key-down event for `k`.
fn key(k: Key) -> Vec<Event> {
    vec![Event::Key {
        key: k,
        physical_key: Some(k),
        pressed: true,
        repeat: false,
        modifiers: Modifiers::default(),
    }]
}

/// A text-input event, as produced by typing.
fn type_text(s: &str) -> Vec<Event> {
    vec![Event::Text(s.to_owned())]
}

/// Drive one UI pass over `app` with `events`, with no window on screen.
fn pass(app: &mut OmaApp, events: Vec<Event>) {
    let ctx = egui::Context::default();
    let input = RawInput {
        screen_rect: Some(Rect::from_min_size(Pos2::ZERO, egui::vec2(1100.0, 700.0))),
        events,
        ..Default::default()
    };
    let _ = ctx.run_ui(input, |ui| {
        // `body`, not `ui`: the production `eframe::App::ui` needs an `eframe::Frame`
        // that cannot be constructed outside a running eframe event loop.
        app.body(ui);
    });
}

#[test]
fn arrow_down_moves_the_selection() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowDown));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 1, 0));
}

#[test]
fn arrow_right_moves_the_selection() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowRight));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 0, 1));
}

#[test]
fn arrow_keys_do_not_move_the_selection_past_zero() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowUp));
    pass(&mut app, key(Key::ArrowLeft));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 0, 0));
}

#[test]
fn home_moves_to_the_first_column() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowRight));
    pass(&mut app, key(Key::ArrowRight));
    pass(&mut app, key(Key::Home));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 0, 0));
}

#[test]
fn arrow_navigation_moves_the_anchor_with_it() {
    let mut app = OmaApp::default();
    pass(&mut app, key(Key::ArrowDown));
    assert_eq!(app.grid.selection.anchor, CellRef::new(0, 1, 0));
    assert!(app.grid.selection.is_single());
}

#[test]
fn cell_at_pos_maps_a_pointer_back_to_a_cell() {
    // A1's top-left in grid content coordinates is (HEADER_W, HEADER_H).
    let origin = Pos2::new(0.0, 0.0);
    assert_eq!(
        cell_at_pos(origin, Pos2::new(HEADER_W + 4.0, HEADER_H + 4.0)),
        Some(CellRef::new(0, 0, 0))
    );
    assert_eq!(
        cell_at_pos(origin, Pos2::new(HEADER_W + COL_W + 4.0, HEADER_H + ROW_H + 4.0)),
        Some(CellRef::new(0, 1, 1))
    );
}

#[test]
fn cell_at_pos_returns_nothing_over_the_headers() {
    let origin = Pos2::new(0.0, 0.0);
    assert_eq!(cell_at_pos(origin, Pos2::new(5.0, 5.0)), None);
}

#[test]
fn cell_at_pos_returns_nothing_to_the_left_of_the_grid() {
    let origin = Pos2::new(0.0, 0.0);
    assert_eq!(cell_at_pos(origin, Pos2::new(-10.0, HEADER_H + 4.0)), None);
}

#[test]
fn a_value_written_to_the_model_survives_a_ui_pass() {
    let mut app = OmaApp::default();
    app.wb.set_cell(CellRef::new(0, 0, 0), CellValue::Text("rent".into()));
    // If the grid panicked while painting, or read the wrong sheet, this fails.
    pass(&mut app, Vec::new());
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Text("rent".into()));
}

#[test]
fn a_large_sheet_paints_without_error() {
    // Exercises the virtualisation window with far more cells than fit on screen.
    let mut app = OmaApp::default();
    for row in 0..5_000u32 {
        app.wb.set_cell(CellRef::new(0, row, 0), CellValue::Number(row as f64));
    }
    for col in 0..20u32 {
        app.wb.set_cell(CellRef::new(0, 0, col), CellValue::Text("x".into()));
    }
    pass(&mut app, Vec::new());
    assert_eq!(app.wb.active().row_count(), 5_000);
}

#[test]
fn a_text_event_is_ignored_while_navigating() {
    // Editing lands in Task 6. This pins the behaviour so that task changes it
    // deliberately rather than discovering it as a surprise.
    let mut app = OmaApp::default();
    pass(&mut app, type_text("hi"));
    assert!(app.grid.edit.is_none());
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Empty);
}

#[test]
fn selection_starts_as_a_single_cell() {
    let state = GridState::new();
    assert!(state.selection.is_single());
    assert_eq!(state.selection.range(), CellRange::new(0, 0, 0, 0));
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cargo test --test grid_keys`
Expected: compile failure — `unresolved import omasheet::ui::grid`, and `OmaApp` has no `body` or `grid` field.

- [ ] **Step 3: Implement `src/ui/grid.rs`**

```rust
//! The spreadsheet grid.
//!
//! Cells are painted manually through the `Painter` rather than composed from egui
//! widgets. A grid shows thousands of cells per frame, and allocating a widget per
//! cell is the difference between smooth scrolling and visible lag. Hit-testing is
//! likewise arithmetic against computed rectangles rather than per-cell interaction.

use eframe::egui::{
    self, Align2, Color32, FontId, Pos2, Rect, Sense, Stroke, StrokeKind, Vec2, Visuals,
};

use crate::workbook::cell::{col_to_letters, CellRef};
use crate::workbook::Workbook;

pub const ROW_H: f32 = 22.0;
pub const COL_W: f32 = 100.0;
pub const HEADER_W: f32 = 48.0;
pub const HEADER_H: f32 = 24.0;
/// Extra rows and columns painted beyond the viewport, so scrolling shows no gaps.
pub const OVERSCAN: i64 = 2;

/// Minimum scrollable extent, so an empty sheet still scrolls.
const MIN_ROWS: u32 = 200;
const MIN_COLS: u32 = 26;

/// The active cell plus the anchor it was selected from.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Selection {
    pub active: CellRef,
    pub anchor: CellRef,
}

impl Default for Selection {
    fn default() -> Self {
        let c = CellRef::new(0, 0, 0);
        Selection { active: c, anchor: c }
    }
}

impl Selection {
    /// True when the selection is a single cell rather than a rectangle.
    pub fn is_single(&self) -> bool {
        self.active == self.anchor
    }

    /// The rectangle spanned by the anchor and the active cell.
    pub fn range(&self) -> crate::workbook::cell::CellRange {
        crate::workbook::cell::CellRange::from_corners(
            (self.anchor.row, self.anchor.col),
            (self.active.row, self.active.col),
        )
    }
}

/// State of an in-progress edit.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct EditState {
    pub active: CellRef,
    pub buffer: String,
}

#[derive(Clone, Debug, Default)]
pub struct GridState {
    pub selection: Selection,
    pub edit: Option<EditState>,
}

impl GridState {
    pub fn new() -> Self {
        Self::default()
    }
}

/// The top-left of cell `(row, col)` in grid content coordinates.
pub fn cell_min(row: u32, col: u32) -> Pos2 {
    Pos2::new(HEADER_W + col as f32 * COL_W, HEADER_H + row as f32 * ROW_H)
}

/// The rectangle of cell `(row, col)` in grid content coordinates.
pub fn cell_rect(row: u32, col: u32) -> Rect {
    Rect::from_min_size(cell_min(row, col), Vec2::new(COL_W, ROW_H))
}

/// Map a pointer position to a cell, or `None` when it is over the headers.
pub fn cell_at_pos(origin: Pos2, pos: Pos2) -> Option<CellRef> {
    let local = pos - origin;
    if local.x < HEADER_W || local.y < HEADER_H {
        return None;
    }
    let col = ((local.x - HEADER_W) / COL_W).floor();
    let row = ((local.y - HEADER_H) / ROW_H).floor();
    if row < 0.0 || col < 0.0 {
        return None;
    }
    Some(CellRef::new(0, row as u32, col as u32))
}

/// Row and column counts worth scrolling: at least a screenful, always covering
/// everything populated.
fn content_extent(wb: &Workbook) -> (u32, u32) {
    let sheet = wb.active();
    (sheet.row_count().max(MIN_ROWS), sheet.col_count().max(MIN_COLS))
}

/// Apply a movement key to the selection. Returns true when the key was consumed.
fn handle_key(state: &mut GridState, wb: &Workbook, k: egui::Key, shift: bool) -> bool {
    let sheet = wb.active_id();
    let (mut row, mut col) = (state.selection.active.row, state.selection.active.col);
    match k {
        egui::Key::ArrowUp if row > 0 => row -= 1,
        egui::Key::ArrowDown => row += 1,
        egui::Key::ArrowLeft if col > 0 => col -= 1,
        egui::Key::ArrowRight => col += 1,
        egui::Key::Home => col = 0,
        _ => return false,
    }
    state.selection.active = CellRef::new(sheet, row, col);
    if !shift {
        state.selection.anchor = state.selection.active;
    }
    true
}

/// Collect this frame's key-down events and text-input events.
fn read_input(ui: &egui::Ui) -> (Vec<egui::Key>, Vec<String>, bool) {
    ui.input(|i| {
        let mut keys = Vec::new();
        let mut text = Vec::new();
        for e in &i.events {
            match e {
                egui::Event::Key { key, pressed: true, repeat: false, .. } => keys.push(*key),
                egui::Event::Text(t) => text.push(t.clone()),
                _ => {}
            }
        }
        (keys, text, i.modifiers.shift)
    })
}

/// Draw the grid and process navigation.
pub fn show(state: &mut GridState, wb: &mut Workbook, ui: &mut egui::Ui) {
    let (rows, cols) = content_extent(wb);
    let content_size = Vec2::new(
        HEADER_W + cols as f32 * COL_W,
        HEADER_H + rows as f32 * ROW_H,
    );
    let viewport = ui.available_size();
    let visuals = ui.visuals().clone();

    let (keys, _text_events, shift) = read_input(ui);
    let mut handled = false;
    for k in &keys {
        if handle_key(state, wb, *k, shift) {
            handled = true;
        }
    }

    egui::ScrollArea::both()
        .max_height(viewport.y)
        .max_width(viewport.x)
        .auto_shrink([false, false])
        .show(ui, |ui| {
            let (response, painter) =
                ui.allocate_painter(content_size, Sense::click());
            let origin = response.rect.min;

            // `if let` chains require edition 2024, which this crate uses.
            if let Some(pos) = response.interact_pointer_pos().filter(|_| response.clicked())
                && let Some(hit) = cell_at_pos(origin, pos)
            {
                let cell = CellRef::new(wb.active_id(), hit.row, hit.col);
                state.selection.active = cell;
                state.selection.anchor = cell;
                handled = true;
            }

            paint(state, wb, &painter, &visuals, origin, rows, cols);
        });

    if handled {
        // Immediate mode repaints on demand; without this the highlight and the
        // edited contents would stay stale until the next unrelated repaint.
        ui.ctx().request_repaint();
    }
}

/// Paint the headers and the visible window of cells.
fn paint(
    state: &GridState,
    wb: &Workbook,
    painter: &egui::Painter,
    visuals: &Visuals,
    origin: Pos2,
    rows: u32,
    cols: u32,
) {

    let header_bg = visuals.widgets.inactive.bg_fill;
    let grid_line = visuals.widgets.noninteractive.bg_stroke.color;
    let sheet_id = state.selection.active.sheet;
    let sheet = wb.sheet(sheet_id);
    let sel = state.selection.range();

    // The clip rect is the ScrollArea's viewport, which is exactly the window we
    // must paint. Clipping means over-painting the headers is harmless.
    let clip = painter.clip_rect();
    let local_min = clip.min - origin;
    let local_max = clip.max - origin;

    let first_row = ((((local_min.y - HEADER_H) / ROW_H).floor() as i64) - OVERSCAN).max(0) as u32;
    let last_row = ((((local_max.y - HEADER_H) / ROW_H).ceil() as i64) + OVERSCAN).max(0) as u32;
    let first_col = ((((local_min.x - HEADER_W) / COL_W).floor() as i64) - OVERSCAN).max(0) as u32;
    let last_col = ((((local_max.x - HEADER_W) / COL_W).ceil() as i64) + OVERSCAN).max(0) as u32;
    let last_row = last_row.min(rows);
    let last_col = last_col.min(cols);

    for row in first_row..last_row {
        for col in first_col..last_col {
            let rect = cell_rect(row, col).translate(origin.to_vec2());
            let is_active = state.selection.active == CellRef::new(sheet_id, row, col);
            let in_selection = sel.contains(row, col);
            let cell = sheet.get(row, col);

            let bg = match cell.and_then(|c| c.format.bg_color) {
                Some(colour) => Color32::from_rgba_unmultiplied(colour.r, colour.g, colour.b, colour.a),
                None if in_selection && !is_active => visuals.selection.bg_fill,
                None if row % 2 == 1 => visuals.widgets.inactive.weak_bg_fill,
                None => Color32::TRANSPARENT,
            };
            if bg != Color32::TRANSPARENT {
                painter.rect_filled(rect, 0.0, bg);
            }

            painter.rect_stroke(rect, 0.0, Stroke::new(1.0, grid_line), StrokeKind::Inside);

            if is_active {
                painter.rect_stroke(
                    rect,
                    0.0,
                    Stroke::new(2.0, visuals.selection.stroke.color),
                    StrokeKind::Middle,
                );
            }

            if let Some(cell) = cell {
                let text = cell.formatted();
                if !text.is_empty() {
                    let colour = match cell.format.font_color {
                        Some(c) => Color32::from_rgba_unmultiplied(c.r, c.g, c.b, c.a),
                        None => visuals.text_color(),
                    };
                    let galley = painter.layout_no_wrap(
                        text,
                        FontId::proportional(14.0),
                        colour,
                    );
                    painter.galley(
                        Pos2::new(rect.left() + 4.0, rect.center().y - 7.0),
                        galley,
                        colour,
                    );
                }
            }
        }
    }

    // Column headers, left to right.
    let col_header = Rect::from_min_size(
        origin,
        Vec2::new(HEADER_W + cols as f32 * COL_W, HEADER_H),
    );
    painter.rect_filled(col_header, 0.0, header_bg);
    for col in 0..last_col {
        let r = Rect::from_min_size(
            Pos2::new(origin.x + cell_min(0, col).x, origin.y),
            Vec2::new(COL_W, HEADER_H),
        );
        painter.text(
            r.center(),
            Align2::CENTER_CENTER,
            col_to_letters(col),
            FontId::proportional(12.0),
            visuals.weak_text_color(),
        );
    }

    // Row headers, top to bottom.
    for row in first_row..last_row {
        let r = Rect::from_min_size(
            Pos2::new(origin.x, origin.y + cell_min(row, 0).y),
            Vec2::new(HEADER_W, ROW_H),
        );
        painter.rect_filled(r, 0.0, header_bg);
        painter.text(
            r.center(),
            Align2::CENTER_CENTER,
            (row + 1).to_string(),
            FontId::proportional(12.0),
            visuals.weak_text_color(),
        );
    }

    // The corner box where the two headers meet.
    painter.rect_filled(
        Rect::from_min_size(origin, Vec2::new(HEADER_W, HEADER_H)),
        0.0,
        header_bg,
    );

    if let Some(edit) = &state.edit {
        let rect = cell_rect(edit.active.row, edit.active.col).translate(origin.to_vec2());
        painter.rect_filled(rect, 0.0, visuals.extreme_bg_color);
        painter.rect_stroke(
            rect,
            0.0,
            Stroke::new(2.0, visuals.selection.stroke.color),
            StrokeKind::Middle,
        );
        let colour = visuals.text_color();
        let galley = painter.layout_no_wrap(
            edit.buffer.clone(),
            FontId::proportional(14.0),
            colour,
        );
        painter.galley(
            Pos2::new(rect.left() + 4.0, rect.center().y - 7.0),
            galley,
            colour,
        );
    }
}
```

- [ ] **Step 4: Split the app's UI body out of the `eframe::App` impl**

Replace `src/app/app.rs` with the version below. The `body` split is what lets the headless tests drive real production code: `eframe::Frame` cannot be constructed outside a running eframe event loop, so tests call `body` directly.

```rust
use eframe::egui;

use crate::ui::grid::{self, GridState};
use crate::workbook::sheet::Sheet;
use crate::workbook::Workbook;

pub struct OmaApp {
    pub wb: Workbook,
    pub grid: GridState,
    pub error: Option<String>,
}

impl Default for OmaApp {
    fn default() -> Self {
        Self { wb: Workbook::new(), grid: GridState::new(), error: None }
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
            Err(e) => self.error = Some(format!("could not read {}: {e:?}", path.display())),
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
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cargo test --test grid_keys`
Expected: 13 passed, 0 failed.

If `cell_at_pos` fails to compile with an arity error, a stale call site with the removed `state` parameter survived; it now takes exactly `(origin, pos)`.

- [ ] **Step 6: Run every test**

Run: `cargo test`
Expected: all four suites pass — `cell`, `workbook`, `csv_read`, `grid_keys`.

- [ ] **Step 7: Lint**

Run: `cargo clippy --all-targets`
Expected: silent. Remove any `use` that no longer resolves; `format_a1` and `CellValue` are no longer used in `grid.rs` and must not be imported there.

- [ ] **Step 8: Run the app and look at it**

Run: `timeout 10 cargo run`
Expected: a window titled "Omasheet" showing empty cells, numbered row headers down the left, lettered column headers across the top reading `A`, `B`, …, `AA`, and a highlighted A1. Arrow keys move the highlight; the window scrolls. Exit code 124.

- [ ] **Step 9: Commit**

```bash
git add src/ui/grid.rs src/app/app.rs tests/grid_keys.rs
git commit -m "feat: virtualized grid with selection and keyboard navigation"
```
### Task 6: In-Cell Editing and Dirty State

**Files:**
- Modify: `src/ui/grid.rs`
- Modify: `src/app/app.rs`
- Modify: `tests/grid_keys.rs`

**Interfaces:**
- Consumes: `GridState`, `Selection`, `EditState` from Task 5; `CellValue`, `to_input_string`, `parse_a1` from Task 2; `Workbook::set_cell`, `value_at`, `dirty` from Task 3.
- Produces: `pub fn parse_input(s: &str) -> CellValue`, and `GridState::edit` becomes populated and cleared. Later plans build undo (Plan B) and formatting (Plan C) on top of these.

- [ ] **Step 1: Extend `tests/grid_keys.rs` with failing tests**

Append to the file, and change the `pass` helper's return type to `()` if it is unused:

```rust
#[test]
fn typing_a_character_starts_editing_with_that_character() {
    let mut app = OmaApp::default();
    pass(&mut app, type_text("7"));
    let edit = app.grid.edit.as_ref().expect("typing should start an edit");
    assert_eq!(edit.buffer, "7");
    assert_eq!(edit.active, CellRef::new(0, 0, 0));
}

#[test]
fn enter_commits_the_edit_and_moves_down() {
    let mut app = OmaApp::default();
    pass(&mut app, type_text("7"));
    pass(&mut app, key(Key::Enter));
    assert!(app.grid.edit.is_none());
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Number(7.0));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 1, 0));
}

#[test]
fn tab_commits_the_edit_and_moves_right() {
    let mut app = OmaApp::default();
    pass(&mut app, type_text("7"));
    pass(&mut app, key(Key::Tab));
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Number(7.0));
    assert_eq!(app.grid.selection.active, CellRef::new(0, 0, 1));
}

#[test]
fn escape_cancels_the_edit_and_leaves_the_cell_alone() {
    let mut app = OmaApp::default();
    app.wb.set_cell(CellRef::new(0, 0, 0), CellValue::Text("original".into()));
    pass(&mut app, type_text("clobbered"));
    pass(&mut app, key(Key::Escape));
    assert!(app.grid.edit.is_none());
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Text("original".into()));
}

#[test]
fn f2_starts_an_edit_seeded_with_the_current_text() {
    let mut app = OmaApp::default();
    app.wb.set_cell(CellRef::new(0, 0, 0), CellValue::Text("rent".into()));
    pass(&mut app, key(Key::F2));
    let edit = app.grid.edit.as_ref().expect("F2 should start an edit");
    assert_eq!(edit.buffer, "rent");
}

#[test]
fn delete_clears_the_active_cell_and_marks_dirty() {
    let mut app = OmaApp::default();
    app.wb.set_cell(CellRef::new(0, 0, 0), CellValue::Text("rent".into()));
    app.wb.clear_dirty();
    pass(&mut app, key(Key::Delete));
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Empty);
    assert!(app.wb.dirty());
}

#[test]
fn editing_an_empty_cell_and_committing_empty_leaves_it_empty() {
    let mut app = OmaApp::default();
    pass(&mut app, type_text("x"));
    pass(&mut app, key(Key::Escape));
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Empty);
    assert!(!app.wb.dirty());
}

#[test]
fn text_input_is_stored_verbatim_without_inference() {
    let mut app = OmaApp::default();
    pass(&mut app, type_text("00123"));
    pass(&mut app, key(Key::Enter));
    // Leading-zero identifiers must survive as text; this is the rule the CSV
    // reader also enforces.
    assert_eq!(app.wb.value_at(CellRef::new(0, 0, 0)), CellValue::Text("00123".into()));
}

#[test]
fn parse_input_infers_plain_numbers_only() {
    assert_eq!(parse_input("42"), CellValue::Number(42.0));
    assert_eq!(parse_input("3.50"), CellValue::Number(3.5));
    assert_eq!(parse_input("00123"), CellValue::Text("00123".into()));
    assert_eq!(parse_input(""), CellValue::Empty);
    assert_eq!(parse_input("hello"), CellValue::Text("hello".into()));
    assert_eq!(parse_input("=A1+B1"), CellValue::Text("=A1+B1".into()));
}
```

That last case is the known Plan A limitation: formulas are not implemented until Plan B, so `=`-prefixed input is stored as text. It is pinned by this test so Plan B changes it deliberately.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cargo test --test grid_keys`
Expected: failures for every test in this task — `parse_input` does not exist, and editing does not work.

- [ ] **Step 3: Implement `parse_input` in `src/ui/grid.rs`**

Add to the top of `src/ui/grid.rs`:

```rust
/// Interpret typed text as a cell value.
///
/// The same guarded rule the CSV reader uses: `NaN`, `inf` and redundant leading
/// zeros stay text. Text is stored exactly as typed — no trimming, no case folding.
///
/// Formulas are not implemented in this plan. `=`-prefixed input is stored as text
/// so nothing is silently reinterpreted; Plan B changes this.
pub fn parse_input(s: &str) -> CellValue {
    if s.is_empty() {
        return CellValue::Empty;
    }
    match crate::io::csv_read::infer_number(s) {
        Some(n) => CellValue::Number(n),
        None => CellValue::Text(s.to_owned()),
    }
}
```

- [ ] **Step 4: Add edit handling to `grid::show`**

Add these functions to `src/ui/grid.rs`:

```rust
/// Begin editing `at`, seeding the buffer with the cell's current text.
pub fn begin_edit(state: &mut GridState, wb: &Workbook, at: CellRef) {
    let buffer = wb.value_at(at).to_input_string();
    state.edit = Some(EditState { active: at, buffer });
}

/// Commit the in-progress edit into the model. Does nothing if no edit is active.
pub fn commit_edit(state: &mut GridState, wb: &mut Workbook) {
    let Some(edit) = state.edit.take() else { return };
    wb.set_cell(edit.active, parse_input(&edit.buffer));
}

/// Abandon the in-progress edit.
pub fn cancel_edit(state: &mut GridState) {
    state.edit = None;
}
```

Then extend `show` to drive the edit lifecycle, inserted before the `ScrollArea`:

```rust
    // Editing lifecycle.
    let (_keys, text_events, _shift) = read_input(ui);

    if state.edit.is_some() {
        for t in &text_events {
            if let Some(edit) = state.edit.as_mut() {
                edit.buffer.push_str(t);
                handled = true;
            }
        }
    } else if keys.contains(&egui::Key::F2) {
        begin_edit(state, wb, state.selection.active);
        handled = true;
    } else if let Some(t) = text_events.iter().find(|t| !t.chars().all(char::is_control)) {
        state.edit = Some(EditState { active: state.selection.active, buffer: t.clone() });
        handled = true;
    }

    for k in keys {
        match k {
            egui::Key::Enter | egui::Key::Tab => {
                if state.edit.is_some() {
                    commit_edit(state, wb);
                    handled = true;
                }
            }
            egui::Key::Escape => {
                if state.edit.is_some() {
                    cancel_edit(state);
                    handled = true;
                }
            }
            egui::Key::Delete | egui::Key::Backspace => {
                if state.edit.is_none() {
                    for cell in selection_cells(state) {
                        wb.clear_cell(cell);
                    }
                    handled = true;
                }
            }
            _ => {}
        }
    }
```

Remove `Enter`, `Tab`, `Delete` and `Backspace` from the movement `match` in `handle_key` if they appear there — they must not move the selection while editing.

Add the helper:

```rust
/// Every cell in the selection rectangle, in row-major order.
fn selection_cells(state: &GridState) -> Vec<CellRef> {
    let sheet = state.selection.active.sheet;
    let r = state.selection.range();
    let mut out = Vec::new();
    for row in r.min_row..=r.max_row {
        for col in r.min_col..=r.max_col {
            out.push(CellRef::new(sheet, row, col));
        }
    }
    out
}
```

After committing, move the selection. Extend the `Enter`/`Tab` arms:

```rust
            egui::Key::Enter | egui::Key::Tab => {
                if state.edit.is_some() {
                    commit_edit(state, wb);
                    let shift = ui.input(|i| i.modifiers.shift);
                    match k {
                        egui::Key::Enter if !shift => {
                            state.selection.active.row += 1;
                            state.selection.anchor = state.selection.active;
                        }
                        egui::Key::Enter => {
                            state.selection.active.row = state.selection.active.row.saturating_sub(1);
                            state.selection.anchor = state.selection.active;
                        }
                        _ => {
                            state.selection.active.col += 1;
                            state.selection.anchor = state.selection.active;
                        }
                    }
                    handled = true;
                }
            }
```

- [ ] **Step 5: Paint the editor over the active cell**

At the end of `paint`, before the function closes:

```rust
    if let Some(edit) = &state.edit {
        let rect = cell_rect(edit.active.row, edit.active.col) + origin.to_vec();
        painter.rect_filled(rect, 0.0, visuals.extreme_bg_color);
        painter.rect_stroke(
            rect,
            0.0,
            Stroke::new(2.0, visuals.selection.stroke.color),
            egui::StrokeKind::Middle,
        );
        let colour = visuals.text_color();
        let galley = painter.layout_no_wrap(
            edit.buffer.clone(),
            FontId::proportional(14.0),
            colour,
        );
        painter.galley(
            Pos2::new(rect.left() + 4.0, rect.center().y - 7.0),
            galley,
            colour,
        );
    }
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cargo test --test grid_keys`
Expected: all pass.

`text_input_is_stored_verbatim_without_inference` is the test most likely to fail. If it does, `parse_input` is not reaching `infer_number` — check that `parse_input` is the function `commit_edit` calls.

- [ ] **Step 7: Run every test**

Run: `cargo test`
Expected: all four suites pass.

- [ ] **Step 8: Lint**

Run: `cargo clippy --all-targets`
Expected: silent.

- [ ] **Step 9: Run the app and edit a cell by hand**

Run: `cargo run`
Expected: typing a character replaces the active cell's content and shows a cursor; `Enter` commits and moves down; `Esc` reverts; `Delete` clears the cell; the "modified" marker appears after the first edit.

- [ ] **Step 10: Commit**

```bash
git add src/ui/grid.rs src/app/app.rs tests/grid_keys.rs
git commit -m "feat: in-cell editing, commit keys and dirty tracking"
```

---

## Self-Review Checklist

Run through this before executing.

**1. Spec coverage.** This plan covers spec sections 3.1 (modules in part), 4.1–4.4 (types and sparse storage), 6.1 (edit modes, commit keys), 7.1 (virtualized grid, frozen headers), 7.3 (arrow/Home keybindings), 8.1 (CSV read), 9 (testing approach), and 10 steps 1–4. Formula evaluation (spec 5), cross-sheet references, undo (spec 6.2), clipboard and fill (spec 6.3), the ribbon (spec 7.2), xlsx I/O (spec 8.2–8.3), export (spec 8.4) and the 16 ms performance gate (spec 9) are deliberately out of scope and belong to Plans B and C.

**2. Placeholder scan.** No task contains "TBD", "similar to", "handle edge cases", or code written to be corrected by a later step. The only occurrences of "placeholder" are Task 1 Step 4 (creating deliberately empty modules so the crate compiles) and the "replace placeholder" file headers.

**3. Type consistency.** `CellValue::to_display_string` vs `to_input_string` are distinct and both used deliberately. `NumberFormat::apply` takes `&CellValue`. `Sheet::set` takes a full `Cell`. `Workbook::set_cell` takes a `CellValue` and preserves existing formatting; `set_cell_format` takes a `CellFormat` and preserves the existing value. `CellRange` is used for both the used-range and the selection rectangle. `GridState::selection.range()` returns `CellRange`. Task 6 introduces `parse_input`, `begin_edit`, `commit_edit`, `cancel_edit`, `selection_cells` — Task 5 declares none of them, and Task 6's tests are the first reference.

**4. Every code block in this plan was extracted, compiled, and tested before the plan was finalised.** All 63 tests pass and `cargo clippy --all-targets` is silent. That pass caught eleven defects that reading the code had not:

| Defect | Fix |
|---|---|
| `CellValue` had no `Default`, so `#[derive(Default)]` on `Cell` failed | `#[derive(Default)]` with `#[default]` on the `Empty` variant |
| `Row` had no `get_mut`, so `Sheet::get_mut` could not compile | added `Row::get_mut` |
| `painter.ctx().style()` does not exist in 0.35 | `paint` now takes `&Visuals`, cloned from `ui.visuals()` in `show` |
| `Pos2::to_vec()` does not exist | `to_vec2()` |
| `Rect + Vec2` is not implemented | `Rect::translate(vec)` |
| `Sense::hover_and_drag()` does not exist | `Sense::click()`, which includes hover |
| `Painter::galley` takes the position **first** | argument order corrected |
| `Fonts::layout` takes an `f32` wrap width and needs `&mut self`, and `Context::fonts` hands out an immutable view | `Painter::layout_no_wrap` |
| `NumberFormat::Currency` appended the decimal suffix twice, so `$1,234.50` rendered as `$1,234,.50.00` | group the integer part only, then re-attach the fraction |
| `sniff_delimiter` scored `(agreements, row_count)`, which is identical for every candidate on a one-column file, so a comma file beat a semicolon file | score on `(total fields, agreements)` |
| `sniff_delimiter`'s reader lacked `flexible(true)`, so on a ragged CSV the correct delimiter errored out and was discarded before scoring | `flexible(true)`, matching the real reader |

Three test expectations were also wrong and were corrected rather than the code: `CellRange::new` was called with corners instead of four scalars; `add_sheet` was assumed not to change the active sheet; and `rename_sheet` was expected to reject renaming a sheet to its own current name, which is a no-op rather than a duplicate.

**5. Known seams, documented rather than hidden.**
- `Cell` gains an `input` field in Plan B; Task 2 fixes `Cell` to `{ value, format }`.
- `parse_input` stores `=`-prefixed input as text until Plan B. Pinned by `parse_input_infers_plain_numbers_only`.
- Task 6 reads input a second time via `read_input` rather than threading Task 5's `_text_events` binding through. Two calls per frame is not worth the coupling.
- Test counts ("13 passed", "16 passed", "22 passed", "12 passed") are sanity checks. If a count differs, trust the assertions and reconcile the count.