# Terminal CSV/XLSX Spreadsheet Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a fast-starting, compiled Go terminal spreadsheet for CSV and XLSX-family workbooks with mouse support and the approved 80-20 editing, formula, and formatting features.

**Architecture:** Bubble Tea renders a virtualized viewport over a workbook session model. CSV uses Go's standard library; XLSX-family files use Excelize while retaining its workbook handle and guarding save round trips. Formula calculations go through Excelize `CalcCellValue`; UI, model operations, and file adapters stay separate.

**Tech Stack:** Go 1.26.0; Bubble Tea; Lip Gloss for terminal-aware styling; Excelize; Go standard `encoding/csv`.

**Spec:** `docs/superpowers/specs/2026-10-18-csv-xlsx-terminal-spreadsheet-design.md`

## Global Constraints

- Go compiled binary; no Python or TypeScript.
- Open/save `.csv`, `.xlsx`, and `.xlsm`; never execute VBA/macros.
- CSV represents one sheet and cannot persist styles or workbook metadata; CSV export writes formula source as text.
- Formula scope: arithmetic, cell references/ranges, `SUM`, `AVERAGE`, `COUNT`, `MIN`, `MAX`, and `IF`; unsupported formulas remain intact and are marked stale/unavailable.
- Target 25,000 rows × 20 columns per sheet; virtualize rendering and measure startup and workbook operations at that target.
- Use terminal default foreground/background and ANSI theme colors; show theme colors before custom RGB/hex; map custom colors to the nearest terminal palette on non-truecolor terminals.
- Preserve unsupported workbook parts where Excelize can round-trip them; do not promise universal fidelity; refuse overwrite when detected loss is possible.
- Keep the toolbar visible by default; `Ctrl+P` opens the command palette, including when the toolbar is hidden.
- Save via a temporary sibling file and replace the original only after successful serialization.
- Stage and commit only explicit task files so unrelated pre-existing changes remain untouched.

## Review Focus

- CSV quoted cells, embedded newlines, empty trailing cells, and formula source must round-trip as data; Task 1 pins these in `TestCSVRoundTripQuotedAndFormulaCells`.
- An XLSX with a package part/relationship Excelize would drop must not overwrite the source; Task 2 pins this in `TestSaveRefusesUnsupportedPartLoss`.
- A formula depending on an edited cell must show its new result, while an unsupported formula remains marked stale; Task 3 pins this in `TestFormulaDependencyRecalculation` and `TestUnsupportedFormulaIsPreservedAndStale`.
- Merging a range with non-top-left values requires confirmation, and sorting merged ranges is rejected; Task 5 pins merge confirmation and Task 6 pins sort rejection.
- Theme colors must precede custom colors, invalid hex must be rejected, and 16-color terminals must fall back to ANSI colors; Task 7 pins these in `TestThemePaletteAndCustomColorFallback`.

---

### Task 1: Establish the Go project and workbook/CSV model

**Files:**
- Create: `go.mod`
- Create: `internal/workbook/model.go`
- Create: `internal/workbook/csv.go`
- Test: `internal/workbook/csv_test.go`
- Test: `internal/workbook/model_test.go`

**Interfaces:**
- `Format` has `CSV`, `XLSX`, and `XLSM` values. `Coord{Row, Col int}` is zero-based; `CellRange{Start, End Coord}` is inclusive.
- `BorderStyle{Preset, ColorRGB string}` and `BorderSet{Top, Right, Bottom, Left BorderStyle}`.
- `CellStyle{FillRGB, FontRGB, NumberFormat string; Bold bool; Borders BorderSet}`.
- `CalculationState` has `Literal`, `Current`, `Stale`, `Unsupported`, and `Error` values. `Cell{Input, Result string; CalculationState CalculationState; Style CellStyle}`; `Sheet{Name string; Rows, Cols int; Cells map[Coord]Cell; ColumnWidths map[int]float64}`; `Workbook{Format Format; Path string; Sheets []Sheet; ActiveSheet int; Dirty bool}`.
- `Input` is literal cell text or formula source; do not duplicate formula source in another field. Blank cells are absent from the sparse cell map.
- `ReadCSV(io.Reader, string) (*Workbook, error)` and `WriteCSV(io.Writer, *Workbook, int) error`.

- [ ] **Step 1: Create the module and failing model/CSV tests.** Initialize module `shtui` with Go 1.26.0 and add Bubble Tea, Lip Gloss, and Excelize dependencies. Add tests for a one-sheet workbook and `TestCSVRoundTripQuotedAndFormulaCells`, covering commas, quotes, embedded newlines, trailing empty cells, and a formula string such as `=SUM(A1:A2)`.
- [ ] **Step 2: Run the focused tests and confirm failure.** Run `go test ./internal/workbook -run 'Test(CSVRoundTripQuotedAndFormulaCells|WorkbookModel)' -count=1`; expected: compile/test failure because the model and CSV functions do not exist.
- [ ] **Step 3: Implement the sparse workbook model and CSV adapter.** Use Go's `encoding/csv`; preserve formula source as ordinary CSV text and create exactly one sheet.
- [ ] **Step 4: Run focused tests.** Run the same command; expected: PASS, with exact parsed fields and formula source preserved.
- [ ] **Step 5: Commit.** Stage the exact Task 1 files and run `git commit --only go.mod internal/workbook/model.go internal/workbook/csv.go internal/workbook/csv_test.go internal/workbook/model_test.go -m "feat: add workbook model and CSV I/O"`.

### Task 2: Add XLSX-family loading, saving, and preservation guard

**Files:**
- Create: `internal/workbook/session.go`
- Create: `internal/workbook/xlsx.go`
- Create: `internal/workbook/preservation.go`
- Test: `internal/workbook/xlsx_test.go`
- Test: `internal/workbook/preservation_test.go`
- Create: `internal/workbook/testdata/preservation/chart.xlsx`
- Create: `internal/workbook/testdata/preservation/pivottable.xlsx`
- Create: `internal/workbook/testdata/preservation/validation.xlsx`
- Create: `internal/workbook/testdata/preservation/macro.xlsm`
- Create: `internal/workbook/testdata/preservation/provenance.md`

**Interfaces:**
- `Session` retains the `Workbook`, source path/format, original package-part snapshot, and open Excelize `*excelize.File` for XLSX-family edits.
- `OpenFile(path string) (*Session, error)` and `SaveFile(session *Session, path string) error`.
- `SnapshotParts(path string) (map[string][32]byte, error)` and `AuditPackage(sourcePath, candidatePath string, allowedChanges map[string]struct{}) error`; audit package entries, relationships, and unsupported XML content in edited parts before replacement.

- [ ] **Step 1: Write XLSX/XLSM round-trip and loss-refusal tests.** Run `TestXLSXCellEditRoundTrip` across the chart, PivotTable, and validation fixtures; when Excelize preserves a feature, assert it remains after a cell edit and reopen. For any fixture the audit classifies unsafe, `TestSaveRefusesUnsupportedPartLoss` must assert a clear refusal and byte-identical source. `TestSaveRefusesUnknownElementLoss` covers an unsupported XML extension inside a modified sheet part. Verify `.xlsm` VBA-part retention or refusal without running macros.
- [ ] **Step 2: Run focused tests and confirm failure.** Run `go test ./internal/workbook -run 'Test(XLSXCellEditRoundTrip|SaveRefusesUnsupportedPartLoss|SaveRefusesUnknownElementLoss)' -count=1`; expected: missing adapter/audit behavior.
- [ ] **Step 3: Implement the Excelize-backed session and safe save path.** Open existing files without rebuilding them from the sparse model. Save to a temporary sibling; compare the source snapshot with the saved package and audit unsupported XML; refuse replacement on detected loss; rename only after the audit succeeds.
- [ ] **Step 4: Run focused tests.** Expected: supported fixture edits survive reopen; detected unsupported-part loss returns an error and original bytes remain unchanged.
- [ ] **Step 5: Commit.** Stage the exact Task 2 files and fixtures; run `git commit --only internal/workbook/session.go internal/workbook/xlsx.go internal/workbook/preservation.go internal/workbook/xlsx_test.go internal/workbook/preservation_test.go internal/workbook/testdata/preservation/chart.xlsx internal/workbook/testdata/preservation/pivottable.xlsx internal/workbook/testdata/preservation/validation.xlsx internal/workbook/testdata/preservation/macro.xlsm internal/workbook/testdata/preservation/provenance.md -m "feat: add guarded XLSX workbook I/O"`.

### Task 3: Implement the common formula contract

**Files:**
- Create: `internal/workbook/formula.go`
- Test: `internal/workbook/formula_test.go`

**Interfaces:**
- `CellAddress{Sheet string, Row, Col int}`.
- `Recalculate(session *Session, changed CellAddress) error` updates formula results and calculation states using Excelize `CalcCellValue`.
- CSV sessions use an in-memory Excelize workbook for calculation; CSV persistence still writes formula source, not cached results.

- [ ] **Step 1: Add failing formula tests.** `TestCommonFormulaFunctions` covers arithmetic, same- and cross-sheet references/ranges, `SUM`, `AVERAGE`, `COUNT`, `MIN`, `MAX`, and `IF`. `TestFormulaDependencyRecalculation` changes a precedent and asserts dependent formula results update. `TestUnsupportedFormulaIsPreservedAndStale` verifies source remains unchanged and status is stale. `TestFormulaErrorIsDistinctFromLiteral` separates calculation errors from cell text.
- [ ] **Step 2: Run formula tests and confirm failure.** Run `go test ./internal/workbook -run 'Test(CommonFormulaFunctions|FormulaDependencyRecalculation|UnsupportedFormulaIsPreservedAndStale|FormulaErrorIsDistinctFromLiteral)' -count=1`; expected: missing evaluator behavior.
- [ ] **Step 3: Implement the Excelize-backed evaluator.** After writes, call `CalcCellValue` for formula cells so dependencies resolve through Excelize; retain unsupported formula text/results and mark them stale rather than displaying cached values as current.
- [ ] **Step 4: Run formula tests.** Expected: all listed functions return the asserted results; edited precedents update dependents; unsupported/error states remain explicit.
- [ ] **Step 5: Commit.** `git add internal/workbook/formula.go internal/workbook/formula_test.go && git commit --only internal/workbook/formula.go internal/workbook/formula_test.go -m "feat: evaluate common spreadsheet formulas"`.

### Task 4: Add editing operations, clipboard ranges, and undo/redo

**Files:**
- Create: `internal/workbook/operations.go`
- Create: `internal/workbook/history.go`
- Create: `internal/workbook/clipboard.go`
- Test: `internal/workbook/operations_test.go`
- Test: `internal/workbook/history_test.go`

**Interfaces:**
- `SetCell(session *Session, address CellAddress, input string) error`.
- `CopyRange(session *Session, sheet string, range CellRange) ([][]string, error)` and `PasteRange(session *Session, address CellAddress, rows [][]string) error`.
- `Undo(session *Session) error` and `Redo(session *Session) error`.
- `InsertRows(session *Session, sheet string, at, count int) error` and `InsertColumns(session *Session, sheet string, at, count int) error` use zero-based insertion positions and positive counts.
- `AddSheet(session *Session, name string) error`, `RenameSheet(session *Session, oldName, newName string) error`, and `DeleteSheet(session *Session, name string) error`; duplicate names and deleting the final sheet return errors.

- [ ] **Step 1: Add failing operation tests.** Verify edits dirty the workbook, rectangular TSV paste preserves cell alignment, undo/redo restores prior values, inserted rows/columns shift cells and formula references without losing them, sheet add/rename/delete maintains a valid active-sheet index, duplicate sheet names fail, and deleting the last sheet is rejected.
- [ ] **Step 2: Run the focused tests and confirm failure.** Run `go test ./internal/workbook -run 'Test(Edit|Paste|Undo|Redo|Insert|Sheet)' -count=1`; expected: missing operations/history.
- [ ] **Step 3: Implement operations and bounded history.** Keep the most recent 100 reversible operations; record cell/range edits, paste, and structural sheet/row/column changes. `CopyRange` returns raw input so formula source is preserved; paste accepts TSV without requiring an external clipboard command.
- [ ] **Step 4: Run focused tests.** Expected: state transitions and undo/redo assertions pass, including multi-cell boundaries.
- [ ] **Step 5: Commit.** `git add internal/workbook/operations.go internal/workbook/history.go internal/workbook/clipboard.go internal/workbook/operations_test.go internal/workbook/history_test.go && git commit --only internal/workbook/operations.go internal/workbook/history.go internal/workbook/clipboard.go internal/workbook/operations_test.go internal/workbook/history_test.go -m "feat: add workbook editing and undo"`.

### Task 5: Add cell styles, borders, merges, and conditional formatting

**Files:**
- Modify: `internal/workbook/model.go`
- Create: `internal/workbook/styles.go`
- Create: `internal/workbook/conditional.go`
- Test: `internal/workbook/styles_test.go`
- Test: `internal/workbook/conditional_test.go`

**Interfaces:**
- `CellStyle` supports fill RGB, font RGB, bold, number/date format, and `BorderSet` presets.
- `ApplyStyle(session *Session, sheet string, range CellRange, style CellStyle) error` and `SetColumnWidth(session *Session, sheet string, col int, width float64) error`.
- `MergeResult` is `Merged` or `ConfirmationRequired`; `MergeRange(session *Session, sheet string, range CellRange, confirmDiscard bool) (MergeResult, error)` returns `ConfirmationRequired` without changing the workbook when non-top-left cells are nonempty. `UnmergeRange(session *Session, sheet string, range CellRange) error` removes merge metadata.
- `RuleOperator` values are `GreaterThan`, `LessThan`, `Equal`, `Blank`, and `TextContains`; `ConditionalRule{Operator RuleOperator, Value, FillRGB, FontRGB string}`.
- `AddConditionalRule(session *Session, sheet string, range CellRange, rule ConditionalRule) error` saves greater/less/equal, blank, and text-contains rules with fill/font colors. `EffectiveStyle(sheet *Sheet, address Coord) CellStyle` combines direct and conditional formatting for display.
- Task 5 extends `Sheet` with `Merges []CellRange` and `ConditionalRules []ConditionalRule`; style, merge, and conditional-format operations are undoable.

- [ ] **Step 1: Add failing style and merge tests.** Verify styles and column widths survive XLSX save/reopen; merge preserves the top-left value; `MergeRange(..., false)` returns a confirmation-required result when another cell is nonempty; confirmed merge discards only those non-top-left values; unmerge removes merge metadata; `TestUndoStyleAndMerge` verifies undo/redo restores direct formatting and merge metadata.
- [ ] **Step 2: Add failing conditional-format tests.** Assert each supported rule matches/non-matches exact boundary values, `EffectiveStyle` displays the expected fill/font style, and saved XLSX retains the rule and colors after reopen.
- [ ] **Step 3: Run focused tests and confirm failure.** Run `go test ./internal/workbook -run 'Test(Style|ColumnWidth|Merge|Unmerge|ConditionalRule|EffectiveStyle)' -count=1`; expected: unsupported operations.
- [ ] **Step 4: Implement styles, merge safeguards, and Excelize conditional-format APIs.** Keep style editing scoped to the selected range; do not rewrite unsupported workbook styling outside edited ranges.
- [ ] **Step 5: Run focused tests.** Expected: styles/widths and rules round-trip; merge confirmation behavior matches the assertions.
- [ ] **Step 6: Commit.** `git add internal/workbook/model.go internal/workbook/styles.go internal/workbook/conditional.go internal/workbook/styles_test.go internal/workbook/conditional_test.go && git commit --only internal/workbook/model.go internal/workbook/styles.go internal/workbook/conditional.go internal/workbook/styles_test.go internal/workbook/conditional_test.go -m "feat: add spreadsheet formatting and merges"`.

### Task 6: Add sorting, filters, and find

**Files:**
- Modify: `internal/workbook/session.go`
- Create: `internal/workbook/data_ops.go`
- Test: `internal/workbook/data_ops_test.go`

**Interfaces:**
- `SortRange(session *Session, sheet string, range CellRange, sortCol int, ascending, hasHeader bool) error` uses a zero-based sort column within the range, sorts whole rows, retains the first row when `hasHeader` is true, and rejects merged ranges. Inferred ranges include contiguous neighboring data; single-column-only sort requires explicit selection.
- `Find(session *Session, sheet, query string) []CellAddress` searches the requested sheet with case-insensitive substring matching.
- `ColumnFilter{Column int, AllowedValues map[string]bool}` uses a zero-based column offset within the filter range; Task 6 adds `Filters map[string][]ColumnFilter` to `Session`. `SetFilter(session *Session, sheet string, range CellRange, filters []ColumnFilter) error` stores active filters, and `VisibleRows(sheet *Sheet, filters []ColumnFilter) []int` hides rows without mutating cell contents.
- [ ] **Step 1: Add failing data-operation tests.** `TestSortKeepsRowsAligned` sorts mixed numeric/text rows without separating columns; `TestSortRejectsMergedRange` returns an actionable error; `TestSortUndoRedo` verifies whole-row sorting can be undone/redone; `TestFilterDoesNotMutateCells` checks visible row indices and original values; `TestFindActiveSheetSubstring` checks case-insensitive matches only on the requested sheet.
- [ ] **Step 2: Run tests and confirm failure.** Run `go test ./internal/workbook -run 'Test(Sort|Filter|Find)' -count=1`; expected: missing behavior.
- [ ] **Step 3: Implement stable range sorting, filters, and find.** Infer a contiguous range by adjacent nonempty rows/columns; require an explicit header choice in the sort dialog; compare numeric cells numerically and text case-insensitively with stable ties; make sorting undoable; persist XLSX filter metadata through Excelize and keep CSV filters session-only.
- [ ] **Step 4: Run focused tests.** Expected: row associations are preserved, merged ranges are rejected, filters leave source data unchanged, and find returns only active-sheet matches.
- [ ] **Step 5: Commit.** `git add internal/workbook/session.go internal/workbook/data_ops.go internal/workbook/data_ops_test.go && git commit --only internal/workbook/session.go internal/workbook/data_ops.go internal/workbook/data_ops_test.go -m "feat: add sorting filters and find"`.

### Task 7: Build the themed, mouse-capable TUI

**Files:**
- Create: `internal/ui/model.go`
- Create: `internal/ui/grid.go`
- Create: `internal/ui/toolbar.go`
- Create: `internal/ui/command_palette.go`
- Create: `internal/ui/theme.go`
- Test: `internal/ui/grid_test.go`
- Test: `internal/ui/command_palette_test.go`
- Test: `internal/ui/theme_test.go`

**Interfaces:**
- `NewModel(session *workbook.Session) AppModel` accepts `nil` to show the no-path open/create screen; `Update(tea.Msg) (tea.Model, tea.Cmd)` and `View() string`.
- `GridViewport` maps visible terminal coordinates to sheet cells, tracks scroll/selection, and renders only visible rows/columns.
- `toolbarVisible` defaults true; `Ctrl+P` opens a searchable command palette in either toolbar state; View and palette commands toggle toolbar visibility.
- `RGB{R, G, B uint8}`; `ThemePalette` renders terminal defaults and ANSI slots. `ParseRGBHex(string) (RGB, error)`, `NearestANSI(RGB) uint8`, and `ResolveThemeColor(index uint8, query func(uint8, time.Duration) (RGB, bool)) RGB` define custom parsing, fallback, and optional terminal palette resolution. Custom colors accept `#RRGGBB`; truecolor terminals render exact RGB, 16-color terminals use nearest ANSI color. Query ANSI RGB only when a theme color is assigned to a cell, with a 100 ms timeout; fall back to the spec's standard ANSI mapping.

- [ ] **Step 1: Add failing viewport and mouse tests.** Verify terminal coordinates map to expected cells at nonzero scroll offsets, wheel events preserve selection, column-boundary drag changes only that width, and terminal resize preserves the selected cell.
- [ ] **Step 2: Add failing command-palette and toolbar tests.** Verify `Ctrl+P` opens the palette, action search invokes a command, hiding the toolbar increases grid viewport, and `Ctrl+P` still works with the toolbar hidden.
- [ ] **Step 3: Add failing theme/color tests.** `TestThemePaletteAndCustomColorFallback` asserts theme colors precede custom input, valid `#RRGGBB` is accepted, malformed hex rejected, terminal ANSI RGB is used when returned within the palette-open query timeout, and custom RGB maps to nearest ANSI on 16-color terminals without changing app default foreground/background.
- [ ] **Step 4: Run focused UI tests and confirm failure.** Run `go test ./internal/ui -count=1`; expected: missing TUI behavior.
- [ ] **Step 5: Implement Bubble Tea model and viewport.** Use Lip Gloss styles that do not paint a fixed background; handle keyboard and mouse messages in the same model update loop. Query palette RGB only on-demand, never during startup.
- [ ] **Step 6: Run focused UI tests.** Expected: viewport, toolbar/palette transitions, theme ordering, and color fallback pass.
- [ ] **Step 7: Commit.** `git add internal/ui/model.go internal/ui/grid.go internal/ui/toolbar.go internal/ui/command_palette.go internal/ui/theme.go internal/ui/grid_test.go internal/ui/command_palette_test.go internal/ui/theme_test.go && git commit --only internal/ui/model.go internal/ui/grid.go internal/ui/toolbar.go internal/ui/command_palette.go internal/ui/theme.go internal/ui/grid_test.go internal/ui/command_palette_test.go internal/ui/theme_test.go -m "feat: add themed mouse-capable spreadsheet TUI"`.

### Task 8: Integrate launch/save flows, docs, and end-to-end verification

**Files:**
- Create: `cmd/shtui/main.go`
- Create: `internal/app/controller.go`
- Create: `internal/app/controller_test.go`
- Create: `docs/usage.md`
- Create: `internal/app/performance_test.go`

**Interfaces:**
- `Run(ctx context.Context, path string) error` starts the TUI; a provided path opens directly, and an empty path shows open/create actions.
- Controller messages represent loading progress, load errors, save completion, and save errors; failed opens leave the current session unchanged.

- [ ] **Step 1: Add failing controller tests.** Verify path launch opens the selected format, no-path launch enters open/create state and creates a new workbook, load errors retain the prior session, CSV loss warnings appear for styles/multiple sheets, and failed saves never mark a workbook clean.
- [ ] **Step 2: Run controller tests and confirm failure.** Run `go test ./internal/app -count=1`; expected: missing controller behavior.
- [ ] **Step 3: Integrate asynchronous loading/saving with progress.** Keep the TUI responsive while a workbook is parsed; display completion/error state and preserve dirty state until a save succeeds.
- [ ] **Step 4: Add the user guide.** Document launch, keyboard/mouse controls, command palette, toolbar toggle, formulas, CSV limitations, custom colors, and XLSX preservation limits.
- [ ] **Step 5: Run the full suite and build.** Run `go test ./...` and `go build -o /tmp/shtui ./cmd/shtui`; expected: all tests pass and a compiled executable is produced.
- [ ] **Step 6: Smoke the real TUI.** Launch with no path and with CSV/XLSX fixtures. Exercise cell editing, formula recalculation, tabs, fill/font/border/merge, conditional formatting, sort/filter/find, mouse selection/column resizing, hidden toolbar plus `Ctrl+P`, and save/reopen. Expected: visible state matches saved content and errors do not damage originals.
- [ ] **Step 7: Measure the target-size path.** Run `go test ./internal/app -run '^$' -bench '^BenchmarkOpenEditSave25kBy20$' -benchmem`; then manually time five cold PTY launches from command start to first grid frame, quitting immediately after display. Record the environment and median with no invented threshold.
- [ ] **Step 8: Commit.** Stage only `cmd/shtui/main.go`, `internal/app/controller.go`, `internal/app/controller_test.go`, `internal/app/performance_test.go`, and `docs/usage.md`; run `git commit --only` with those exact paths and message `feat: ship CSV and XLSX terminal spreadsheet`.
