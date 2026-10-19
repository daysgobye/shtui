# OmaSheet TUI Interaction Redesign

## Goal

Make the existing compiled Go CSV/XLSX terminal spreadsheet usable and visually coherent for everyday sheet work, while preserving its virtualized custom grid, workbook behavior, terminal-theme colors, and supported unknown-feature round-trip path.

## Constraints and audience

- Bubble Tea v1 app; retain Go as the only application language and native compiled startup.
- Keep the existing custom `GridViewport`; it owns cell-level styling, selection, filtering, widths, and merged-cell display.
- Keep the current practical sheet scale target (about 25,000 rows by 20 columns per sheet) and avoid eager full-sheet rendering.
- Use terminal palette colors for chrome and preset formatting; offer custom RGB after theme colors.
- Preserve current CSV/XLSX load/save semantics, asynchronous load/save, dirty-workbook protection, and unknown-feature preservation. Do not add macro execution or claim full Excel compatibility.
- Preserve working mouse sheet-tab selection.

## Architecture decision

Keep the custom virtualized grid and integrate Charm Bubbles v0.21 components for interaction surfaces: `list` for command/function/shortcut search, `filepicker` for workbook browsing, `textinput` for cell and filename entry, and `help` for contextual keybinding hints. The project uses Bubble Tea v1.3.10; Bubbles v0.21 targets Bubble Tea v1.

Add one UI-level overlay compositor that places a centered, readable modal over the existing base view rather than replacing the workbook view. The modal covers only its own footprint; the surrounding live grid, toolbar, headers, tabs, and status remain visible. Each overlay has explicit Escape/Cancel behavior and owns its input while open.

Do not replace the grid with Bubbles `table`: the sheet requires virtualized rows, per-cell styles, filtered-row mapping, resizing, and merged ranges that the current grid owns.

## Interaction contract

### Workbook shell and grid

- Use the approved Crush-inspired direction: rounded grouped chrome, compact toolbar controls, clear active-cell/range treatment, and accent colors derived from the terminal palette.
- Show the toolbar by default. Hide/show affects only the toolbar row; row numbers and column letters remain visible in either state.
- Render subtle cell gridlines by default, plus strong direct borders for formatted edges. Column boundaries are visible and accept a small mouse hit zone; dragging still resizes only the targeted column.
- Render merged ranges as joined cell regions: suppress internal edges and display the top-left value. Keep the existing confirmation before discarding covered-cell values and provide success/error status.
- Offer border presets for all, outside, top, bottom, left, right, and none.
- Mouse clicks on sheet tabs continue to activate the clicked sheet. Ctrl+PageUp/Down and command-palette previous/next sheet commands provide keyboard alternatives.

### Editing and formulas

- Use Bubbles `textinput` for cell input and filename/range fields. Accept spaces, Unicode runes, and pasted text.
- Enter commits the edit and selects the cell below. Tab commits and selects the cell to the right. The next typed value begins editing the newly selected cell. Escape cancels the current edit without moving.
- Display the selected cell or range in A1 notation. Mouse drag remains the primary range-selection gesture.
- Keep direct formula entry (`=SUM(A1:A5)`) and add a searchable function picker for the currently supported functions: `SUM`, `AVERAGE`, `COUNT`, `MIN`, `MAX`, and `IF`. The picker exposes syntax/examples and a range field initialized from the selection, with a formula preview before applying.
- Preserve formula source, result, and stale/error distinctions. Do not silently convert unsupported formulas to literals.

### Command palette and keybindings

- Ctrl+P opens a centered Bubbles `list` over the live workbook. Search is fuzzy; arrows move; Enter invokes; Escape closes.
- Command rows show their keybinding where one exists. Include a searchable **Keyboard shortcuts** section whose entries match by action name and key name.
- Use Bubbles `help` for compact contextual hints; do not make help the only lookup path because the user needs searchable action-to-key lookup.
- Reuse the same overlay frame, theme-aware border, and focus styling for the palette and other dialogs.

### File picker

- Open File launches a centered Bubbles `filepicker` modal, limited to supported `.csv` and `.xlsx` files. Directory enumeration remains asynchronous so the UI stays responsive.
- Selecting a file reuses the existing asynchronous controller load path. Keep the current workbook intact on load failure and preserve the dirty-workbook guard.
- Save As uses the same popup surface with directory browsing and a Bubbles text input for the filename; retain the current CSV-loss warning and save-result behavior.

### Formatting and conditional rules

- The conditional-format editor is a centered popup modeled on the supplied reference: editable Apply to range, Single color and Color scale tabs, rule controls, formatting style, Cancel/Done, and Add another rule.
- Default Apply to range to the current selection. Keep supported single-color operators (greater than, less than, equal, blank, text contains) and expose theme colors before custom RGB.
- Implement two- and three-color scales using Excelize conditional formatting and render their effective colors in the terminal grid. Persist the rule with the workbook; do not display a scale mode that is not applied.
- Multiple rules are added one at a time and retain their selected range and style. Invalid range, rule value, or color leaves the dialog open with a visible error rather than mutating the workbook.

## Error and state behavior

- Modal input takes precedence over grid shortcuts; Escape closes/cancels the active modal only.
- A failed cell write, workbook load/save, resize persistence, merge, border, or conditional rule reports a visible status and does not pretend success.
- Closing an overlay restores the underlying selection, scroll position, active sheet, and toolbar visibility.
- Only the top-left value survives a confirmed merge, matching current workbook semantics. Undo restores original cell values and merge metadata.

## Acceptance checks

- UI input test enters `Quarterly plan` including spaces, commits it, and proves Enter/Tab target the correct neighboring cells.
- Mouse resize test starts within the displayed boundary hit zone, drags, and asserts only the intended persisted column width changes.
- Render tests show gridlines, explicit border edges, and a visually joined merged range; merge confirmation and undo preserve data.
- Palette and file-picker tests assert the base workbook remains visible around the centered popup, and Escape restores the unchanged sheet.
- File-picker tests cover CSV/XLSX filtering, directory navigation, open failure, and dirty-workbook protection.
- UI formula test enters `=SUM(A1:A5)` through the editor and asserts the calculated value; function-picker test pre-fills the selected range and produces the same formula result.
- Conditional-format tests cover single-color matching, 2/3-color scale persistence and terminal rendering, multiple rules, range editing, cancellation, and invalid input.
- Navigation tests preserve mouse tab selection and verify keyboard previous/next sheet behavior.
- Keybinding search finds an action by key and by action name.
- Existing workbook and preservation tests remain unchanged and pass; the UI is smoke-tested in a real terminal at small and normal dimensions.

## Alternatives considered

1. **Keep custom grid, add Bubbles overlays (selected):** least risk to virtualization, styling, and workbook behavior; uses maintained Charm components for the UI primitives they provide.
2. **Replace grid with Bubbles table:** reduces custom rendering but does not fit merged cells, per-cell Excel styles, or the virtualized sheet contract; higher rewrite and performance risk.
3. **Keep all UI bespoke:** avoids the Bubbles dependency but duplicates list, filtering, input, and file-picker behavior; conflicts with the requested Charm ecosystem and increases maintenance.

## Out of scope

- Full Excel function compatibility, macros, charts, pivots, and arbitrary formula editing.
- Replacing the workbook engine or changing the CSV/XLSX persistence format.
- Increasing the supported sheet-size target or removing virtualization.
