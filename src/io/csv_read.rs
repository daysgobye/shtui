//! CSV reading.
//!
//! The `csv` crate handles quoting, escaping and delimiter mechanics. Omasheet owns
//! delimiter sniffing, because it needs the raw field text that the crate hands back
//! verbatim. Number inference is *not* owned here: it lives with the value type in
//! [`CellValue::from_text`], so the editor and this reader cannot disagree about what a
//! field means.

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

impl std::fmt::Display for CsvError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let msg = match self {
            CsvError::Empty => "the file is empty",
            CsvError::Utf8 => "the file is not valid UTF-8 text",
            CsvError::Io => "the file could not be read",
        };
        f.write_str(msg)
    }
}

impl std::error::Error for CsvError {}

/// Guess the delimiter by parsing the first [`SNIFF_ROWS`] rows with each candidate
/// and scoring each on `(total fields, agreements)`.
///
/// Total fields is the primary signal: a delimiter that does not appear splits every
/// line into one field, so the real delimiter yields the most fields overall.
/// Agreement — how often the records match the first record's width — is only a
/// tie-break, because genuinely ragged files have inconsistent widths and must not
/// lose to a wrong delimiter.
///
/// Ties fall back to [`CANDIDATES`] order, so a comma wins an exact tie *among candidates
/// that scored at all*. A candidate whose records all error is skipped before it can be
/// scored, and then the earliest surviving candidate wins regardless of delimiter — the
/// default `b','` return is a starting guess, not a fallback that always beats the field
/// count.
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
        // A single record is still scoreable: its field count is a meaningful signal
        // for a one-row export, and skipping it would make every one-record file look
        // like a comma-delimited single cell. Every candidate whose records error, by
        // contrast, is skipped here — which is why the tie-break above is stated in terms
        // of surviving candidates rather than `CANDIDATES` order.
        if counts.is_empty() {
            continue;
        }

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

/// Parse CSV text into a sheet.
///
/// Every field goes through [`CellValue::from_text`], the same conversion the grid editor
/// uses, so a value means the same thing however it entered the sheet. The one visible
/// consequence of sharing it is that an empty CSV field becomes an `Empty`-valued *cell*
/// rather than an absent one: this reader treats "the record had a field here" as
/// populated, which is a defensible reading and is pinned by the CSV tests.
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
        // The reader is built over a `&[u8]`, so the only error kind the `csv` crate
        // can raise here is invalid UTF-8. Any other variant would mean the reader was
        // swapped for a file-backed one, at which point this mapping must be revisited.
        let record = record.map_err(|_| CsvError::Utf8)?;
        for (col, field) in record.iter().enumerate() {
            sheet.set(
                row,
                col as u32,
                Cell {
                    value: CellValue::from_text(field),
                    format: Default::default(),
                },
            );
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
