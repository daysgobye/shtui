# Shtui Colored Borders Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace ineffective border presets with a theme/custom color action that applies visible thin borders to every selected cell edge.

**Architecture:** Keep border color in the workbook's existing per-edge `BorderStyle.ColorRGB` fields. Add explicit workbook operations for applying one color to all four edges and removing borders; route the UI action through the existing theme-first/custom RGB picker and selected-range path.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, Excelize. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-19-shtui-borders-exit-formulas-design.md`

## Global Constraints

- Keep Go, Bubble Tea/Bubbles, Lip Gloss, and the current workbook/Excelize layers; no new dependency is proposed.
- Route Border color through the existing color picker and use the existing selected-range operation path. Apply the chosen RGB/theme-resolved color consistently to all four stored edges.
- Keep the existing per-edge border model and Excelize style serialization; do not add a second border representation.
- Remove the current All, Outside, Top, Bottom, Left, and Right border preset actions; keep Remove borders.

## Review Focus

1. A multi-cell selection receives the chosen color on all four edges of every cell, not only the outer selection perimeter. Assert each edge for every cell in `TestApplyBorderColorAppliesAllEdgesToSelection`.
2. Applying borders preserves fill, font, number format, and cell inputs. Assert those fields in the same workbook-operation test.
3. Removing borders affects only selected cells and is undoable/redoable. Assert selection isolation and history in `TestRemoveBordersSelectedRangeAndHistory`.
4. Theme and custom RGB choices share the same application path; a color-picker click applies once and an outside click applies nothing. Assert both in `TestBorderColorPickerMouseActions`.
5. CSV feature notices appear only after a successful border application, not after cancel/failure. Assert success and failure paths in `TestBorderColorCSVNoticeOnlyAfterApply`.

---

### Task 1: Add colored all-edge and remove-border workbook operations

**Files:**
- Modify: `internal/workbook/styles.go`, `internal/workbook/xlsx.go`
- Modify: `internal/workbook/styles_test.go`

**Interfaces:**
- Consumes: `CellStyle.Borders`, `BorderStyle`, `applyStyledCellStates`, and existing undo/redo history.
- Produces: `ApplyBorderColor(session *Session, sheetName string, selection CellRange, colorRGB string) error` and `RemoveBorders(session *Session, sheetName string, selection CellRange) error`.

- [x] **Step 1: Write failing workbook-operation tests**
  - Add `TestApplyBorderColorAppliesAllEdgesToSelection`: apply `112233` to a 2×2 selection and assert all four edges on all four cells are thin and have that RGB value; assert pre-existing fill, font, number format, and input remain unchanged; save and reopen the XLSX and assert all four borders on all four cells are still present.
  - Add `TestRemoveBordersSelectedRangeAndHistory`: apply borders to two separated cells, remove borders from only one, assert the other remains bordered, then assert Undo and Redo restore the expected edges.
- [x] **Step 2: Run the focused tests and confirm they fail**

  Run: `go test ./internal/workbook -run 'TestApplyBorderColorAppliesAllEdgesToSelection|TestRemoveBordersSelectedRangeAndHistory' -count=1`

  Expected: FAIL because no color-aware all-edge operation exists.
- [x] **Step 3: Implement the two workbook operations**
  - Add `ApplyBorderColor` using the existing per-cell state/history path; set each edge to thin with `ColorRGB: colorRGB` for every coordinate in the selection.
  - Add `RemoveBorders` using the same history path and clear only the four edge values, preserving the remaining style fields.
  - Extend `workbookFromExcelize` to enumerate styled blank-cell references from Excelize's in-memory worksheet package parts, because `GetRows` omits empty styled cells; include those styles in the workbook model and update its used row/column dimensions.
  - Keep `ApplyBorderPreset` temporarily so the current UI caller remains buildable. Remove the obsolete exported API in Task 2 after the UI commands are migrated; do not retain it as a compatibility shim.
- [x] **Step 4: Run focused workbook tests**

  Run: `go test ./internal/workbook -run 'TestApplyBorderColor|TestRemoveBorders|TestUndoStyle' -count=1`

  Expected: PASS; RGB is stored on every selected edge, unrelated formatting is unchanged, and removal/history are range-local.
- [x] **Step 5: Commit** — `31114a5` (`feat(workbook): apply colored cell borders`)

  ```bash
  git add internal/workbook/styles.go internal/workbook/xlsx.go internal/workbook/styles_test.go
  git commit -m "feat(workbook): apply colored cell borders"
  ```

### Task 2: Replace border presets with the color-picker action

**Files:**
- Modify: `internal/ui/model.go`, `internal/ui/command_palette.go`, `internal/ui/toolbar.go`, `internal/ui/operations_test.go`, `internal/ui/command_palette_test.go`, `internal/workbook/styles.go`, `docs/usage.md`

**Interfaces:**
- Consumes: Task 1's `ApplyBorderColor` and `RemoveBorders`, plus existing `openColorPicker`, `ThemePalette`, `selectedRange`, and CSV notice behavior.
- Produces: a **Border color** command and toolbar action that opens the existing color picker; selecting a theme or RGB color applies all four borders. **Remove borders** remains directly reachable.

- [x] **Step 1: Write failing UI and palette tests**
  - Add `TestBorderColorPickerMouseActions` for a theme-color click applying the chosen RGB to all four edges, a custom RGB selection applying that value, and outside-click dismissal leaving borders unchanged.
  - Add `TestBorderColorAppliesToSelectedRange` for a multi-cell selection and `TestBorderColorCSVNoticeOnlyAfterApply` for successful apply versus canceled picker.
  - Add a palette assertion that **Border color** and **Remove borders** are present and all six old edge-preset entries are absent.
- [x] **Step 2: Run focused tests and confirm they fail**

  Run: `go test ./internal/ui -run 'TestBorderColor|TestRemoveBorders|TestCommandPalette' -count=1`

  Expected: FAIL because the palette still exposes the preset actions and the color picker has no border target.
- [x] **Step 3: Implement UI routing and remove obsolete actions**
  - Add a border color target and route it through `ApplyBorderColor` after resolving the selected theme/custom color.
  - Replace the toolbar's Borders menu action with **Border color**; remove old preset command registrations and dispatch; keep **Remove borders** routed through `RemoveBorders`.
  - Show the existing CSV feature notice only after a successful apply; update `docs/usage.md` to describe the two border actions and color behavior.
- [x] **Step 4: Run focused UI and workbook tests**

  Run: `go test ./internal/ui -run 'TestBorderColor|TestRemoveBorders|TestCommandPalette' -count=1 && go test ./internal/workbook -run 'TestApplyBorderColor|TestRemoveBorders' -count=1`

  Expected: PASS; picker mouse actions apply once, cancellation does not mutate, old presets are gone, and removal remains available.
- [ ] **Step 5: Commit**

  ```bash
  git add internal/ui/model.go internal/ui/command_palette.go internal/ui/toolbar.go internal/ui/operations_test.go internal/ui/command_palette_test.go internal/workbook/styles.go docs/usage.md docs/superpowers/plans/2026-10-19-shtui-colored-borders.md
  git commit -m "feat(ui): add color picker for cell borders"
  ```
