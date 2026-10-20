# Shtui Quit Confirmation and Exit Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a mouse- and keyboard-operable Ctrl+C confirmation and print a truthful, branded session summary after the TUI restores the terminal.

**Architecture:** Keep quit ownership in the root app controller. Reuse the rounded overlay and input-routing conventions from the CSV save dialog; on confirmed exit, print the summary only after `tea.Program.Run` returns and the alternate screen is restored.

**Tech Stack:** Go, Bubble Tea, Lip Gloss. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-19-shtui-borders-exit-formulas-design.md`

## Global Constraints

- Keep Go, Bubble Tea/Bubbles, Lip Gloss, and the current workbook/Excelize layers; no new dependency is proposed.
- Use a topmost modal owner for quit confirmation so keyboard and pointer input cannot leak into the grid while it is open. The controller owns process termination and the post-TUI summary, after Bubble Tea restores the alternate screen.
- Track the latest successful user-visible action at the controller/workbook boundary, including destination path and whether subsequent edits are unsaved.
- Do not infer save success from a requested save or picker selection; update the summary only after the file write succeeds.
- The quit flow does not auto-save. A second Ctrl+C is the explicit immediate-quit bypass.

## Review Focus

1. The first Ctrl+C defaults to **Nope**, while a second Ctrl+C confirms immediately. Assert initial selection, first/second key behavior, and resulting quit commands in `TestQuitConfirmationKeyboardAndDoubleCtrlC`.
2. Mouse clicks use the visible action geometry and never leak to the workbook; assert Yep, Nope, and outside-click behavior in `TestQuitConfirmationMouseActions`.
3. Dirty workbooks show a loss warning while clean workbooks do not; assert both rendered states in `TestQuitConfirmationDirtyWarning`.
4. Canceled or failed saves are never described as saved; assert last-action summary remains truthful in `TestExitSummaryIgnoresFailedOrCanceledSave`.
5. A new unsaved workbook and a sheet added after a save report the correct workbook path and unsaved state; assert both in `TestExitSummaryNewAndAddedSheet`.

---

### Task 1: Add the centered, keyboard/mouse quit dialog

**Files:**
- Create: `internal/app/quit_dialog.go`
- Modify: `internal/app/controller.go`, `internal/app/controller_test.go`, `internal/ui/model.go`

**Interfaces:**
- Consumes: `csvSaveDialog`'s Lip Gloss layout and overlay hit-testing conventions, `Controller.View`, and `Controller.Update`.
- Produces: internal `quitDialog` state with `open`, `selected`, `View(width, height, dirty)`, `handleKey(tea.KeyMsg) quitAction`, and `handleMouse(tea.MouseMsg, width, height) quitAction`; controller state records whether the user confirmed quit.

- [x] **Step 1: Write failing dialog/controller tests**
  - Add `TestQuitConfirmationKeyboardAndDoubleCtrlC`: the first Ctrl+C opens the prompt with Nope selected; arrows/Tab and Enter can select Yep/Nope; Y confirms; N/Esc cancel; a second Ctrl+C quits immediately.
  - Add `TestQuitConfirmationMouseActions`: click each rendered action and assert the same confirm/cancel result; a click outside the button rows must not quit or reach the underlying workbook.
  - Add `TestQuitConfirmationDirtyWarning`: assert the dialog includes an unsaved-loss warning iff `session.Workbook.Dirty` is true.
- [x] **Step 2: Run focused tests and confirm they fail**

  Run: `go test ./internal/app -run 'TestQuitConfirmation' -count=1`

  Expected: FAIL because Controller currently returns `tea.Quit` on the first Ctrl+C.
- [x] **Step 3: Implement quit dialog state and event routing**
  - Add `quitDialog` as a centered rounded overlay, default selection Nope, and shared keyboard/mouse actions.
  - Route the first Ctrl+C to opening the dialog; while open, route all key and pointer input to it. A second Ctrl+C sets the confirmed-quit state and returns `tea.Quit` immediately. Yep confirms; Nope/Esc cancels and restores the unchanged view.
  - Remove the direct first-Ctrl+C quit path from both `AppModel.Update` and the root controller so no app entry path bypasses confirmation; do not add auto-save.
- [x] **Step 4: Run focused dialog tests**

  Run: `go test ./internal/app -run 'TestQuitConfirmation' -count=1`

  Expected: PASS; default focus is Nope, keyboard and mouse agree, dirty warning is accurate, and the double-Ctrl+C bypass is immediate.
- [x] **Step 5: Commit** — `e28fec8` (`feat(app): confirm quit with keyboard and mouse`)

  ```bash
  git add internal/app/quit_dialog.go internal/app/controller.go internal/app/controller_test.go internal/ui/model.go docs/superpowers/plans/2026-10-19-shtui-quit-summary.md
  git commit -m "feat(app): confirm quit with keyboard and mouse"
  ```

### Task 2: Print a branded, truthful summary after confirmed quit

**Files:**
- Modify: `internal/ui/model.go`, `internal/app/controller.go`, `internal/app/controller_test.go`, `docs/usage.md`

**Interfaces:**
- Consumes: Task 1's confirmed-quit state, `saveCompleteMsg`, `newWorkbook`, successful UI mutation paths, `tea.Program.Run`, and `Workbook.Dirty`.
- Produces: `AppModel.TakeActionSummary() string` for successful workbook actions, `Controller.exitSummary() string`, and post-Run output gated on confirmed quit.

- [x] **Step 1: Write failing action-summary tests**
  - Add `TestExitSummaryNewAndAddedSheet`: a new unsaved CSV workbook reports that it is not saved; adding Sheet2 to a saved path reports the path and `(unsaved)`.
  - Add `TestExitSummarySaveAndLaterEdits`: successful save reports the destination; later edits mark the summary unsaved rather than implying they were saved.
  - Add `TestExitSummaryIgnoresFailedOrCanceledSave`: a failed save or canceled save choice never replaces a prior successful action with a false Saved message.
  - Add `TestRunPrintsSummaryOnlyAfterConfirmedQuit` around a testable post-Run formatter/print helper; assert no summary for context cancellation and summary text after confirmation.
- [x] **Step 2: Run focused tests and confirm they fail**

  Run: `go test ./internal/app -run 'TestExitSummary|TestRunPrintsSummary' -count=1`

  Expected: FAIL because Run currently discards the returned model and prints no summary.
- [x] **Step 3: Track successful actions and print after terminal restoration**
  - Have the UI expose successful action summaries for edits and workbook commands; drain them in Controller after UI updates. Controller records successful open/create/save events and only records Saved after `saveCompleteMsg.err == nil`.
  - Derive the unsaved suffix from `Workbook.Dirty`; include the workbook path when known and identify untitled workbooks as not saved.
  - Retain the `Controller` passed to `tea.NewProgram`; after `program.Run()` returns successfully, print a compact Shtui wordmark and `exitSummary()` only if quit was confirmed. Keep errors/context cancellation from being reported as a user-confirmed quit.
  - Update `docs/usage.md` with the first-Ctrl+C, second-Ctrl+C, mouse, and post-exit summary behavior.
- [x] **Step 4: Run focused app tests and native TUI smoke**

  Run: `go test ./internal/app -run 'TestQuitConfirmation|TestExitSummary|TestRunPrintsSummary' -count=1`

  Expected: PASS; output is gated on confirmed quit and accurately distinguishes saved, unsaved, and canceled/failed operations. Then launch the native app in a PTY, test Nope by mouse and keyboard, Yep by mouse and keyboard, double Ctrl+C, and observe that the logo/summary appears after the alternate screen exits.
- [x] **Step 5: Commit**

  ```bash
  git add internal/ui/model.go internal/app/controller.go internal/app/controller_test.go docs/usage.md docs/superpowers/plans/2026-10-19-shtui-quit-summary.md
  git commit -m "feat(app): print workbook session summary on exit"
  ```
