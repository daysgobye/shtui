# Spreadsheet Editing Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add system clipboard range editing, row/column actions, and duplicate-sheet support with undo/redo.

**Architecture:** Keep range and structural mutations in `internal/workbook`, record them through the existing history system, and route keyboard/palette input through `AppModel`. A small injectable UI clipboard adapter wraps the existing system clipboard dependency; TSV is the interchange format.

**Tech Stack:** Go, Bubble Tea, existing Excelize session/history, existing system clipboard dependency.

**Spec:** `docs/superpowers/specs/2026-10-29-spreadsheet-clipboard-sumif-resize-design.md`

## Global Constraints

- `Ctrl+Shift+C` copies, `Ctrl+X` cuts, and `Ctrl+V` pastes; preserve existing `Ctrl+C` quit behavior.
- Clipboard ranges use TSV, retain cell input/formula source and rectangular alignment, and do not transfer styles.
- A failed clipboard write MUST NOT clear cut source cells.
- Successful workbook mutations MUST be undoable/redoable.
- Duplicate formulas MUST remain exactly as written; CSV remains one-sheet and cannot persist widths.

## Review Focus

- Clipboard failure during cut: source data stays unchanged; test with injected write failure in `internal/ui/clipboard_test.go`.
- Rectangles with empty edge cells: TSV dimensions remain intact after copy/paste; test in `internal/workbook/clipboard_test.go`.
- Ctrl+C remains quit and shortcuts do not leak through open modals; test in `internal/ui/model_test.go`.
- Formula references and dimensions after row/column deletion: verify shifted references and full undo/redo in `internal/workbook/operations_test.go`.
- Duplicate sheet formula text, styles, merges, widths, and conditional rules survive duplicate/undo/redo; verify in `internal/workbook/operations_test.go`.

---

### Task 1: Workbook rectangular clipboard operations

**Files:**
- Modify: `internal/workbook/clipboard.go`
- Create: `internal/workbook/clipboard_test.go`

**Interfaces:**
Consumes: existing `CopyRange(session *Session, sheetName string, selection CellRange) ([][]string, error)` and `PasteRange(session *Session, address CellAddress, rows [][]string) error`. These workbook APIs preserve the normalized rectangle and exact `Cell.Input`; keep the matrix representation inside the workbook layer. Produces: `ClearRange(session *Session, sheetName string, selection CellRange) error`, recording one undoable clear. System clipboard TSV serialization/parsing belongs to the UI adapter, which passes matrices to these existing workbook APIs.

- [ ] **Step 1: Write/retain tests** for rectangular `CopyRange` source preservation, `ClearRange` undo/redo, and `PasteRange` rectangular boundaries/undo/redo. Assert exact matrix contents, formulas, changed cells, and history state.
- [ ] **Step 2: Implement `ClearRange`** in `internal/workbook/clipboard.go`; use existing cell-state/history helpers so a clear spanning many cells remains one undoable operation. Keep `CopyRange` and `PasteRange` signatures unchanged.
- [ ] **Step 3: Run `go test ./internal/workbook -run 'Test(CopyRange|ClearRange|PasteRange)' -count=1`; expected: PASS.**
- [ ] **Step 4: Commit** the workbook clipboard implementation and tests.

### Task 2: Structural deletion and sheet duplication

**Files:**
- Modify: `internal/workbook/operations.go`
- Modify: `internal/workbook/operations_test.go`

**Interfaces:**
- Produces: `DeleteRows(session *Session, sheetName string, at, count int) error`, `DeleteColumns(session *Session, sheetName string, at, count int) error`, and `DuplicateSheet(session *Session, sourceName, newName string) error`.
- Deletion removes the requested structure at zero-based `at`, shifts later cells, updates formula references and dimensions, and records one undoable snapshot/history entry. Duplication validates `newName`, activates the new sheet, and preserves formula inputs exactly plus modeled styles, widths, merges, and conditional-format rules.

- [ ] **Step 1: Write tests** `TestDeleteRowsShiftsCellsAndFormulaReferencesUndoRedo`, `TestDeleteColumnsShiftsCellsWidthsAndFormulaReferencesUndoRedo`, and `TestDuplicateSheetCopiesSupportedStateAndFormulaTextUndoRedo`. Include direct deleted-cell refs (`#REF!`), unaffected refs on other sheets, ranges fully before/after deletion, and partially surviving row/column ranges whose start or trailing endpoint is deleted; formulas must cover exactly their surviving cells. Assert shifted values/references, dimensions, unique sheet contents, and exact formulas.
- [ ] **Step 2: Implement deletion using existing shift logic** and implement duplication using Excelize sheet-copy support when it preserves the specified state; synchronize the workbook model and ensure undo/redo restores the active-sheet index.
- [ ] **Step 3: Run `go test ./internal/workbook -run 'Test(DeleteRows|DeleteColumns|DuplicateSheet)' -count=1`; expected: PASS.**
- [ ] **Step 4: Commit** structural operations and tests.

### Task 3: Keyboard, palette, and system clipboard integration

**Files:**
- Create: `internal/ui/keyboard_protocol.go`
- Create: `internal/ui/keyboard_protocol_test.go`
- Modify: `internal/app/controller.go` (enable terminal keyboard protocol through Bubble Tea options)
- Create: `internal/ui/clipboard.go`
- Create: `internal/ui/clipboard_test.go`
- Modify: `internal/ui/keymap.go`
- Modify: `internal/ui/command_palette.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/model_test.go`
- Modify: `go.mod` (promote the already-present clipboard library to a direct dependency)
- Modify: `docs/usage.md`

**Interfaces:**
- Consumes: workbook matrix APIs from Tasks 1–2.
- Produces: an injectable UI clipboard interface with `Read() (string, error)` and `Write(string) error`; keyboard/palette routes for Copy, Cut, Paste, Copy row, Cut row, Delete row, Delete column, and Duplicate sheet. A Kitty keyboard protocol adapter disambiguates Ctrl+Shift+C on capable terminals while preserving Ctrl+C and existing control shortcuts.

- [ ] **Step 1: Write UI tests** `TestClipboardShortcutsOperateOnSelectedRange`, `TestClipboardCutFailurePreservesSource`, `TestPaletteClipboardAndStructuralCommands`, `TestCopyCutRowUseActiveCellRow`, `TestClipboardModalPrecedenceAndCtrlCQuit`, and tests for Kitty key decoding/mode enable and restore. Assert command effects, errors, and Ctrl+C behavior.
- [ ] **Step 2: Implement the injectable adapter and command routing.** Serialize `CopyRange` matrices to TSV and parse clipboard TSV back to matrices in the UI layer; preserve empty rectangle edges and formulas. For cut, copy → successful clipboard write → clear. Use the system clipboard library; copy/cut rows over the used-column span; generate a unique duplicate-sheet name through existing name validation conventions. Enable the Kitty keyboard protocol only while the TUI's alternate screen is active; translate enhanced key events into Bubble Tea messages without consuming Ctrl+C or existing control shortcuts.
- [ ] **Step 3: Update `docs/usage.md`** with shortcuts, palette commands, TSV/formula behavior, and the style-copy limitation.
- [ ] **Step 4: Run `go test ./internal/ui ./internal/workbook -run 'Test(Clipboard|Copy|Cut|Paste|Palette.*Structural|DeleteRows|DeleteColumns|DuplicateSheet)' -count=1`; expected: PASS.**
- [ ] **Step 5: Smoke the real TUI** with an XLSX workbook: copy/paste a cell and a multi-cell rectangle, cut/paste, execute row/column commands, duplicate a sheet, and verify OS clipboard interoperability and undo.
- [ ] **Step 6: Commit** UI, docs, and integration tests.
