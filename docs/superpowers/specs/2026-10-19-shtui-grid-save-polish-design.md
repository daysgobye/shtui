# Shtui Grid and Save Interaction Design

**Status:** Approved for written-spec review  
**Date:** 2026-10-19

## Goal

Polish Shtui's current terminal spreadsheet flows without replacing the Go/Bubble Tea architecture or inspecting legacy implementations. Preserve the fast, theme-aware, mouse-capable Charm TUI while making workbook-only features understandable in CSV sessions and making merged cells behave as one visible cell.

## User-visible changes

### Launcher

- Increase the centered SHTUI wordmark so it has clear first-launch prominence.
- Use a compact wordmark when the terminal is too small for the full treatment; keep the existing centered actions and shortcut guidance usable.

### Toolbar and conditional formatting

- Keep column labels visible when the toolbar is hidden; the toolbar toggle controls only the toolbar.
- Add direct `Conditional` and `ƒx` Function actions beside Format. They invoke the existing conditional-format dialog and function picker.
- Keep the toolbar on one row. At ordinary widths, both new actions remain directly visible. Move lower-priority actions that do not fit into a `More` entry that opens the existing command palette; retain all commands there. The overflow layout must not change mouse hit targets for visible actions.
- Restyle the conditional-format dialog toward the approved grouped mockup: more space between sections; clear range, rule, style-preview, and rule-list areas; theme-derived colors; and visible focus on every control.
- The dialog supports mouse interaction and complete keyboard navigation: Tab and Shift+Tab traverse controls, arrows change focused choices, Enter activates the focused control, and Esc cancels. Color selection and adding/removing rules must not require a mouse.

### Fill and borders

Shtui's grid is rendered in terminal character cells. Column widths are integer character columns; row height is currently fixed at two terminal lines. A second render pass cannot split a terminal cell's background, so this design does not add fractional cell sizes or a layered renderer.

- Paint a shared vertical or horizontal border-glyph cell with a fill background only when both adjacent visible cells have the same non-empty effective fill.
- Keep exterior edges and edges between different or empty fills on the terminal-theme background.
- Paint a four-way junction background only when all four adjoining visible cells share the same non-empty effective fill; otherwise leave it theme-colored.
- Treat a merged range as one visual cell, using its top-left cell's effective fill for the merged region and its outside edges.
- Preserve border glyph foreground colors/styles independently of the fill decision.

The shared glyph cell is colored only where the adjoining regions already have the same fill, preventing color bleed into empty or differently colored cells while keeping contiguous same-color fills seamless.

### CSV notice and save choice

CSV cannot persist workbook structure or formatting. When a CSV session successfully applies a feature that CSV cannot store, show a small, theme-aware corner notice that dismisses automatically. Notify for successful additions of sheets, column-width changes, cell formatting/borders, merges, and conditional-format rules. Do not notify when merely opening a dialog or when an operation fails.

When saving a CSV session whose workbook contains such features, show the approved centered overlay with three actions:

1. **Save as CSV** — explicitly export the active sheet's cell inputs only to the current CSV path, or open the save picker for an untitled workbook. Other sheets and workbook formatting/structure will not be in the CSV file.
2. **Save as XLSX** — open the save picker with an `.xlsx` filename and preserve the supported workbook state.
3. **Cancel** — close the overlay without saving or changing the session.

The CSV export leaves the in-memory workbook and its features intact, so the user can still choose XLSX later. XLSX conversion reuses the session's in-memory Excelize workbook: CSV operations already create it lazily and keep it synchronized with the workbook model. Only after the XLSX file is successfully written and replaced does the session and workbook model switch their format/path to XLSX and record the new package snapshot. A canceled or failed save leaves the CSV session metadata and workbook untouched. This avoids a second serializer and keeps the active workbook consistent with the saved file.

### Merged cells

A merged range is one selectable/editable visual cell whose value is stored at the top-left coordinate.

- A mouse click anywhere inside a merged range selects its top-left coordinate.
- Keyboard movement must not expose hidden interior coordinates; moving into a merged range resolves to its anchor, and movement out of it skips the range's hidden cells.
- Editing from the grid or formula bar targets the anchor, never a hidden interior coordinate.
- Wrap displayed text over the merged range's combined visible width. Top-align it and use the merged range's existing height; do not automatically resize rows. If wrapped content exceeds that height, show an ellipsis at the visible end rather than drawing outside the merge.
- Suppress internal grid boundaries as today; use those interior lines as available merged-text space.

## Architecture and data flow

- Keep the existing Go, Bubble Tea, Bubbles, Lip Gloss, workbook model, and Excelize dependencies.
- Route toolbar buttons through the existing command IDs for conditional formatting and function insertion.
- Keep toolbar rendering and mouse hit-testing based on the same responsive item layout.
- Surface CSV notices as non-blocking transient UI state; the notice must not replace or obscure the save-choice overlay.
- Route save-choice actions through the controller. CSV uses the existing active-sheet exporter. XLSX uses the synchronized in-memory Excelize package and commits session format/path changes only after successful save completion.
- Centralize merged-coordinate normalization so mouse selection, keyboard navigation, and edit targeting cannot disagree about the visible cell.
- Keep the grid character-cell model. No sub-character drawing, dynamic row-height feature, or pixel rendering is introduced.

## Alternatives considered

### Reuse the in-memory Excelize workbook — selected

The CSV session already lazily builds an Excelize package for workbook operations, and those operations update it. Saving that package as XLSX avoids duplicating cell, formula, style, merge, and conditional-format serialization. The session changes format only after the file save succeeds.

### Rebuild an XLSX package from the workbook model

This isolates export from the live operation package, but creates a second serializer that can omit state or drift from normal workbook operations. Not selected.

### Export an XLSX copy while leaving the active session as CSV

This avoids changing active-session metadata, but keeps the session in CSV mode and continues to prompt about CSV losses on later saves. Not selected.

## Failure and boundary behavior

- A failed style, merge, sheet, width, or conditional-format operation produces no CSV compatibility notice.
- Canceling the save overlay, canceling the XLSX file picker, or failing an XLSX write does not change the session format/path or discard data.
- CSV output is intentionally lossy and contains only the active sheet. The live in-memory workbook remains available until the user closes or replaces it.
- Long merged-cell text never spills into neighboring cells. Its visible rendering is bounded by the merged range's interior width and height.
- Terminal sizes too small for all toolbar entries must preserve working overflow/command-palette access and accurate mouse targets.

## Acceptance checks

- At normal and compact terminal sizes, the launcher wordmark is more prominent without clipping its actions.
- At ordinary width, Conditional and Function are visible and clickable; toolbar hiding does not hide column labels; overflow actions remain reachable.
- Conditional-format controls match the approved hierarchy and can be completed, color-selected, and canceled by keyboard and mouse.
- Shared fill borders are colored only for equal non-empty neighboring fills; unlike/empty/exterior borders remain theme-colored; junction behavior follows the four-neighbor rule; border strokes remain visible.
- In a CSV session, each successful workbook-only feature shows a transient corner notice. Saving opens the centered dialog by keyboard or mouse.
- CSV save output contains only the active sheet's cell inputs; the in-memory workbook remains intact. XLSX save/reopen preserves supported values/formulas, sheets, widths, styles, borders, merges, and conditional rules. Cancel/failure preserves the CSV session.
- Merged-cell click and keyboard paths select the anchor; hidden interior coordinates cannot be edited; wrapped text is top-aligned, bounded, and ellipsized only when it exceeds the existing merge height.
- Run an actual terminal smoke: open a CSV, apply several workbook-only features, observe the corner notice, exercise both save choices, reopen the XLSX, and verify retained workbook features and merged-cell behavior.

## Out of scope

- Arbitrary pixel/fractional cell dimensions or a two-layer sub-character renderer.
- Automatic row-height adjustment for wrapped cells.
- General CSV-to-XLSX conversion of unsupported Excel features not represented by Shtui's current workbook model.
- Expanding formula coverage or adding unrelated spreadsheet features.
