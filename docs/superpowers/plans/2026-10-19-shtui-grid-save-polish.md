# Shtui Grid and Save Interaction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the approved launcher, toolbar, conditional-formatting, fill-boundary, CSV save, and merged-cell interaction improvements.

**Architecture:** Keep the existing Go/Bubble Tea application and workbook model. Share one responsive toolbar layout between rendering and mouse hit-testing; normalize merged coordinates in the grid; use the CSV session's synchronized Excelize package for XLSX conversion; present transient notices and save choices as theme-aware overlays.

**Tech Stack:** Go, Bubble Tea, Charm Bubbles, Lip Gloss, Excelize. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-19-shtui-grid-save-polish-design.md`

## Global Constraints

- Use the current Go implementation only; do not inspect or restore legacy source or sessions.
- Preserve the existing terminal character-cell grid; do not add fractional cell sizes, a second background layer, or automatic row-height changes.
- Use theme-derived colors for new UI chrome and keep all existing commands reachable through the command palette.
- Reuse the synchronized in-memory Excelize workbook for CSV-to-XLSX saves; do not add a second XLSX serializer.
- CSV export writes only the active sheet's cell inputs and leaves the in-memory workbook intact.
- Merged-cell display and editing use the top-left anchor; wrapped text stays inside the current merge dimensions.

## Review Focus

1. CSV-to-XLSX save before `session.file` exists, full supported workbook state on conversion, and a failed write leaving CSV format/path unchanged. Test all three cases in Task 3.
2. CSV export from a multi-sheet workbook with formatting; the file contains only the active sheet while the live session retains its sheets and styles. Test in Task 4.
3. Same-color, unlike-color, empty, exterior, and four-way fill boundaries, including merged ranges; only same-fill joins receive boundary background. Test in Task 1.
4. Merged text with wide Unicode, multiple rows/columns, and content exceeding available height; it wraps within the block, ellipsizes at its end, and never edits hidden coordinates. Test in Task 2.
5. Narrow terminal widths and toolbar hit-testing after overflow; visible actions stay clickable and hidden actions remain reachable. Test in Task 6.

---

### Task 1: Match fill background to shared neighboring cells

**Files:**
- Modify: `internal/ui/grid.go`
- Test: `internal/ui/grid_test.go`

**Interfaces:**
- Consumes: `workbook.EffectiveStyleResolver`, `mergeAt`, existing boundary-rendering helpers.
- Produces: shared-edge and junction backgrounds that follow the approved equal-fill rule; no new exported API.

- [ ] **Step 1: Write failing grid tests**
  - Add `TestGridSharedBoundaryFillRequiresMatchingAdjacentFills` for equal non-empty fills, different fills, one empty fill, and exterior edges in both orientations.
  - Add `TestGridJunctionFillRequiresFourMatchingCells` for four matching fills and each mismatch/empty case.
  - Add `TestGridMergedBoundaryFillUsesAnchor` for a merged region adjacent to an equally filled cell and to an unlike-filled cell.
  - Update `TestGridFillTouchesCellBoundaries`, `TestGridSharedBoundaryUsesLeftAndUpperFill`, and the fill-background assertion in `TestFilledMergedCellKeepsOuterBorders`; they currently pin the old one-sided separator-background behavior.
- [ ] **Step 2: Run the focused tests and confirm they fail**

  Run: `go test ./internal/ui -run 'TestGridSharedBoundaryFillRequiresMatchingAdjacentFills|TestGridJunctionFillRequiresFourMatchingCells|TestGridMergedBoundaryFillUsesAnchor' -count=1`

  Expected: FAIL because a single adjacent cell currently supplies the border-glyph background.
- [ ] **Step 3: Implement matching boundary-fill selection in `internal/ui/grid.go`**
  - Compare both adjacent cells' effective fills for each internal edge; return no fill for exterior edges or empty/mismatched fills.
  - For a four-way junction, require all four adjoining effective fills to be the same non-empty value.
  - Resolve coordinates inside merges to the merge anchor before comparing fills; keep border foreground styling unchanged.
- [ ] **Step 4: Run grid tests**

  Run: `go test ./internal/ui -run 'TestGrid|TestFilledMergedCell' -count=1`

  Expected: PASS; same-color joins blend, while empty/unlike/exterior boundaries have no fill background and border glyphs remain visible.
- [ ] **Step 5: Commit**

  ```bash
  git add internal/ui/grid.go internal/ui/grid_test.go
  git commit -m "fix(ui): prevent fill bleed at grid edges"
  ```

### Task 2: Wrap merged text and make merged ranges one selectable cell

**Files:**
- Modify: `internal/ui/grid.go`, `internal/ui/model.go`
- Test: `internal/ui/grid_test.go`, `internal/ui/model_test.go`

**Interfaces:**
- Consumes: Task 1's fill resolution; current `GridViewport.CellAt`, `AppModel.selectCell`, and `AppModel.moveSelection` paths.
- Produces: `normalizeToMergeAnchor(sheet *workbook.Sheet, coord workbook.Coord) workbook.Coord`, used by mouse selection, keyboard selection, and edit targeting.

- [ ] **Step 1: Write failing merged-cell interaction tests**
  - Add `TestCellAtNormalizesMergedCoordinatesToAnchor` for clicks in the first, interior, and final cells of a merge.
  - Add `TestKeyboardMovementSkipsMergedInterior` for entering a merge and moving out horizontally and vertically without selecting hidden coordinates.
  - Add `TestMergedTextWrapsWithinExistingArea` with a multi-row/multi-column merge and wide Unicode text; assert top alignment, line wrapping, no drawing beyond the merge, and a visible ellipsis when vertical space is insufficient.
- [ ] **Step 2: Run the focused tests and confirm they fail**

  Run: `go test ./internal/ui -run 'TestCellAtNormalizesMergedCoordinatesToAnchor|TestKeyboardMovementSkipsMergedInterior|TestMergedTextWrapsWithinExistingArea' -count=1`

  Expected: FAIL because mouse/keyboard selection can land on hidden coordinates and merged text is truncated to one line.
- [ ] **Step 3: Implement anchor normalization and bounded wrapping**
  - Normalize coordinates in `CellAt` and the keyboard selection path; when leaving an anchor, move to the first coordinate beyond the merge on that axis.
  - Render wrapped text across the merged range's combined visible width and existing interior height. Keep row heights fixed and ellipsize overflow without drawing into neighboring cells.
- [ ] **Step 4: Run focused UI tests**

  Run: `go test ./internal/ui -run 'TestCellAt|TestKeyboardMovementSkipsMergedInterior|TestMergedText|TestMergedRangeRenders' -count=1`

  Expected: PASS; clicking or navigating into a merge selects its anchor, movement skips hidden cells, and wrapped output remains within the merged area.
- [ ] **Step 5: Commit**

  ```bash
  git add internal/ui/grid.go internal/ui/model.go internal/ui/grid_test.go internal/ui/model_test.go
  git commit -m "fix(ui): treat merged cells as one grid cell"
  ```

### Task 3: Save CSV sessions as XLSX through the live Excelize package

**Files:**
- Modify: `internal/workbook/xlsx.go`
- Test: `internal/workbook/xlsx_test.go`

**Interfaces:**
- Consumes: existing `SaveFile(session *Session, path string) error`, `ensureCalculationFile`, and workbook operations that synchronize the Excelize package.
- Produces: `SaveFile` accepts CSV-to-XLSX conversion and updates both `Session` and `Workbook` format/path only after a successful replacement.

- [ ] **Step 1: Write failing conversion and failure-atomicity tests**
  - Add `TestSaveFileConvertsCSVSessionToXLSX` using a CSV session with multiple sheets, a formula, custom width, fill/font/border styles, a merge, a single-color conditional rule, and a color-scale rule; save, reopen, and assert those supported features remain.
  - Add `TestSaveFileConversionInitializesUntouchedCSVSession` for cell inputs in a CSV session whose `file` is still nil.
  - Add `TestSaveFileConversionFailureLeavesCSVSessionUnchanged` using a destination beneath a nonexistent parent directory; assert format, path, workbook data, and dirty state are unchanged.
  - Keep an XLSX-to-XLSX save regression assertion to verify preservation behavior remains unchanged.
- [ ] **Step 2: Run the focused tests and confirm they fail**

  Run: `go test ./internal/workbook -run 'TestSaveFileConvertsCSVSessionToXLSX|TestSaveFileConversionInitializesUntouchedCSVSession|TestSaveFileConversionFailureLeavesCSVSessionUnchanged' -count=1`

  Expected: FAIL because `SaveFile` currently rejects mismatched session and path formats.
- [ ] **Step 3: Implement CSV-to-XLSX save in `SaveFile`**
  - Initialize the Excelize package when needed, save it through the existing sibling-temp/atomic-replacement path, and skip source-package auditing for a CSV source.
  - After successful replacement only, set the session and workbook format/path to XLSX and capture the new package snapshot. Keep CSV state intact on every earlier error.
- [ ] **Step 4: Run workbook save tests**

  Run: `go test ./internal/workbook -run 'TestSaveFile|Test.*CSV.*XLSX' -count=1`

  Expected: PASS; reopened XLSX retains supported state and failed conversion leaves the CSV session unchanged.
- [ ] **Step 5: Commit**

  ```bash
  git add internal/workbook/xlsx.go internal/workbook/xlsx_test.go
  git commit -m "feat(workbook): convert CSV sessions to XLSX"
  ```

### Task 4: Replace the inline CSV warning with a save-choice overlay

**Files:**
- Modify: `internal/app/controller.go`
- Create: `internal/app/csv_save_dialog.go`
- Test: `internal/app/controller_test.go`

**Interfaces:**
- Consumes: Task 3's CSV-to-XLSX `SaveFile` behavior; existing `csvLossWarnings`, save picker, and overlay composition.
- Produces: a centered keyboard/mouse dialog with **Save as CSV**, **Save as XLSX**, and **Cancel** actions.

- [ ] **Step 1: Write failing controller tests**
  - Add `TestCSVSaveChoiceOpensAsOverlay` and assert the warning is centered over the workbook, not appended below the grid.
  - Add `TestCSVSaveChoiceKeyboardActions` for CSV, XLSX, and Escape cancellation.
  - Add `TestCSVSaveChoiceMouseActions` for all three mouse targets.
  - Add `TestCSVSaveChoiceCSVExportsActiveSheetAndKeepsSession` with multiple sheets/styles; inspect output and assert the live model retains those features.
  - Add `TestCSVSaveChoiceXLSXPickerUsesXLSXName` and assert the selected save path reaches Task 3's conversion path without reopening the CSV warning.
  - Update `TestSaveAsPickerKeepsCSVLossWarning` to assert the new choice dialog, not the removed confirmation flag.
- [ ] **Step 2: Run the focused controller tests and confirm they fail**

  Run: `go test ./internal/app -run 'TestCSVSaveChoice' -count=1`

  Expected: FAIL because the controller currently renders a one-line warning and only supports Enter/Y confirmation.
- [ ] **Step 3: Implement the dialog and controller flow**
  - Replace `confirmCSVLoss` key-only confirmation with a focusable overlay; route keyboard and mouse actions to the same choices.
  - Save CSV to the current path, or open a CSV save picker for an untitled workbook. Export only the active sheet and leave the in-memory workbook intact.
  - Save XLSX through a picker with a same-basename `.xlsx` name. Cancel or failure leaves the CSV session unchanged; success uses Task 3's conversion.
  - Remove obsolete inline-confirmation text and handlers.
- [ ] **Step 4: Run controller tests**

  Run: `go test ./internal/app -run 'TestCSV|TestSaveAsPicker' -count=1`

  Expected: PASS; keyboard and mouse choices produce the specified export, conversion, and cancel behavior.
- [ ] **Step 5: Commit**

  ```bash
  git add internal/app/controller.go internal/app/csv_save_dialog.go internal/app/controller_test.go
  git commit -m "feat(app): add CSV save format choices"
  ```

### Task 5: Show transient notices for CSV-incompatible features

**Files:**
- Modify: `internal/ui/model.go`, `internal/ui/operations_test.go`
- Create: `internal/ui/notice.go`

**Interfaces:**
- Consumes: existing successful mutation results and `Workbook.Format`.
- Produces: `func (model *AppModel) showCSVFeatureNotice() tea.Cmd` and `type csvNoticeExpiredMsg struct { generation uint64 }`; notices expire after 2.5 seconds and render beneath blocking overlays.

- [ ] **Step 1: Write failing notice lifecycle and trigger tests**
  - Add `TestCSVFeatureNoticeAppearsAfterSuccessfulMutation` for sheet addition, resize, direct formatting, merge, and conditional-format completion.
  - Add `TestCSVFeatureNoticeDoesNotAppearForFailedMutationOrXLSXSession`.
  - Add `TestCSVFeatureNoticeExpiryDoesNotDismissNewerNotice` by delivering an older generation's timer message after a newer notice.
  - Assert the notice is theme-aware and does not obscure a dialog overlay.
- [ ] **Step 2: Run the focused tests and confirm they fail**

  Run: `go test ./internal/ui -run 'TestCSVFeatureNotice' -count=1`

  Expected: FAIL because no transient notice state or expiry handling exists.
- [ ] **Step 3: Implement notice state, rendering, and triggers**
  - Add a small corner notice that expires after 2.5 seconds; use a generation token so stale timer messages cannot clear a later notice.
  - Trigger it only after successful CSV-incompatible changes: sheet addition, column-width change, direct formatting/borders, merge, and conditional rule application.
  - Render it beneath dialogs and palettes so it cannot replace or obscure them.
- [ ] **Step 4: Run notice tests**

  Run: `go test ./internal/ui -run 'TestCSVFeatureNotice|TestConditional' -count=1`

  Expected: PASS; only successful CSV feature mutations notify, and timer ordering is safe.
- [ ] **Step 5: Commit**

  ```bash
  git add internal/ui/model.go internal/ui/notice.go internal/ui/operations_test.go
  git commit -m "feat(ui): notify when CSV cannot preserve features"
  ```

### Task 6: Improve the launcher and responsive toolbar

**Files:**
- Modify: `internal/ui/home.go`, `internal/ui/toolbar.go`, `internal/ui/model.go`
- Test: `internal/ui/model_test.go`, `internal/ui/command_palette_test.go`

**Interfaces:**
- Consumes: existing command IDs `conditional-format`, `insert-function`, and command-palette behavior.
- Produces: `func layoutToolbar(width int) []toolbarItemPlacement`, shared by rendering and hit-testing. Define `toolbarItemPlacement` with `action string`, `label string`, `start int`, and `end int`; `start` is inclusive and `end` exclusive in terminal-column offsets.

- [ ] **Step 1: Write failing launcher and toolbar tests**
  - Add `TestWelcomeWordmarkIsProminentAndCompact` for a normal viewport and a 30x10 viewport; assert the wordmark and actions remain visible without overflow.
  - Add `TestToolbarPrimaryActionsFitAndAreClickableAt80Columns` for `Conditional` and `ƒx` placement and mouse hit-testing.
  - Add `TestToolbarOverflowRetainsCommandsAtNarrowWidths` and `TestToggleToolbarDoesNotHideColumnHeaders`.
- [ ] **Step 2: Run the focused tests and confirm they fail**

  Run: `go test ./internal/ui -run 'TestWelcomeWordmarkIsProminentAndCompact|TestToolbarPrimaryActionsFitAndAreClickableAt80Columns|TestToolbarOverflowRetainsCommandsAtNarrowWidths|TestToggleToolbarDoesNotHideColumnHeaders' -count=1`

  Expected: FAIL because the wordmark is one small line and toolbar rendering currently clips its fixed-width item list.
- [ ] **Step 3: Implement the wordmark and shared toolbar layout**
  - Render a larger centered SHTUI wordmark at normal sizes and a compact form on small screens.
  - Add Conditional and `ƒx` items beside Format, routed to the existing command IDs.
  - Keep one toolbar row; move secondary actions into a `More` entry that opens the command palette when space is constrained. Compute rendering and hit targets from `layoutToolbar`.
  - Keep the toolbar toggle independent from the always-visible column labels.
- [ ] **Step 4: Run launcher and toolbar tests**

  Run: `go test ./internal/ui -run 'TestWelcomeWordmark|TestToolbar|TestToggleToolbar' -count=1`

  Expected: PASS at normal and narrow widths; visible mouse targets match rendered positions and every action remains reachable.
- [ ] **Step 5: Commit**

  ```bash
  git add internal/ui/home.go internal/ui/toolbar.go internal/ui/model.go internal/ui/model_test.go internal/ui/command_palette_test.go
  git commit -m "feat(ui): improve launcher and toolbar"
  ```

### Task 7: Make conditional formatting match the approved layout and keyboard flow

**Files:**
- Modify: `internal/ui/conditional_dialog.go`, `internal/ui/model.go`
- Test: `internal/ui/operations_test.go`

**Interfaces:**
- Consumes: existing conditional-dialog model, color picker, overlay, and toolbar command.
- Produces: visibly focused controls with reversible keyboard traversal and keyboard-accessible color/rule actions.

- [ ] **Step 1: Write failing conditional-dialog keyboard and layout tests**
  - Add `TestConditionalDialogTabAndShiftTabVisitEveryControl` for single-color and color-scale modes, including range, rule, value, colors, add-rule, Done, and Cancel.
  - Add `TestConditionalDialogKeyboardOpensColorPickerAndCompletesRule`.
  - Add `TestConditionalDialogShowsVisibleFocusAndGroupedSections` at 80x24 and retain the existing 30x10 fit test; remove assertions that pin obsolete compact wording or unhighlighted controls.
- [ ] **Step 2: Run the focused tests and confirm they fail**

  Run: `go test ./internal/ui -run 'TestConditionalDialog(TabAndShiftTab|Keyboard|ShowsVisibleFocus)' -count=1`

  Expected: FAIL because reverse traversal and visible focus for all controls are not implemented.
- [ ] **Step 3: Implement the spaced dialog layout and keyboard actions**
  - Group range, rule, formatting-style, preview, and rule-list sections with the terminal theme.
  - Render clear focus on every keyboard target; support Tab/Shift+Tab, arrows for focused choices, Enter activation, and Esc cancellation.
  - Ensure color controls open the existing picker from the keyboard and mouse.
- [ ] **Step 4: Run conditional-dialog tests**

  Run: `go test ./internal/ui -run 'TestConditionalDialog|TestConditional' -count=1`

  Expected: PASS for keyboard, mouse, standard viewport, and compact viewport behavior.
- [ ] **Step 5: Commit**

  ```bash
  git add internal/ui/conditional_dialog.go internal/ui/model.go internal/ui/operations_test.go
  git commit -m "feat(ui): improve conditional formatting dialog"
  ```

### Task 8: Update usage guidance and verify the complete app

**Files:**
- Modify: `docs/usage.md`

**Interfaces:**
- Consumes: the completed behaviors from Tasks 1–7.
- Produces: current user guidance for CSV warnings, CSV/XLSX save choices, merged-cell editing/wrapping, toolbar actions, and conditional-format keyboard use.

- [ ] **Step 1: Run the full test suite**

  Run: `go test ./...`

  Expected: PASS.
- [ ] **Step 2: Build a smoke binary without overwriting workspace files**

  Run: `go build -o /tmp/shtui-grid-save-smoke ./cmd/shtui`

  Expected: successful native Go build.
- [ ] **Step 3: Smoke the actual TUI in a PTY**
  - Start `/tmp/shtui-grid-save-smoke`, create a CSV workbook, enter data, and apply fill, border, merge, conditional formatting, a new sheet, and a column resize.
  - Confirm the CSV notice appears and expires; click and keyboard-navigate toolbar actions and merged cells.
  - Exercise Cancel, Save as CSV, and Save as XLSX; reopen the XLSX and verify the workbook features remain.
  - Expected: all actions are visible and usable, no fill bleed appears, merged text stays in bounds, CSV export is explicitly lossy, and XLSX retains supported state.
- [ ] **Step 4: Update `docs/usage.md` after the smoke proof** with the save-choice semantics, supported CSV limitations, merged-cell behavior, and toolbar/conditional-format controls.
- [ ] **Step 5: Commit usage guidance**

  ```bash
  git add docs/usage.md
  git commit -m "docs: explain CSV and merged-cell workflows"
  ```
