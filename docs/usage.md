# OmaSheet usage

OmaSheet is a compiled Go terminal spreadsheet for CSV and XLSX files. It uses a virtualized grid and targets workbooks up to roughly 25,000 rows by 20 columns per sheet; larger files may work but are not a performance guarantee.

## Build and launch

```sh
go build -o omasheet ./cmd/omasheet
./omasheet [workbook.csv|workbook.xlsx]
```

With no path, choose **Open file**, **New CSV workbook**, or **New XLSX workbook** from the home screen or command palette. Loading and saving run asynchronously. A failed open keeps the current workbook; a failed save keeps it dirty and reports the cause. **Save as** chooses a new path but does not convert between CSV and XLSX; keep the destination extension consistent with the current workbook.

## Grid and editing

- Arrow keys move the active cell. Press Enter or type to edit; typing starts with the entered character, including spaces and Unicode.
- Enter commits and selects the cell below ready to type; Tab commits and selects the cell to the right. Esc cancels without committing.
- Drag with the left mouse button to select a rectangular range. Drag a column-header boundary to resize that column; column labels stay visible when the toolbar is hidden. The mouse wheel scrolls.
- Click a sheet tab or use Ctrl+PageUp / Ctrl+PageDown to switch sheets. **Add sheet** creates a sheet and activates it. **Insert row** and **Insert column** insert before the selected cell.
- Ctrl+Z / Ctrl+Y undo and redo. **Find** searches the active sheet for a case-insensitive substring.
- The formula bar shows the selected cell's source input. Enter formulas directly with `=`, for example `=SUM(A1:A5)`. Select ranges with the mouse, or use **Insert function** to open the supported-function picker and set its range and result cell. For `IF`, enter all three comma-separated arguments, for example `A1>0,"yes","no"`; the picker builds `=IF(A1>0,"yes","no")`.

The initial formula scope is arithmetic, cell references/ranges, and common functions `SUM`, `AVERAGE`, `COUNT`, `MIN`, `MAX`, and `IF`. Excelize performs calculation; formulas it cannot recalculate are retained as source and marked stale or unsupported rather than presenting an old cached result as current. This is not full Excel formula compatibility.

## Formatting and data operations

Ctrl+P opens the centered, searchable command palette over the current sheet. Search for an action or a shortcut; commands show their keybinding when one exists. The palette remains available when the toolbar is hidden. Press Ctrl+T or choose **Toggle toolbar** to hide/show the toolbar.

- Ctrl+O opens the floating file picker. Use the arrow keys to browse and Enter to open; Esc cancels. **Save as** uses the same popup flow.
- **Fill color** and **Font color** open terminal theme colors first, followed by custom RGB input (`#RRGGBB`). Theme colors are resolved only when selected. The app uses truecolor when supported and nearest ANSI colors otherwise.
- **Toggle bold** switches bold for the selected cell or range.
- **All borders**, **Outside borders**, **Top border**, **Bottom border**, **Left border**, **Right border**, and **Remove borders** apply to the selected range.
- **Merge selected cells** merges the selection. If non-top-left cells contain data, the app asks before discarding those values. **Unmerge selected cells** removes the merge.
- **Add conditional formatting** opens a dialog for an editable A1 range. Choose a single-color rule (greater than, less than, equal to, blank, or text contains) with fill/font color, or a two-/three-color scale. Theme colors precede custom RGB colors. Add multiple rule drafts, review the preview, then choose **Done** to apply; **Cancel** discards the drafts.
- **Sort by selected column** sorts the contiguous data block containing the active row, moving whole rows together. Choose `a` for ascending with a header, `d` for descending with a header, or `h` for ascending without a header.
- **Filter selected column** accepts allowed values separated by semicolons. Press Enter with no values to clear filters. Filters hide rows without deleting data.

## File format behavior

CSV contains one sheet and has no native styles, merges, formulas, or workbook metadata. Formula source is saved as CSV text; formatting and other-sheet content cannot be represented. OmaSheet warns before a CSV save that would discard styles or extra sheets.

XLSX retains workbook content through Excelize, including content not directly editable in the terminal UI when the library can round-trip it. OmaSheet does not execute macros and does not implement all Excel features. Formula calculation and conditional-format evaluation are limited to the supported subset. Keep backups of workbooks that depend on unsupported Excel features; do not assume every Excel-specific feature is editable or recalculated by OmaSheet.
