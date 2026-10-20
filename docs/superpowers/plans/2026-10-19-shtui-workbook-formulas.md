# Shtui Workbook Formula Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Recalculate and expose the additional spreadsheet functions used by `Cozy_Bullet_Journal_Render_Test.xlsx` without building a separate formula engine.

**Architecture:** Reuse Excelize's existing `CalcCellValue` path and the formula-function guard in the workbook package. Update the formula picker catalog and formula assembly together so its syntax matches the supported functions; keep formulas stale and preserved whenever Excelize cannot calculate them.

**Tech Stack:** Go, Bubble Tea/Bubbles, Excelize. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-19-shtui-borders-exit-formulas-design.md`

## Global Constraints

- Keep Go, Bubble Tea/Bubbles, Lip Gloss, and the current workbook/Excelize layers; no new dependency is proposed.
- Reuse Excelize's existing formula calculation path. Extend its supported-function guard and the function-picker catalog together, based on verified engine behavior.
- Do not introduce a custom formula evaluator or claim support if Excelize cannot calculate a function reliably.
- Unsupported formulas remain preserved as source and continue to surface calculation errors instead of substituting a value.
- `COUNT` is already supported and remains in the function picker.

## Review Focus

1. Criteria with quoted text and numeric comparisons calculate correctly. Assert `COUNTIF(A1:A3,"pear")` is `1`, `COUNTIF(A1:A3,">0")` is `2`, `SUMIF(A1:A3,"pear",B1:B3)` is `30`, and `SUMIF(A1:A3,">0",B1:B3)` is `30` in `TestAdditionalFormulaFunctions`.
2. `IFERROR` catches a nested calculation error and returns the fallback, while ordinary calculation errors remain errors. Assert both in `TestAdditionalFormulaFunctions` and existing error-state coverage.
3. `COUNTA` is enabled only if the pinned Excelize version calculates blank/nonblank ranges correctly. Run the direct Excelize probe before adding it to the supported list; if it fails, omit it and report the limitation.
4. Nested calls no longer become stale merely because a newly supported function appears inside them. Assert `IFERROR(COUNTIF(A1:A3,">0")/COUNT(A1:A3),0)` returns `"1"` with current calculation state.
5. The function picker generates exact formulas with commas, quoted criteria, and ranges, and keeps the correct syntax preview. Assert in `TestFormulaPickerAdditionalFunctions`.

---

### Task 1: Verify Excelize and add supported calculation functions

**Files:**
- Modify: `internal/workbook/formula.go`, `internal/workbook/formula_test.go`

**Interfaces:**
- Consumes: `supportedFormulaFunctions`, `hasUnsupportedFunction`, `Recalculate`, and `excelize.File.CalcCellValue`.
- Produces: verified support for `COUNTIF`, `SUMIF`, `IFERROR`, and `COUNTA` only where the current Excelize dependency returns correct results; `COUNT` behavior remains unchanged.

- [ ] **Step 1: Probe Excelize's `COUNTA` behavior before enabling it**
  - In a throwaway test file, create a numeric cell, text cell, and blank in one range; call Excelize calculation for `COUNTA(A1:A3)` and assert the result is `2`. Delete the probe after recording the result.
  - Run: `go test ./internal/workbook -run TestExcelizeCalculatesCountaRange -count=1`. If the pinned engine does not return `2`, exclude COUNTA from the supported-function list; do not substitute a custom evaluator. If it passes, keep the `COUNTA(A1:A3) == 3` assertion in Step 2's permanent application-level test.
- [ ] **Step 2: Write failing recalculation tests for supported functions**
  - Add `TestAdditionalFormulaFunctions` using literals `A1=2`, `A2=4`, `A3="pear"`, `B1=10`, `B2=20`, `B3=30`; assert `COUNTIF(A1:A3,"pear")` is `1`, `COUNTIF(A1:A3,">0")` is `2`, `SUMIF(A1:A3,"pear",B1:B3)` is `30`, `SUMIF(A1:A3,">0",B1:B3)` is `30`, and `IFERROR(1/0,"safe")` is `safe`.
  - If Step 1 passed, also assert `COUNTA(A1:A3)` is `3`.
  - Add `TestNestedSupportedFunctionsRecalculate` asserting `IFERROR(COUNTIF(A1:A3,">0")/COUNT(A1:A3),0)` has the expected current result and unchanged formula source.
- [ ] **Step 3: Run focused tests and confirm they fail**

  Run: `go test ./internal/workbook -run 'TestAdditionalFormulaFunctions|TestNestedSupportedFunctionsRecalculate' -count=1`

  Expected: FAIL because the supported-function guard marks these formulas stale.
- [ ] **Step 4: Enable only verified functions**
  - Add `COUNTIF`, `SUMIF`, and `IFERROR` to `supportedFormulaFunctions`; add `COUNTA` only if Step 1 proves it.
  - Do not change the generic Excelize recalculation/error path. Preserve original formula input for current, stale, and error results.
- [ ] **Step 5: Run workbook formula tests**

  Run: `go test ./internal/workbook -run 'Test.*Formula|TestCommonFormulaFunctions' -count=1`

  Expected: PASS; every enabled function has the asserted result and current state, while unsupported/error cases retain their existing semantics.
- [ ] **Step 6: Commit**

  ```bash
  git add internal/workbook/formula.go internal/workbook/formula_test.go
  git commit -m "feat(workbook): support fixture formula functions"
  ```

### Task 2: Expose the supported functions through the formula picker

**Files:**
- Modify: `internal/ui/formula_picker.go`, `docs/usage.md`
- Create: `internal/ui/formula_picker_test.go`

**Interfaces:**
- Consumes: Task 1's verified function set, `functionDefinition`, `formulaPickerState.Formula`, and the existing single argument text input.
- Produces: picker entries and syntax previews for `COUNTIF(range, criteria)`, `SUMIF(range, criteria, sum_range)`, `IFERROR(value, value_if_error)`, and verified `COUNTA(range)`; generated formula text preserves the user's complete comma-separated arguments.

- [ ] **Step 1: Write failing formula-picker tests**
  - Add `TestFormulaPickerAdditionalFunctions` as table-driven cases: `COUNTIF` with `A1:A3,">0"`; `SUMIF` with `A1:A3,">0",B1:B3`; `IFERROR` with `A1/0,"safe"`; and `COUNTA` with `A1:A3` only if enabled in Task 1.
  - Assert each function appears in the list, the syntax preview names the correct arguments, and `Formula()` returns the exact `=`-prefixed expression.
  - Assert the existing COUNT formula still returns `=COUNT(A1:A3)`.
- [ ] **Step 2: Run focused picker tests and confirm they fail**

  Run: `go test ./internal/ui -run 'TestFormulaPickerAdditionalFunctions' -count=1`

  Expected: FAIL because the new functions are absent from the catalog or formula assembly.
- [ ] **Step 3: Implement function definitions and formula assembly**
  - Add descriptions and argument syntax matching the verified calculation functions.
  - For multi-argument functions, accept the entered expression as the function's full argument list rather than passing it through the single-range parser; retain range validation for existing one-range functions.
  - Keep the picker preview synchronized with the exact generated formula and omit COUNTA if Task 1 did not enable it.
- [ ] **Step 4: Update formula usage documentation**
  - Update `docs/usage.md` to list the additional verified functions and give examples for criteria, SUMIF, and IFERROR; retain the warning that Excelize support is not full Excel compatibility.
- [ ] **Step 5: Run picker and workbook integration checks**

  Run: `go test ./internal/ui -run 'TestFormulaPickerAdditionalFunctions' -count=1 && go test ./internal/workbook -run 'TestAdditionalFormulaFunctions|TestNestedSupportedFunctionsRecalculate' -count=1`

  Expected: PASS; picker output matches formulas that the workbook layer recalculates correctly.

  Then use a throwaway Go smoke program to open `test_sheets/Cozy_Bullet_Journal_Render_Test.xlsx`, recalculate a representative formula from each newly supported function, and assert the formula source remains unchanged and the calculation state is current where Excelize supports it. Remove the program after the run; record any remaining formula errors instead of hiding them.
- [ ] **Step 6: Commit**

  ```bash
  git add internal/ui/formula_picker.go internal/ui/formula_picker_test.go docs/usage.md
  git commit -m "feat(ui): expose workbook formula functions"
  ```
