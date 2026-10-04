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
fn cell_range_new_normalises_unsorted_bounds() {
    // `new` is total: unsorted corners are ordered rather than underflowing `rows()`.
    let r = CellRange::new(5, 4, 1, 2);
    assert_eq!(r.min_row, 1);
    assert_eq!(r.min_col, 2);
    assert_eq!(r.max_row, 5);
    assert_eq!(r.max_col, 4);
    assert_eq!(r.rows(), 5);
    assert_eq!(r.cols(), 3);
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
    assert_eq!(
        NumberFormat::Decimal(2).apply(&CellValue::Number(1.23456)),
        "1.23"
    );
    assert_eq!(
        NumberFormat::Currency {
            symbol: '$',
            places: 2
        }
        .apply(&CellValue::Number(1234.5)),
        "$1,234.50"
    );
    assert_eq!(
        NumberFormat::Percent(1).apply(&CellValue::Number(0.456)),
        "45.6%"
    );
    assert_eq!(NumberFormat::Text.apply(&CellValue::Number(3.5)), "3.5");
}

#[test]
fn number_format_passes_text_and_errors_through() {
    assert_eq!(
        NumberFormat::Decimal(2).apply(&CellValue::Text("hi".into())),
        "hi"
    );
    assert_eq!(
        NumberFormat::General.apply(&CellValue::Error(ErrorKind::DivZero)),
        "#DIV/0!"
    );
    assert_eq!(NumberFormat::General.apply(&CellValue::Empty), "");
}

#[test]
fn thousands_separators_group_correctly() {
    let f = NumberFormat::Currency {
        symbol: '$',
        places: 2,
    };
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
    assert_eq!(
        CellValue::Error(ErrorKind::DivZero).to_display_string(),
        "#DIV/0!"
    );
    assert_eq!(
        CellValue::Error(ErrorKind::Value).to_display_string(),
        "#VALUE!"
    );
    assert_eq!(
        CellValue::Error(ErrorKind::Name).to_display_string(),
        "#NAME?"
    );
    assert_eq!(
        CellValue::Error(ErrorKind::Ref).to_display_string(),
        "#REF!"
    );
    assert_eq!(
        CellValue::Error(ErrorKind::Circular).to_display_string(),
        "#CIRCULAR!"
    );
    assert_eq!(
        CellValue::Error(ErrorKind::Num).to_display_string(),
        "#NUM!"
    );
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
            number_format: NumberFormat::Currency {
                symbol: '$',
                places: 2,
            },
            ..Default::default()
        },
    };
    assert_eq!(c.formatted(), "$1,234.50");
}
