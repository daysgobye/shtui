# Shtui Borders, Exit Flow, and Workbook Formula Design

**Status:** Draft for user review  
**Date:** 2026-10-19

## Goal

Improve three observed spreadsheet workflows in Shtui: visible colored borders, safe and informative exit behavior, and formula support for functions used by the user's six-sheet `Cozy_Bullet_Journal_Render_Test.xlsx` workbook. Preserve the Go/Bubble Tea architecture, existing theme-aware color picker, workbook model, and Excelize calculation dependency.

## User-visible changes

### Colored borders

- Remove the current All, Outside, Top, Bottom, Left, and Right border preset actions; the user reports that they do not provide useful visible borders.
- Add a **Border color** action that opens the existing theme-first/custom RGB color picker.
- Selecting a color applies a thin border of that color on all four edges of every selected cell. The operation applies immediately to the selection.
- Keep **Remove borders** as the explicit border-removal action.
- Use the existing per-edge border model and renderer/Excelize style serialization; do not add a second border representation.

### Quit confirmation and session summary

- The first Ctrl+C opens a centered confirmation popup over the current TUI view. It has **Yep!** and **Nope** actions, with **Nope** focused by default.
- Keyboard operation: arrows or Tab change the focused action; Enter activates it; Y confirms; N or Esc cancels. The popup also supports clicking either visible action.
- A second Ctrl+C while the confirmation is open quits immediately without another confirmation.
- When the workbook has unsaved changes, the confirmation explicitly warns that they will be lost. This flow does not add auto-save.
- After a confirmed quit, restore the terminal and print a Shtui wordmark and a concise last-action summary. Examples: `Saved workbook: /path/file.xlsx`; `Added Sheet2 to /path/file.xlsx (unsaved)`; or `New CSV workbook (not saved)`.
- The summary must describe completed actions only. A failed or canceled operation must not replace the last successful action. Sheet creation in a saved workbook identifies the workbook path and makes the unsaved state explicit. If no meaningful action occurred, show a neutral Shtui sign-off rather than inventing an action.
- Mouse and keyboard behavior must be equivalent; cancellation returns to the unchanged workbook view.

### Workbook formula support

- `COUNT` is already supported and remains available in the function picker.
- Add `COUNTIF`, `SUMIF`, `IFERROR`, and `COUNTA` to the formula picker and supported-function calculation guard only when the existing Excelize engine produces correct results for each function.
- Verify `COUNTA` specifically before enabling it; do not introduce a custom formula evaluator or claim support if Excelize cannot calculate it reliably.
- Ensure the picker exposes the functions' actual arguments, while direct cell/formula-bar entry continues to accept normal formulas.
- Use the fixture's real formulas as integration cases, including criteria/range references and nested calls. Preserve unsupported formulas as source and continue to surface calculation errors instead of substituting a value.
- Do not hide a real formula error such as a formula that attempts arithmetic on text.

## Architecture and data flow

- Keep Go, Bubble Tea/Bubbles, Lip Gloss, and the current workbook/Excelize layers; no new dependency is proposed.
- Route Border color through the existing color picker and use the existing selected-range operation path. Apply the chosen RGB/theme-resolved color consistently to all four stored edges.
- Use a topmost modal owner for quit confirmation so keyboard and pointer input cannot leak into the grid while it is open. The controller owns process termination and the post-TUI summary, after Bubble Tea restores the alternate screen.
- Track the latest successful user-visible action at the controller/workbook boundary, including destination path and whether subsequent edits are unsaved. Do not infer save success from a requested save or picker selection; update the summary only after the file write succeeds.
- Reuse Excelize's existing formula calculation path. Extend its supported-function guard and the function-picker catalog together, based on verified engine behavior.

## Failure and boundary behavior

- Canceling the color picker changes no borders. Applying a color changes only the current selection; Remove borders clears only the selected cells' borders.
- The quit popup's Nope/Esc action leaves the workbook and current UI state unchanged. A second Ctrl+C is an explicit immediate-quit bypass.
- A save that fails or is canceled is not reported as saved. An unsaved workbook or edits after the last successful save are clearly identified in the exit summary and quit warning.
- A function remains unsupported if the existing engine cannot reliably calculate it. Unsupported formulas remain preserved; calculation errors are not converted to success.
- Very small terminal dimensions must keep quit actions visible and mouse targets aligned with rendered controls, or provide a reachable compact layout.

## Acceptance checks

- Border UI no longer offers the six existing preset actions. Choosing a theme or custom color through Border color applies that color to all four edges of every selected cell; the rendered borders are visibly distinguishable from ordinary gridlines and survive XLSX save/reopen. Remove borders remains available and works on the selected range.
- Quit confirmation is centered, defaults to Nope, responds correctly to mouse and keyboard, cancels without mutation, warns when dirty, and quits immediately on the second Ctrl+C.
- After an actual confirmed quit, the terminal is restored before the Shtui wordmark and truthful last-action summary are printed. Exercise successful save, new unsaved workbook, added sheet, canceled save, failed save, and quit with no recorded action.
- The function picker lists supported fixture functions with correct argument prompts. Evaluate fixture formulas using COUNTIF, SUMIF, IFERROR, and COUNTA against expected values; confirm COUNT remains supported. If COUNTA fails the engine verification, omit it from the supported list and report the limitation.
- Reopen the fixture and verify representative formerly stale formula cells recalculate without changing their formula source; verify intentional errors remain errors.
- Run the relevant Go tests, `go test ./...`, build the native CLI, and smoke the real TUI paths including pointer and keyboard interaction.

## Out of scope

- Reimplementing Excel calculation semantics or adding a separate formula engine.
- Auto-saving on quit, changing the existing file format/save flow, or silently discarding unsupported XLSX features.
- Additional border shapes, border thickness controls, or per-edge color controls; the approved action applies one selected color to all four cell edges.
- Adding formulas not found in the workbook audit, except maintaining functions already supported.
- Inspecting legacy code or session history.
