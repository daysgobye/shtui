# Terminal CSV/XLSX Spreadsheet — Design

## Status and intent

This design was approved in chat, then revised from the user's written-spec feedback. It is awaiting review of this revision. It defines a compiled, fast-starting terminal spreadsheet for practical CSV and Excel workbook work. Repository code and prior sessions were not inspected.

## Goals

- Open CSV and XLSX-family spreadsheets in a modern terminal UI with keyboard and mouse input.
- Keep startup and visible-grid interaction fast; target sheets up to 25,000 rows × 20 columns.
- Cover common editing, formatting, formulas, sheets, sorting, filtering, and conditional-formatting tasks without implementing all of Excel.
- Avoid silently destroying workbook content the app cannot edit.

## Non-goals

- Full Excel feature or formula compatibility.
- Creating, editing, refreshing, or recalculating charts, PivotTables, or other advanced Excel features.
- Executing VBA/macros.
- Supporting million-row sheets as a performance target.

## Recommended stack

- **Go** for a compiled single-binary application.
- **Bubble Tea** for the event-driven TUI and keyboard/mouse input.
- **Excelize** for XLSX-family workbook reading/writing, cell styles, conditional formatting, and formula calculation through `CalcCellValue`.
- Go's CSV package for CSV import/export.

Keep the TUI separate from workbook/file operations. The session model owns workbook state and edits; the UI renders the current viewport and translates keyboard/mouse events into model actions. File adapters load and save CSV and XLSX-family files. Formula calculation uses Excelize rather than a second formula engine.

References: [Bubble Tea API](https://pkg.go.dev/github.com/charmbracelet/bubbletea), [Excelize project](https://github.com/qax-os/excelize), [Excelize formula calculation](https://xuri.me/excelize/en/cell.html#CalcCellValue).

## User interface and launch behavior

Use a toolbar-first layout, selected from visual mockups: top-level File/Edit/View/Data/Format menus; visible undo/redo, insert-row/column, fill/font color, sort, and filter controls; name box and formula bar; central grid; sheet tabs; status bar with selection and dimensions. Keep the grid as the dominant surface. Support mouse selection, wheel scrolling, and drag-to-resize column boundaries, with keyboard navigation and shortcuts for the same core operations. Include a `Ctrl+P` command palette for searching and invoking actions. The toolbar is visible by default and can be hidden or restored from View or the command palette; the palette remains available while the toolbar is hidden.

Launching with a path opens that workbook directly. Launching without a path shows an open/create screen. Do not add splash animations, network lookups, or background discovery before the usable screen.

### Terminal theme and color selection

Use the terminal's default foreground/background and ANSI theme colors for the application chrome; avoid painting a fixed background so the app fits both light and dark terminal themes. Render theme-color choices through ANSI palette slots.

Cell fill, font, border, and conditional-format color menus show the terminal theme palette first, followed by a custom RGB/hex entry. Use 24-bit color for custom colors when supported; otherwise map to the nearest available terminal palette color. Store a concrete color in XLSX styles; when the terminal cannot report its configured ANSI RGB values, use a documented standard ANSI-to-RGB mapping for saved theme-color choices.

## File and workbook behavior

### CSV

CSV is one sheet of cell data. It cannot represent formatting, conditional formatting, multiple sheets, or workbook metadata. In the app session, formula cells use the common formula behavior below; CSV export writes formula source as CSV text, so no cached formula result is stored in the file. Saving a styled or multi-sheet workbook as CSV must warn that unsupported workbook features will be discarded.

### XLSX-family workbooks

Open and save `.xlsx` and `.xlsm`; do not execute macros. Preserve unsupported workbook parts where Excelize can round-trip them, including supported pass-through content. Unsupported features remain uneditable in the app. Do not describe preservation as universal: test representative workbook fixtures, including charts, PivotTables, data validation, and VBA projects where applicable. If the app detects a feature that the selected save path would drop, refuse to overwrite the original and explain the issue.

For all formats, save to a temporary sibling file and replace the original only after serialization succeeds. A failed save leaves the original intact. Report file errors with the path and actionable reason.

## V1 editing and data operations

- Edit cells and rectangular selections; copy/paste tabular data; undo/redo.
- Insert rows and columns.
- Add, rename, and delete sheets.
- Resize columns by dragging a header boundary.
- Apply fill color, font color, bold, common number/date formats, and basic border presets. Color menus list terminal theme colors first, then custom RGB/hex input.
- Merge/unmerge rectangular selections. Keep the top-left value. If another selected cell is nonempty, ask before discarding its value.
- Sort a selected rectangular data range as a whole. When sorting from a cell or column, infer the contiguous data range; sort a single column in isolation only when the user explicitly requests it. Block sorting ranges containing merged cells.
- Filter rows by column values without changing their underlying cell data.
- Find text within the active sheet.
- Apply conditional formatting to a range with simple value/text conditions (greater/less/equal, blank, and text-contains classes) and fill/font colors.

## Formula behavior

Use Excelize `CalcCellValue` for formulas entered or edited in the app. The initial supported scope is arithmetic, cell references/ranges, and common functions `SUM`, `AVERAGE`, `COUNT`, `MIN`, `MAX`, and `IF`. Recalculate affected formulas when dependencies change. Verify dependency recalculation behavior against Excelize before implementation; do not display a cached result as current if the app cannot recalculate it.

Excelize documents calculation as in progress and lists unsupported formula types. Preserve unsupported formula text and any stored result. Mark such a result as not recalculated; show calculation errors distinctly from literal cell contents. Do not claim Excel compatibility beyond the tested supported subset.

## Performance

Virtualize grid rendering: draw only visible cells and headers, while preserving selection and scroll position across redraws. CSV loading should be incremental where practical. XLSX opening may require parsing workbook content through Excelize; show progress rather than blocking the UI without feedback. Measure both cold application startup and open/edit/save latency at 25,000 × 20 per sheet on the target workstation. Larger files may work, but responsiveness beyond the target is not guaranteed.

## Errors and safety

- Invalid or unreadable files: show the filename and cause; leave the current workbook unchanged.
- Unsupported formula: preserve source, mark result stale/unavailable, and continue editing.
- Merge with nonempty cells: require confirmation before dropping non-top-left values.
- Save failure: keep original file and report the failure; never present an unsaved state as saved.
- Suspected XLSX feature loss: refuse overwrite rather than silently discard content.

## Verification

- Use deterministic tests for formula results and dependency updates, range sorting, filtering without data mutation, merge confirmation semantics, conditional-format rule behavior, command-palette access while the toolbar is hidden, theme palette ordering, custom color input, and save failure safety. Include fixture-based XLSX round trips for supported styles and representative unsupported features; compare reopened workbook content and package relationships rather than only checking that serialization returns no error.

- Smoke the real TUI: launch with and without a path, open/edit/save/reopen CSV and XLSX, exercise sheet tabs, formula recalculation, mouse selection, column resizing, toolbar hide/show, command-palette invocation, theme colors, custom colors, and scrolling. Benchmark cold launch and a 25,000 × 20 sheet. A successful unit-test suite alone is not proof of the TUI or preservation behavior.
