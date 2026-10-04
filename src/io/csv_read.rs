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
            cells.push(Cell {
                value,
                format: Default::default(),
            });
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
