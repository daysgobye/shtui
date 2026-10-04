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
    Currency {
        symbol: char,
        places: u32,
    },
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

/// An inclusive rectangle of cells, always normalised so that `min_*` is the
/// top-left corner and `max_*` the bottom-right.
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

    /// Build from explicit bounds, normalising them.
    ///
    /// Delegates to [`CellRange::from_corners`] so `new` is total. Storing the four
    /// values verbatim would let unsorted corners produce a range whose `rows()`
    /// underflows — a panic in debug, a silent wrap to ~4.29 billion in release.
    /// Later tasks build ranges from UI drag state and from parsed references, so
    /// this constructor must not depend on its callers having sorted the corners.
    pub fn new(min_row: u32, min_col: u32, max_row: u32, max_col: u32) -> Self {
        Self::from_corners((min_row, min_col), (max_row, max_col))
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
