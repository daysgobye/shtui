use omasheet::io::csv_read::*;
use omasheet::workbook::Workbook;
use omasheet::workbook::cell::{CellRef, CellValue};

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
    assert_eq!(
        sheet.get(0, 0).unwrap().value,
        CellValue::Text("Name".into())
    );
    assert_eq!(sheet.row_count(), 2);
}

#[test]
fn quoted_fields_may_contain_the_delimiter_and_newlines() {
    let sheet = read_csv("\"a,b\",\"c\"\"d\"\n\"multi\nline\",2\n").unwrap();
    assert_eq!(
        sheet.get(0, 0).unwrap().value,
        CellValue::Text("a,b".into())
    );
    assert_eq!(
        sheet.get(0, 1).unwrap().value,
        CellValue::Text("c\"d".into())
    );
    assert_eq!(
        sheet.get(1, 0).unwrap().value,
        CellValue::Text("multi\nline".into())
    );
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
    assert_eq!(
        sheet.get(0, 0).unwrap().value,
        CellValue::Text("hello".into())
    );
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

#[test]
fn a_single_record_file_is_still_sniffed() {
    // A one-row export must not silently parse as one comma-delimited cell.
    assert_eq!(sniff_delimiter("a\tb\tc\n"), b'\t');
    assert_eq!(sniff_delimiter("a;b;c\n"), b';');
    assert_eq!(sniff_delimiter("a|b|c\n"), b'|');
}

#[test]
fn a_single_row_tab_file_loads_as_three_columns() {
    let sheet = read_csv("a\tb\tc\n").unwrap();
    assert_eq!(sheet.col_count(), 3);
    assert_eq!(sheet.get(0, 2).unwrap().value, CellValue::Text("c".into()));
}

#[test]
fn reading_a_missing_file_is_an_io_error() {
    let path = std::env::temp_dir().join("omasheet-no-such-file-4c1f.csv");
    let _ = std::fs::remove_file(&path);
    assert!(matches!(read_csv_file(&path), Err(CsvError::Io)));
}

#[test]
fn reading_a_file_that_is_not_utf8_is_an_encoding_error() {
    let path =
        std::env::temp_dir().join(format!("omasheet-invalid-utf8-{}.csv", std::process::id()));
    std::fs::write(&path, [0xffu8, 0xfe, b'A']).expect("temp fixture should be writable");
    let result = read_csv_file(&path);
    let _ = std::fs::remove_file(&path);
    assert!(matches!(result, Err(CsvError::Utf8)));
}

#[test]
fn csv_errors_render_as_a_sentence_fragment() {
    assert_eq!(CsvError::Empty.to_string(), "the file is empty");
    assert_eq!(
        CsvError::Utf8.to_string(),
        "the file is not valid UTF-8 text"
    );
    assert_eq!(CsvError::Io.to_string(), "the file could not be read");
}

/// The reader and the grid editor share one conversion, so the same field means the same
/// thing however it entered the sheet. This is the pin for that sharing: were the CSV
/// reader to grow its own copy of the rule, the tricky cases would drift apart here first.
#[test]
fn the_reader_and_the_editor_agree_on_every_field() {
    let fields = [
        "42", "3.50", "-7", "00123", "abc", " 42", "1,234", "NaN", "00.5", "0e1",
    ];
    // Quoted, because `"1,234"` would otherwise be read as two fields — and quoting also
    // confirms the value reaching the conversion is the raw field, not a split fragment.
    let header = fields
        .iter()
        .map(|f| format!("\"{f}\""))
        .collect::<Vec<_>>()
        .join(",");
    let sheet = read_csv(&header).expect("single-row fixture should parse");
    for (col, field) in fields.iter().enumerate() {
        assert_eq!(
            sheet.get(0, col as u32).map(|c| c.value.clone()),
            Some(CellValue::from_text(field)),
            "reader and editor disagreed about {:?}",
            field
        );
    }
}
