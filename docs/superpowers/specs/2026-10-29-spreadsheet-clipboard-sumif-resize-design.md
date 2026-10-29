# Spreadsheet clipboard, SUMIF, and workbook actions

## Goal

Add dependable spreadsheet editing commands and formula support to Shtui while preserving the existing workbook model, undo/redo, XLSX round-tripping, and terminal interaction conventions. The scope includes system clipboard copy/cut/paste, row and column actions, sheet duplication, SUMIF, and persistence of mouse-resized XLSX columns.

## User-visible behavior

### Clipboard

- `Ctrl+Shift+C` copies the active cell or selected rectangular range to the system clipboard as tab-separated values (TSV).
- `Ctrl+X` cuts the active cell or selected rectangular range; `Ctrl+V` pastes clipboard TSV starting at the active cell.
- Ctrl+P offers Copy, Cut, and Paste for the active selection. Copy row and Cut row operate on the entire row containing the active cell, across the sheet's used columns.
- Clipboard text preserves cell input/formula source and rectangular alignment, including empty cells. Styles are not copied. A pasted leading `=` is interpreted as formula input by existing cell-write semantics.
- Clipboard errors are visible. A cut does not clear source cells unless the clipboard write succeeds. Range mutations are single undoable operations.
- The existing terminal quit behavior on `Ctrl+C` remains unchanged.

### Row, column, and sheet commands

- Ctrl+P offers Delete row and Delete column. Each removes the structure at the active cell, shifts later cells into the gap, updates formula references/dimensions, and is undoable/redoable.
- Ctrl+P offers Duplicate sheet. It duplicates the active sheet under a unique valid name and activates that new sheet. Duplicate the sheet's supported values/formulas, direct styles, column widths, merges, and conditional-format rules. Preserve each formula's text exactly; do not rewrite references to the duplicate. Use native Excelize copy support when it faithfully carries the supported sheet state; otherwise use the existing model and serialization APIs without changing unrelated workbook relationships.
- CSV's existing one-sheet serialization and feature-loss warnings remain authoritative; CSV cannot persist column widths or multiple sheets.

### SUMIF

- Evaluate `SUMIF(criteria_range, criteria, [sum_range])`, with omitted `sum_range` defaulting to `criteria_range`.
- Support standard single-range SUMIF matching: exact text, numeric and blank criteria; comparison criteria using `=`, `<>`, `<`, `<=`, `>`, `>=`; wildcard text criteria using `*` and `?`, with `~` escaping the wildcard characters. Text comparison is case-insensitive. Keep criteria and sum-range alignment row/column relative, and preserve formula source unchanged.
- Recalculate formula results and dependents when source cells change, including formulas loaded from and saved to XLSX. Invalid argument counts, invalid ranges, and unsupported criteria must not return fabricated totals; retain a clear stale/error state consistent with the existing formula model.
- This is support for SUMIF, not a claim of complete Excel formula compatibility.

### XLSX column resize persistence

- Resizing a column in an opened XLSX with the mouse persists through save and close/reopen, including the displayed width in the reopened workbook.
- Preserve the current width limits and save/preservation safeguards. Do not add CSV width persistence or sidecar configuration.

## Architecture and implementation boundaries

- Extend the existing `internal/workbook` operations and history model for range clipboard data, cell clearing/pasting, structural deletion, and sheet duplication. Keep workbook state and Excelize state synchronized through existing raw-operation patterns.
- Add a small system clipboard adapter around the existing clipboard dependency. Keep UI command handling independent of platform APIs by exposing copy/read/write errors through a narrow interface that can be tested without invoking the host clipboard.
- Wire keyboard bindings through `KeyMap` and actions through `AppModel.runCommand` and the existing Ctrl+P command palette. Preserve modal key ownership, `Ctrl+C` quit behavior, current range normalization, status reporting, and visible action summaries.
- Integrate SUMIF with formula recalculation without marking the standard supported cases stale. Reuse existing parsing/reference conventions where possible; avoid expanding unrelated formula-engine scope.
- Trace the mouse release → `SetColumnWidth` → XLSX save → `OpenFile` path with a regression scenario before changing resize code. Fix only the boundary shown to drop the persisted width.
- Update `docs/usage.md` for keyboard shortcuts, palette commands, SUMIF, and format-specific limitations.

## Error handling and invariants

- No cut clears data after a failed system clipboard write.
- Failed paste/structural operations report an error and do not claim success; successful workbook mutations participate in undo/redo.
- Clipboard dimensions and TSV alignment are preserved, including blank cells at range edges.
- Deleting the active row/column follows existing workbook limits and formula-reference behavior; deleted cells are restorable by undo.
- Duplicate names are avoided using existing sheet-name validation. Active sheet selection remains valid through undo/redo.
- Unsupported SUMIF criteria never silently produce a plausible but incorrect total.
- Saving/reopening must not discard allowed column-width edits or other preserved XLSX package parts.

## Verification

- Workbook tests: copy/paste rectangular boundaries and formula text; cut and undo/redo; row/column deletion and restoration, formula shifts, and dimensions; duplicate sheet contents/formula text/styles/widths/merges/rules and undo/redo; SUMIF exact, blank, numeric, comparison, wildcard/escape, omitted sum range, range alignment, and dependent recalculation.
- UI tests: Ctrl+Shift+C/Ctrl+X/Ctrl+V routing, Ctrl+P command invocation, row selection at the active cell, modal precedence, errors, and existing Ctrl+C behavior.
- XLSX persistence regression: open a saved workbook, exercise mouse column resize, save, reopen, and assert the persisted Excelize width and rendered grid width.
- Run relevant package tests, full `go test ./...`, build the executable, and smoke the real TUI through cell/block copy-cut-paste, row/column operations, sheet duplication, SUMIF recalculation, and opened-XLSX resize/save/reopen.

## Out of scope

- Copying cell formatting through the system clipboard.
- Full Excel formula-engine compatibility beyond the specified SUMIF behavior.
- Persisting layout for CSV files.
- Rewriting formulas during sheet duplication.
- Unrelated UI, workbook-format, or formula-engine redesign.
