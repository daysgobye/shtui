# Shtui usage

Shtui is a compiled Go terminal spreadsheet for CSV and XLSX files. It uses a virtualized grid and targets workbooks up to roughly 25,000 rows by 20 columns per sheet; larger files may work but are not a performance guarantee.

## Build and launch

```sh
go build -o shtui ./cmd/shtui
./shtui [workbook.csv|workbook.xlsx]
```

With no path, choose **Open file**, **New CSV workbook**, or **New XLSX workbook** from the home screen or command palette. Loading and saving run asynchronously. A failed open keeps the current workbook; a failed save keeps it dirty and reports the cause. **Save as** opens a floating picker and proposes the current file's basename with the format's extension. CSV-to-XLSX conversion is available from the save-choice dialog when CSV cannot preserve workbook features.

## Linux file-browser integration

From the repository checkout, run:

```sh
bash scripts/install.sh
```

The installer builds `shtui` into `~/.local/bin`, adds a user-level desktop entry that opens the terminal UI, and sets Shtui as the default application for CSV and XLSX files. It requires Go and `xdg-mime`; it does not use `sudo`. Setting CSV as the default may replace the app that currently opens CSV files. Run the script again after updating the checkout to rebuild and refresh the desktop entry.

## Grid and editing

- Arrow keys move the active cell. Press Enter or type to edit; typing starts with the entered character, including spaces and Unicode.
- Enter commits and selects the cell below ready to type; Tab commits and selects the cell to the right. Esc cancels without committing.
- Drag with the left mouse button to select a rectangular range. Drag a column-header boundary to resize that column; column labels stay visible when the toolbar is hidden. The mouse wheel scrolls.
- Click a sheet tab or use Ctrl+PageUp / Ctrl+PageDown to switch sheets. **Add sheet** creates a sheet and activates it. Use Ctrl+P → **Rename sheet** to rename the active sheet in a centered dialog; Enter applies, Esc cancels, and invalid names stay open with an error. Rename updates formula references and is undoable. **Insert row** and **Insert column** insert before the selected cell.
- Ctrl+Shift+C copies the selected rectangular range; Ctrl+X cuts it only after the OS clipboard write succeeds; Ctrl+V pastes at the active cell. **Copy**, **Cut**, and **Paste** are also available in Ctrl+P.
- Clipboard ranges use quoted tab-separated values with newline-separated rows, preserving blank rectangle edges, embedded separators, and formula source text. The UI copies raw cell inputs only; cell formatting/styles are not copied.

- The terminal temporarily enables the Kitty keyboard protocol's disambiguation mode while the alternate screen is active, then restores the previous keyboard mode before leaving it. Terminals without protocol support may not distinguish Ctrl+Shift+C from Ctrl+C.
- **Copy row** and **Cut row** operate on the active cell's row across its used-column span. **Delete row** and **Delete column** remove the active cell's row or column. **Duplicate sheet** creates and activates a uniquely named copy of the active sheet. All are available from Ctrl+P.
- Merged ranges are edited through their top-left cell. Clicking or navigating inside a merged range selects that anchor; its text wraps within the merge's existing width and height, with overflow ellipsized rather than changing row heights.
- Ctrl+Z / Ctrl+Y undo and redo. **Find** searches the active sheet for a case-insensitive substring.
- The formula bar shows the selected cell's source input. Enter formulas directly with `=`, for example `=SUM(A1:A5)`. Select ranges with the mouse, or use **Insert function** to open the supported-function picker and set its range and result cell. For `IF`, `COUNTIF`, `IFERROR`, and `SUMIF`, enter the full comma-separated argument list; examples: `A1>0,"yes","no"`, `A1:A5,">0"`, `A1/0,"safe"`, and `A1:A5,"tea",B1:B5`.

The formula scope is arithmetic, cell references/ranges, `SUM`, `AVERAGE`, `COUNT`, `COUNTA`, `MIN`, `MAX`, `IF`, `COUNTIF`, `IFERROR`, and `SUMIF`. SUMIF uses `SUMIF(criteria_range, criteria, [sum_range])`; omitting `sum_range` sums the criteria range itself. Criteria support case-insensitive exact text, numeric or blank matches, `=`, `<>`, `<`, `<=`, `>`, `>=`, and `*`/`?` wildcards with `~` escaping. When the sum range has a different size, calculation aligns from its first cell to the criteria range's shape: excess cells in a longer declared range are ignored, while a shorter declared range expands to include cells beyond its stated end. SUMIF criteria and ranges outside this supported syntax, and other formulas Excelize cannot calculate, remain stale or show an error rather than presenting an old cached result as current. This is not full Excel formula compatibility.

## Formatting and data operations

Ctrl+P opens the centered, searchable command palette over the current sheet. Search for an action or shortcut; commands show their keybinding when one exists. The palette remains available when the toolbar is hidden. The toolbar keeps **Conditional** and **ƒx** visible as primary actions when space permits, moving other actions under **More** at narrow widths. Press Ctrl+T or choose **Toggle toolbar** to hide/show the toolbar; column labels stay visible.

- Ctrl+O opens the floating file picker. Use the arrow keys to browse and Enter to open; Esc cancels. **Save as** uses the same popup flow.
- **Fill color** and **Font color** open terminal theme colors first, followed by custom RGB input (`#RRGGBB`). Theme colors are resolved only when selected. The app uses truecolor when supported and nearest ANSI colors otherwise.
- **Toggle bold** switches bold for the selected cell or range.
- **Border color** opens the theme-first/custom RGB picker and applies a thin border of that color to all four edges of every selected cell. **Remove borders** clears borders from the selected range.
- **Merge selected cells** merges the selection. If non-top-left cells contain data, the app asks before discarding those values. **Unmerge selected cells** removes the merge.
- **Add conditional formatting** opens a dialog for an editable A1 range. Choose a single-color rule (greater than, less than, equal to, blank, or text contains) with fill/font color, or a two-/three-color scale. Theme colors precede custom RGB colors. Use Tab / Shift+Tab to move through controls, arrows to choose conditions or scale type, Enter to open color pickers and activate Add, Remove, Done, or Cancel; Esc closes without applying. The dialog shows a preview and draft rule list. Add multiple rule drafts, review the preview, then choose **Done** to apply; **Cancel** discards the drafts.
- **Sort by selected column** sorts the contiguous data block containing the active row, moving whole rows together. Choose `a` for ascending with a header, `d` for descending with a header, or `h` for ascending without a header.
- **Filter selected column** accepts allowed values separated by semicolons. Press Enter with no values to clear filters. Filters hide rows without deleting data.

## File format behavior

CSV contains one sheet and has no native styles, merges, formulas, or workbook metadata. Formula source is saved as CSV text; formatting and other-sheet content cannot be represented. After a successful workbook-only change, Shtui briefly shows a corner notice that CSV cannot preserve the feature. When saving, **Save as CSV** exports only the active sheet's cell inputs and drops workbook-only features; **Save as XLSX** converts the live workbook and keeps the supported workbook state; **Cancel** leaves the session unchanged. CSV export does not remove features from the live workbook, so the warning appears again if you save it as CSV later.

XLSX retains workbook content through Excelize, including content not directly editable in the terminal UI when the library can round-trip it. Shtui does not execute macros and does not implement all Excel features. Formula calculation and conditional-format evaluation are limited to the supported subset. Keep backups of workbooks that depend on unsupported Excel features; do not assume every Excel-specific feature is editable or recalculated by Shtui.

## Quitting

The first Ctrl+C opens a centered confirmation dialog; **Nope** is selected by default. Use the arrow keys or Tab and Enter, press Y to quit, or N/Esc to stay. You can also click **Yep!** or **Nope**. A dirty workbook warns that unsaved changes will be lost. Press Ctrl+C again while the dialog is open to quit immediately. Shtui never auto-saves on exit.

After a confirmed quit, the terminal is restored and Shtui prints its wordmark and a summary of the last successful action. Unsaved changes show as **(unsaved)**; untitled workbooks show as **(not saved)**. Canceled or failed saves do not replace the previous successful action.
