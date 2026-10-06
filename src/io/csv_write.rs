//! CSV writing.
//!
//! The inverse of [`crate::io::csv_read`], and separated from it for the same reason
//! the reader is a pure function: nothing here touches the filesystem except
//! [`write_csv_file`], so the whole of the quoting and value rules can be exercised
//! from a plain test binary.
//!
//! ## Quoting
//!
//! A field is quoted only when it has to be: it holds the delimiter, a quote, a
//! carriage return or a newline, or it begins or ends with whitespace. Anything
//! else is written bare, because a file where every field is quoted is one that
//! Excel and LibreOffice both read and one that no human can read at all. The quote
//! inside a quoted field is doubled, as RFC 4180 says, and that is the only escape
//! this module performs.
//!
//! ## The value written is the value held
//!
//! [`crate::workbook::cell::CellValue::to_storage_string`], not the cell's formatted
//! display and not the formula bar's input form. A `NumberFormat` that groups
//! thousands would emit `1,234`, which `infer_number` rejects — so a writer that
//! formatted its numbers would silently turn every one of them into text, on every
//! save, with nothing on screen to show for it.
//!
//! ## Why the replace is atomic
//!
//! [`write_csv_file`] writes a temporary beside the target, flushes it, and renames
//! it over the target. A save that fails halfway then leaves the file that was
//! there byte for byte, rather than the first half of the new one — and the dirty
//! flag stays set, so the work is still visibly unsaved. The flush is not ceremony:
//! a rename is atomic with respect to readers, but without the data on disk a power
//! loss can leave the renamed name pointing at nothing. The trade is that the target
//! becomes a new inode, so a symlink at the save path is replaced by a real file
//! rather than written through. That is accepted rather than worked around.

use std::path::Path;

use crate::workbook::sheet::Sheet;

/// How a save failed.
///
/// One variant, because there is one way to fail: the filesystem. The reader's
/// `CsvError` has three because a file can be empty, ill-encoded or absent, and none
/// of those is reachable on the way out.
#[derive(Debug, PartialEq, Eq)]
pub enum WriteError {
    /// The file could not be created, written, flushed or renamed into place.
    Io,
}

impl std::fmt::Display for WriteError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            WriteError::Io => f.write_str("the file could not be written"),
        }
    }
}

impl std::error::Error for WriteError {}

/// The delimiter a path's extension asks for.
///
/// The extension, not the sniffer: Save As is the user naming the file, and a file
/// called `budget.csv` is a comma file whatever its cells happen to contain. Sniffing
/// here would rewrite a `.csv` whose contents sniff as tab-delimited into a tab file
/// under the same name. Anything that is not `.tsv`/`.tab` is a comma, which is also
/// what an untitled document starts with.
pub fn delimiter_for_extension(path: &Path) -> u8 {
    match path
        .extension()
        .and_then(|e| e.to_str())
        .map(str::to_ascii_lowercase)
        .as_deref()
    {
        Some("tsv" | "tab") => b'\t',
        _ => b',',
    }
}

/// Whether a field has to be written inside quotes.
///
/// Bytes rather than `char`s for the delimiters, because that is the unit the `csv`
/// crate splits records in and the unit a `\r`/`\n` is found by. The whitespace test
/// is on `char`s because a `str`'s edges are only meaningful there, and a non-ASCII
/// space is still a space.
fn needs_quotes(field: &str, delimiter: u8) -> bool {
    let special = field
        .bytes()
        .any(|b| b == delimiter || b == b'"' || b == b'\r' || b == b'\n');
    let padded = field.starts_with(char::is_whitespace) || field.ends_with(char::is_whitespace);
    special || padded
}

/// Append one field, quoted if it has to be.
fn push_field(out: &mut String, field: &str, delimiter: u8) {
    if !needs_quotes(field, delimiter) {
        out.push_str(field);
        return;
    }
    out.push('"');
    for ch in field.chars() {
        if ch == '"' {
            out.push('"');
        }
        out.push(ch);
    }
    out.push('"');
}

/// A sheet as CSV text.
///
/// Every row of the used range is written at the used range's full width, so a row
/// that is short in memory comes out rectangular. That is the shape a spreadsheet
/// shows and the shape [`crate::io::csv_read::read_csv`] leaves short rows in, and
/// the missing cells read back as empty — which is what they already were.
///
/// `\n` ends every line, the last one included. The reader accepts `\r\n` too, but
/// this app targets Unix and a `\r` in the output would have to be quoted on the way
/// out for a Windows tool to read the line ending as an ending rather than as data.
pub fn write_csv(sheet: &Sheet, delimiter: u8) -> String {
    let Some(used) = sheet.used() else {
        return String::new();
    };

    let mut out = String::new();
    for row in used.min_row..=used.max_row {
        for col in used.min_col..=used.max_col {
            if col > used.min_col {
                out.push(delimiter as char);
            }
            let field = sheet
                .get(row, col)
                .map(|cell| cell.value.to_storage_string())
                .unwrap_or_default();
            push_field(&mut out, &field, delimiter);
        }
        out.push('\n');
    }
    out
}

/// Write a sheet to `path`, replacing it only once the bytes are on disk.
///
/// The temporary is created in the target's own directory, which is what makes the
/// rename a same-filesystem one: `rename` across a filesystem boundary fails, and
/// `EXDEV` is the one error a "write a temporary in `/tmp`" version of this would
/// hand back on every save. The name carries the process id because two saves in one
/// process, or two instances of the app, must not share a temporary.
///
/// Every failure past the point the temporary exists removes it, so a failed save
/// leaves the directory exactly as it found it. The temporary file handle is closed
/// before the rename, which Windows requires and which makes the flush meaningful on
/// the platforms that care.
pub fn write_csv_file(path: &Path, sheet: &Sheet, delimiter: u8) -> Result<(), WriteError> {
    let text = write_csv(sheet, delimiter);

    // A relative name has a parent of `""`, which names no directory, and a bare file
    // name has no parent at all. Both mean "the current directory", which is where a
    // temporary has to be for the rename to stay on one filesystem.
    let dir = match path.parent() {
        Some(parent) if !parent.as_os_str().is_empty() => parent.to_path_buf(),
        _ => Path::new(".").to_path_buf(),
    };
    let name = path.file_name().ok_or(WriteError::Io)?;
    let temp_name = format!(
        ".{}.omasheet-{}.tmp",
        name.to_string_lossy(),
        std::process::id()
    );
    let temp = dir.join(temp_name);

    let written = (|| -> std::io::Result<()> {
        let mut file = std::fs::File::create(&temp)?;
        std::io::Write::write_all(&mut file, text.as_bytes())?;
        // Flush before the rename, not after: after the rename the data is already
        // visible under the target's name, and a power loss between the two would
        // leave the new name pointing at an empty file.
        file.sync_all()?;
        Ok(())
    })();
    if let Err(e) = written {
        let _ = std::fs::remove_file(&temp);
        return Err(e.into());
    }

    if let Err(e) = std::fs::rename(&temp, path) {
        let _ = std::fs::remove_file(&temp);
        return Err(e.into());
    }
    Ok(())
}

impl From<std::io::Error> for WriteError {
    fn from(_: std::io::Error) -> Self {
        WriteError::Io
    }
}
