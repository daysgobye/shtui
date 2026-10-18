# OmaSheet usage

OmaSheet is a compiled Go terminal spreadsheet for CSV and XLSX files. It uses a virtualized grid and targets workbooks up to roughly 25,000 rows by 20 columns per sheet; larger files may work but are not a performance guarantee.

## Build and launch

```sh
go build -o omasheet ./cmd/omasheet
./omasheet [workbook.csv|workbook.xlsx]
```

With no path, choose **Open file**, **New CSV workbook**, or **New XLSX workbook** from the home screen or command palette. Loading and saving run asynchronously. A failed open keeps the current workbook; a failed save keeps it dirty and reports the cause. Save to a new path with **Save as**.

## Grid and editing

- Arrow keys move the active cell. Press Enter or type to edit; Enter commits, Esc cancels.
- Drag with the left mouse button to select a rectangular range. Drag a column-header boundary to resize that column. The mouse wheel scrolls.
- Click a sheet tab to activate it. **Insert row** and **Insert column** insert before the selected cell.
- Ctrl+Z / Ctrl+Y undo and redo. **Find** searches the active sheet for a case-insensitive substring.
- The formula bar shows the selected cell's source input. Formulas are entered with `=`.

The initial formula scope is arithmetic, cell references/ranges, and common functions `SUM`, `AVERAGE`, `COUNT`, `MIN`, `MAX`, and `IF`. Excelize performs calculation; formulas it cannot recalculate are retained as source and marked stale or unsupported rather than presenting an old cached result as current. This is not full Excel formula compatibility.

## Formatting and data operations

Ctrl+P opens the searchable command palette. It remains available when the toolbar is hidden. Use **Toggle toolbar** to hide/show the toolbar.

- **Fill color** and **Font color** open terminal theme colors first, followed by custom RGB input (`#RRGGBB`). Theme colors are resolved only when selected. The app uses truecolor when supported and nearest ANSI colors otherwise.
- **Toggle bold** switches bold for the selected cell or range. **Add sheet** creates and activates a new sheet; click tabs to switch sheets.
- **Apply bottom border** adds a thin bottom border to the selected range.
- **Merge selected cells** merges the selection. If non-top-left cells contain data, the app asks before discarding those values. **Unmerge selected cells** removes the merge.
- **Add conditional formatting** applies one rule to the selected range. Enter `>number`, `<number`, `=text`, `blank`, or `contains:text`, then choose a theme or custom fill color.
- **Sort by selected column** sorts the contiguous data block containing the active row, moving whole rows together. Choose `a` for ascending with a header, `d` for descending with a header, or `h` for ascending without a header.
- **Filter selected column** accepts allowed values separated by semicolons. Press Enter with no values to clear filters. Filters hide rows without deleting data.

## File format behavior

CSV contains one sheet and has no native styles, merges, formulas, or workbook metadata. Formula source is saved as CSV text; formatting and other-sheet content cannot be represented. OmaSheet warns before a CSV save that would discard styles or extra sheets.

XLSX retains workbook content through Excelize, including content not directly editable in the terminal UI when the library can round-trip it. OmaSheet does not execute macros and does not implement all Excel features. Formula calculation and conditional-format evaluation are limited to the supported subset. Keep backups of workbooks that depend on unsupported Excel features; do not assume every Excel-specific feature is editable or recalculated by OmaSheet.
