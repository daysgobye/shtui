# XLSX Column Resize Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist mouse-resized columns in opened XLSX workbooks across save and reopen.

**Architecture:** Add a regression test spanning mouse release, workbook width mutation, XLSX serialization, and reopening. Trace the observed failing boundary and make the narrowest fix in the existing UI/workbook/save/load path.

**Tech Stack:** Go, Bubble Tea mouse events, Excelize XLSX read/write, existing workbook tests.

**Spec:** `docs/superpowers/specs/2026-10-29-spreadsheet-clipboard-sumif-resize-design.md`

## Global Constraints

- Resizing an opened XLSX by mouse MUST survive save and close/reopen in both persisted workbook width and rendered grid width.
- Preserve current width limits and XLSX package preservation safeguards.
- Do not add CSV width persistence or sidecar configuration.
- Follow systematic debugging: trace the failure boundary before selecting a fix; make no unrelated changes.

## Review Focus

- Resizing columns with existing content and an empty column: both widths survive reopen; pin in `internal/app/controller_test.go` or the lowest test layer that exercises the full path.
- Non-integer mouse delta is impossible in terminal columns; saved width remains the exact clamped integer shown by the grid; assert model and Excelize values.
- Existing workbook package preservation checks still allow the width-only sheet XML change without dropping unsupported parts; cover with an opened-workbook fixture.
- Undo/redo after resizing continues to restore and reapply widths; retain `internal/workbook/styles_test.go` coverage.
- Saving the same opened workbook path and Save As both preserve width; cover the observed path and an independent save-path case if the root cause is path-dependent.

---

### Task 1: Reproduce the reported boundary in a regression scenario

**Files:**
- Modify: `internal/ui/model_test.go` or `internal/app/controller_test.go` (choose the lowest existing layer that can exercise mouse release and save/reopen without a real terminal)
- Modify: `internal/workbook/xlsx_test.go` if the failing boundary is isolated to load/save

**Interfaces:**
- Consumes: existing `Grid.HandleMouse`, `SetColumnWidth`, `SaveFile`, and `OpenFile` paths.
- Produces: `TestMouseResizedOpenedXLSXColumnPersistsAfterSaveAndReopen` asserting the changed width in Excelize and the reopened UI grid.

- [ ] **Step 1: Write the regression test** with a saved XLSX containing data, open it with the existing session path, send mouse press/motion/release at a header boundary, save, reopen, and assert the new column width and rendered viewport width.
- [ ] **Step 2: Run `go test ./internal/ui ./internal/app ./internal/workbook -run 'TestMouseResizedOpenedXLSXColumnPersistsAfterSaveAndReopen' -count=1`; inspect the exact failure boundary and expected error/output.**

### Task 2: Fix the demonstrated persistence boundary

**Files:**
- Modify only the relevant implementation file among `internal/ui/model.go`, `internal/ui/grid.go`, `internal/workbook/styles.go`, `internal/workbook/xlsx.go`, and `internal/workbook/preservation.go`
- Modify: regression test file from Task 1

**Interfaces:**
- Consumes: the failing regression from Task 1.
- Produces: unchanged user-facing resize flow with saved and reopened width equal to the mouse-selected width.

- [ ] **Step 1: Implement the minimal root-cause fix** at the boundary shown by Task 1; retain current clamp values and preservation audits.
- [ ] **Step 2: Run the regression plus `go test ./internal/ui ./internal/app ./internal/workbook -run 'Test(MouseResizedOpenedXLSXColumnPersistsAfterSaveAndReopen|StyleAndColumnWidthRoundTrip|UndoColumnWidth)' -count=1`; expected: PASS.**
- [ ] **Step 3: Smoke the real TUI** against an existing XLSX: drag a column boundary, save, quit, reopen, and verify the visible width remains changed.
- [ ] **Step 4: Commit** only the regression and root-cause implementation files.
