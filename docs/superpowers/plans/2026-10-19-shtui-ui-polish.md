# Shtui UI Interaction Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make sheet tabs, color controls, and conditional formatting mouse-operable; remove the visual gap between cell fills and borders; and ship the approved Shtui welcome screen and user-facing name.

**Architecture:** Keep the virtualized grid and controller/workbook boundaries. Derive pointer hit regions from the same layout used to render tabs and overlays; make `AppModel` the single home-screen owner and continue routing actions through the existing controller. Keep conditional changes as drafts until Done.

**Tech Stack:** Go 1.26.0, Bubble Tea v1.3.10, Bubbles v0.21.0, Lip Gloss v1.1.0, Excelize v2.11.0.

**Spec:** `docs/superpowers/specs/2026-10-19-shtui-ui-polish-design.md`

## Global Constraints

- Keep Go as the application language and preserve native compiled startup.
- Retain Bubble Tea/Bubbles, Lip Gloss, the custom virtualized grid, and Excelize workbook operations; add no dependencies.
- Preserve supported keyboard workflows, terminal-theme colors, async open/save, CSV-loss warnings, and unknown XLSX feature round-tripping.
- Keep the internal Go module and source paths (`shtui`) unchanged. User-facing brand and executable name become `Shtui` and `shtui`.
- No recent-file persistence or new spreadsheet functions are in scope.

## Review Focus

1. **Small terminals (especially 30×10):** keep conditional range/rule inputs and Cancel/Done reachable; pin in Task 3.
2. **Color-picker clicks while nested in conditional formatting:** act only on the topmost popup; outside click must not reach the dialog/grid; pin in Tasks 2–3.
3. **Toolbar hidden or viewport resized:** tabs and `+` must still hit at their rendered row and width; pin in Task 1.
4. **Adjacent differently filled cells and merged cells:** preserve border strokes, deterministic shared-boundary fill, and suppressed merge interiors; pin in Task 4.
5. **No-workbook start and mouse selection:** actions must fit the viewport and route through existing async/controller behavior; pin in Task 5.

---

### Task 1: Make sheet tabs and Add Sheet mouse targets match the rendered row

**Files:**
- Modify: `internal/ui/grid.go`
- Modify: `internal/ui/model.go`
- Test: `internal/ui/model_test.go`

**Interfaces:**
- `GridViewport.renderedHeight(height int) int` returns the physical line count emitted by `GridViewport.View` for the given height, accounting for the header and visible filtered rows.
- A private sheet-tab layout result supplies the tab text, x-ranges, and `+` hit target to both `sheetTabs` rendering and mouse handling. `AppModel.sheetTabsY() int` derives y from the grid origin and rendered height.

- [ ] **Step 1: Write failing mouse tests.** Add `TestSheetTabClickUsesRenderedRow` for toolbar visible/hidden at normal and compact sizes; locate the visible tab row in `model.View()`, click the second tab through `AppModel.Update`, and assert active sheet plus selection/scroll reset. Add `TestAddSheetPlusClickCreatesAndActivatesSheet`; click the rendered `+` hit area and assert the new sheet is active.
- [ ] **Step 2: Run focused tests and confirm the current hit test misses.** Run `go test ./internal/ui -run 'Test(SheetTabClickUsesRenderedRow|AddSheetPlusClickCreatesAndActivatesSheet)$' -count=1`; expect failure for at least the toolbar-hidden geometry and missing `+` target.
- [ ] **Step 3: Share tab geometry between rendering and hit testing.** Add the rendered grid-height helper, derive the tab row rather than comparing to `height-2`, and generate x-ranges from the same labels and gaps used by `sheetTabs`. Render a visible `+` after the final sheet; route its click through the existing `add-sheet` command.
- [ ] **Step 4: Re-run focused tests.** Run `go test ./internal/ui -run 'Test(SheetTabClickUsesRenderedRow|AddSheetPlusClickCreatesAndActivatesSheet|SheetNavigation)' -count=1`; expect all pass with toolbar on and off.
- [ ] **Step 5: Commit.** `git add internal/ui/grid.go internal/ui/model.go internal/ui/model_test.go && git commit -m "fix(ui): make sheet tabs clickable"`.

### Task 2: Route mouse clicks through the shared color picker overlay

**Files:**
- Modify: `internal/ui/overlay.go`, `internal/ui/overlay_test.go`, `internal/ui/model.go`
- Create: `internal/ui/color_picker.go`
- Test: `internal/ui/theme_test.go`, `internal/ui/operations_test.go`

**Interfaces:**
- `overlayRect` contains `X`, `Y`, `Width`, and `Height` in terminal cells.
- `overlayBounds(popup string, width, height int) overlayRect` computes the same clipped centered geometry used by `ComposeOverlay`; `ComposeOverlay` uses this helper.
- `AppModel.handleColorPickerMouse(message tea.MouseMsg)` owns pointer events whenever the color picker is open and never forwards them to the underlying dialog/grid.

- [ ] **Step 1: Write failing geometry and picker tests.** Add `TestOverlayBoundsMatchesComposePlacement`, `TestColorPickerMouseSelectsThemeOption`, `TestColorPickerMouseOpensCustomRGB`, and `TestColorPickerOutsideClickDismissesWithoutApplying`. Drive clicks through `AppModel.Update`; assert the selected fill/font value or unchanged workbook state.
- [ ] **Step 2: Run focused tests and confirm the current modal early return ignores clicks.** Run `go test ./internal/ui -run 'Test(OverlayBounds|ColorPickerMouse)' -count=1`; expect picker clicks to leave the popup open and style unchanged.
- [ ] **Step 3: Implement shared overlay bounds and picker hit testing.** Make `ComposeOverlay` and mouse handling use `overlayBounds`; route picker mouse events before other modal/grid handlers. Clicking a theme/custom option follows the existing Enter action; clicking custom RGB opens the existing text-entry flow; outside click closes only the picker without applying a color. Keep the theme-first ordering and resolve terminal colors only on selection.
- [ ] **Step 4: Run focused and related tests.** Run `go test ./internal/ui -run 'Test(Overlay|ColorPicker|ThemePalette|ConditionalDialog)' -count=1`; expect pass, including keyboard selection and nested conditional-picker behavior.
- [ ] **Step 5: Commit.** `git add internal/ui/overlay.go internal/ui/overlay_test.go internal/ui/model.go internal/ui/color_picker.go internal/ui/theme_test.go internal/ui/operations_test.go && git commit -m "fix(ui): support mouse color selection"`.

### Task 3: Reorganize conditional formatting and support mouse controls

**Files:**
- Modify: `internal/ui/conditional_dialog.go`, `internal/ui/model.go`, `internal/ui/operations_test.go`
- Reuse: `overlayRect` from Task 2 and the existing workbook conditional-rule APIs.

**Interfaces:**
- `conditionalDialogLayout` contains the rendered popup plus control hit regions generated from that same layout.
- `conditionalDialogState.layout(width, height int) conditionalDialogLayout` is used by both `View` and `HandleMouse`, keeping hit targets aligned with the visible controls.
- `conditionalDialogState.HandleMouse(message tea.MouseMsg, width, height int) string` returns the same action values (`color`, `done`, `cancel`, or empty) used by keyboard handling. It does not apply workbook rules itself.

- [ ] **Step 1: Write failing dialog mouse and layout tests.** Add tests for clicking Single color/Color scale, opening and selecting a condition dropdown option, focusing range/value inputs, opening Fill/Font/Min/Mid/Max color controls, toggling 2/3-color scales, Add another rule, Done, and Cancel. Assert clicked controls receive focus/action, Cancel leaves workbook rules unchanged, Done applies every visible draft once, and `TestConditionalDialogShowsAddedRuleSummaries` displays previously added drafts. Extend the small-viewport case to assert all required inputs and actions remain visible.
- [ ] **Step 2: Run focused tests and confirm modal clicks are ignored.** Run `go test ./internal/ui -run 'TestConditionalDialog(Mouse|Shows|Fits)' -count=1`; expect mouse tests to fail before the new handler and grouped layout exist.
- [ ] **Step 3: Build the approved range → rule → style dialog.** Generate the view and hit regions from one layout method. Add clear section labels, spacing, visible rule-draft summaries, a condition dropdown with the supported operators, style controls and preview, and fixed Add/Cancel/Done actions. Keep the compact layout for short viewports.
- [ ] **Step 4: Route dialog clicks and preserve modal priority.** In `AppModel.handleMouse`, route an open color picker first; otherwise route an open conditional dialog and translate returned actions through the existing dialog action handler. Clicks in fields focus/update the Bubbles input; clicks on tabs, dropdown options, color controls, scale toggles, Add, Cancel, and Done perform their corresponding actions. Clicking outside the conditional dialog does not mutate the workbook.
- [ ] **Step 5: Run focused and related tests.** Run `go test ./internal/ui -run 'TestConditionalDialog|TestConditionalRule|TestColorPickerMouse' -count=1`; expect all dialog, nested palette, keyboard, cancellation, and validation tests to pass.
- [ ] **Step 6: Commit.** `git add internal/ui/conditional_dialog.go internal/ui/model.go internal/ui/operations_test.go && git commit -m "feat(ui): clarify conditional formatting dialog"`.

### Task 4: Extend cell fills to grid boundaries

**Files:**
- Modify: `internal/ui/grid.go`
- Test: `internal/ui/grid_test.go`

**Interfaces:**
- Boundary rendering receives the already-created `workbook.EffectiveStyleResolver` so it can apply fills without recalculating styles per edge.
- `verticalBoundary` and `horizontalBoundary` preserve existing border glyph and foreground selection while adding the deterministic shared-boundary background from the cell to the left/above; outer edges use the adjacent cell's fill.

- [ ] **Step 1: Write failing render tests.** Add `TestGridFillTouchesCellBoundaries` for a single filled cell, `TestGridSharedBoundaryUsesLeftAndUpperFill` for adjacent different fills, and `TestFilledMergedCellKeepsOuterBorders`. Assert cell-background styling reaches vertical/horizontal border glyphs, explicit border foreground remains intact, and merged interiors remain suppressed.
- [ ] **Step 2: Run focused tests and confirm boundary backgrounds are absent.** Run `go test ./internal/ui -run 'Test(GridFill|GridSharedBoundary|FilledMergedCell)' -count=1`; expect the current separately rendered, unstyled boundary glyphs to fail the style assertions.
- [ ] **Step 3: Apply fill background to boundary glyph cells.** Use the resolver created once in `GridViewport.View`; keep cell dimensions and glyph count unchanged. Style boundary backgrounds from the adjacent effective cell fill, including merged spans, while retaining border foreground/preset and selection behavior.
- [ ] **Step 4: Run grid and border tests.** Run `go test ./internal/ui -run 'Test(Grid|Border|MergedRange)' -count=1`; expect all render and existing merge/border tests to pass.
- [ ] **Step 5: Commit.** `git add internal/ui/grid.go internal/ui/grid_test.go && git commit -m "fix(ui): extend cell fills to borders"`.

### Task 5: Build the centered Shtui launcher and rename user-facing identity

**Files:**
- Create: `internal/ui/home.go`
- Modify: `internal/ui/model.go`, `internal/app/controller.go`, `internal/app/controller_test.go`, `cmd/shtui/main.go`, `docs/usage.md`
- Test: `internal/ui/model_test.go`

**Interfaces:**
- `homeScreen` owns a Bubbles `list.Model` with stable actions `open-file`, `new-csv`, and `new-xlsx`. Its exact methods are `View(width, height int) string`, `HandleKey(message tea.KeyMsg) string`, and `HandleMouse(message tea.MouseMsg) string`.
- `AppModel` owns `homeScreen` when no workbook is loaded and emits the selected action through its existing `TakeCommand()` handoff.
- `Controller.Update` consumes `TakeCommand()` after home mouse updates and routes it through the existing `handleAction`; `Controller.View` delegates the no-workbook view to `AppModel` while retaining the loading state.

- [ ] **Step 1: Write failing home-screen tests.** Add `TestWelcomeScreenUsesShtuiBrandAndFitsViewport`, `TestWelcomeScreenMouseSelectsWorkbookAction`, `TestWelcomeMouseCreatesCSVWorkbook`, and `TestWelcomeMouseStartsOpenPicker`. Assert centered actions, keyboard hints, dimensions at normal and 30×10 sizes, and one-click routing into the existing controller actions.
- [ ] **Step 2: Run focused tests and confirm the current text-only/duplicate screens fail.** Run `go test ./internal/ui ./internal/app -run 'Test(WelcomeScreen|NoPathScreen|NoPathShortcut)' -count=1`; expect missing Shtui branding, non-interactive actions, or clipping.
- [ ] **Step 3: Implement the centered launcher in `internal/ui/home.go`.** Use a small Bubbles list with Open/New CSV/New XLSX actions, the approved centered wordmark/tagline, terminal-theme styling, and visible shortcut hints. Ensure click activation is a single click and the layout clamps to short/narrow terminals.
- [ ] **Step 4: Make `AppModel` the single home-screen owner.** Route home keyboard/mouse selection through `runCommand`/`TakeCommand`; have `Controller.View` delegate to `view.View()` instead of rendering a second plain-text screen. In `Controller.Update`, consume home actions after mouse updates and preserve existing open/create controller paths and keyboard shortcuts.
- [ ] **Step 5: Apply the Shtui user-facing name.** Change CLI usage to `shtui`, update `docs/usage.md` build/run examples to `go build -o shtui ./cmd/shtui` and `./shtui`, and update the welcome/loading labels. Keep Go module, imports, and source directory unchanged.
- [ ] **Step 6: Run focused launch/controller tests.** Run `go test ./internal/ui ./internal/app -run 'Test(WelcomeScreen|WelcomeMouse|NoPathScreen|NoPathShortcut|OpenPicker)' -count=1`; expect pass with mouse and keyboard actions reaching the existing controller behavior.
- [ ] **Step 7: Commit.** `git add internal/ui/home.go internal/ui/model.go internal/ui/model_test.go internal/app/controller.go internal/app/controller_test.go cmd/shtui/main.go docs/usage.md && git commit -m "feat(ui): add Shtui welcome launcher"`.

### Task 6: Full verification, real terminal smoke, and final review

**Files:**
- Verify: all files changed by Tasks 1–5; do not inspect or stage pre-existing user changes.

- [ ] **Step 1: Run the full test suite.** Run `go test ./...`; expect all packages to pass.
- [ ] **Step 2: Build the native Shtui executable.** Run `go build -o /tmp/shtui-smoke ./cmd/shtui`; expect exit 0. Run `/tmp/shtui-smoke one two` and confirm usage names `shtui`.
- [ ] **Step 3: Smoke the actual TUI.** Launch `/tmp/shtui-smoke` at 80×24 and 30×10; inspect the centered Shtui launcher, create/open a workbook, inspect the sheet `+` tab and fill boundaries, open the color picker and conditional popup, and verify the popup layouts remain within bounds. Use deterministic `AppModel.Update` mouse tests for each click target.
- [ ] **Step 4: Request one fresh final code review** of the committed Go/docs diff for layout drift, modal event leakage, formatting regressions, and unsupported-feature/data preservation.
- [ ] **Step 5: Remove only `/tmp/shtui-smoke` and temporary fixtures created by the smoke run.** Preserve all pre-existing user deletions and untracked files.
