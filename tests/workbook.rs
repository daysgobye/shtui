use omasheet::workbook::Workbook;
use omasheet::workbook::cell::*;
use omasheet::workbook::sheet::Sheet;

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
        number_format: NumberFormat::Currency {
            symbol: '$',
            places: 2,
        },
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
    assert!(
        !wb.rename_sheet(0, "Two"),
        "a name already in use must be refused"
    );
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

#[test]
fn clearing_an_interior_cell_leaves_the_used_range_untouched() {
    // Removing an interior cell cannot change the bounding box, so `remove` must
    // return without rebuilding it. Both `rows_iterate_in_column_order_and_skip_holes`
    // and this test would break if the fast path were dropped.
    let mut wb = Workbook::new();
    wb.set_cell(CellRef::new(0, 0, 0), CellValue::Number(1.0));
    wb.set_cell(CellRef::new(0, 5, 5), CellValue::Number(2.0));
    wb.set_cell(CellRef::new(0, 9, 9), CellValue::Number(3.0));
    wb.clear_cell(CellRef::new(0, 5, 5));
    let used = wb.active().used().expect("sheet is not empty");
    assert_eq!(used.min_row, 0);
    assert_eq!(used.min_col, 0);
    assert_eq!(used.max_row, 9);
    assert_eq!(used.max_col, 9);
}

#[test]
fn clearing_a_cell_that_is_absent_does_not_mark_the_workbook_dirty() {
    // Pressing Delete on an empty cell must not make the app claim unsaved changes.
    let mut wb = Workbook::new();
    wb.clear_dirty();
    wb.clear_cell(CellRef::new(0, 3, 3));
    assert!(!wb.dirty());
}

#[test]
fn value_at_reads_a_missing_cell_as_empty() {
    // The contract the grid and the later formula engine both rely on.
    let mut wb = Workbook::new();
    assert_eq!(wb.value_at(CellRef::new(0, 7, 7)), CellValue::Empty);
    wb.set_cell(CellRef::new(0, 7, 7), CellValue::Number(5.0));
    assert_eq!(wb.value_at(CellRef::new(0, 7, 7)), CellValue::Number(5.0));
}

#[test]
fn iter_cells_visits_every_cell_in_row_major_order() {
    // The contract save and CSV export depend on.
    let mut wb = Workbook::new();
    wb.set_cell(CellRef::new(0, 2, 1), CellValue::Text("b".into()));
    wb.set_cell(CellRef::new(0, 0, 3), CellValue::Text("a".into()));
    wb.set_cell(CellRef::new(0, 1, 0), CellValue::Text("c".into()));
    let coords: Vec<(u32, u32)> = wb.active().iter_cells().map(|(r, c, _)| (r, c)).collect();
    assert_eq!(coords, vec![(0, 3), (1, 0), (2, 1)]);
}

#[test]
fn replace_sheet_stamps_the_id_so_it_matches_its_index() {
    let mut wb = Workbook::new();
    wb.add_sheet("Two");
    // A reader may hand back a sheet built with the wrong id; the workbook must not
    // end up holding a sheet whose `id()` disagrees with its position.
    let sheet = Sheet::new(99, "Loaded");
    wb.replace_sheet(1, sheet);
    assert_eq!(wb.sheet(1).id(), 1);
}
