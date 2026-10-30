# SUMIF Formula Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Calculate common Excel-compatible SUMIF formulas and keep results current when their source cells change.

**Architecture:** Add a focused SUMIF evaluator to the existing workbook formula path, retaining source formula text and existing stale/error states for unsupported inputs. Recalculate results through the same dependency refresh path used by cell edits and workbook loads.

**Tech Stack:** Go, Excelize workbook model, existing formula/reference utilities.

**Spec:** `docs/superpowers/specs/2026-10-29-spreadsheet-clipboard-sumif-resize-design.md`

## Global Constraints

- Evaluate `SUMIF(criteria_range, criteria, [sum_range])`; omitted `sum_range` means `criteria_range`.
- Support exact text, numeric and blank criteria; `=`, `<>`, `<`, `<=`, `>`, `>=`; `*` and `?` wildcards; and `~` wildcard escaping.
- Compare text case-insensitively, preserve formula source, and recalculate dependents.
- Unsupported or invalid criteria/ranges MUST NOT produce fabricated totals; do not claim full Excel compatibility.

## Review Focus

- Criteria/sum ranges with unequal dimensions or reversed criteria ranges: no panics or misaligned aggregation; test in `internal/workbook/formula_test.go`.
- Text containing literal `*`, `?`, or `~`: escape semantics match the spec; test in `internal/workbook/formula_test.go`.
- Numeric-looking text and blank cells: classify consistently and preserve stale/error state when comparison is unsupported; tests in `internal/workbook/formula_test.go`.
- SUMIF formulas loaded from XLSX keep source and produce current results after load; test in `internal/workbook/xlsx_test.go`.
- Editing a matched amount or category recalculates a dependent total; test in `internal/workbook/formula_test.go`.

---

### Task 1: Criteria matcher

**Files:**
- Create: `internal/workbook/formula_sumif.go`
- Create: `internal/workbook/formula_sumif_test.go`

**Interfaces:**
- Produces: `matchesSUMIFCriteria(value, criteria string) (bool, error)`; matcher is case-insensitive for text, uses numeric ordering when both values are numeric and case-insensitive lexical ordering for text relational criteria, supports blank matching, wildcard `*`/`?`, and `~` escaping. Malformed operators and unsupported criteria return errors rather than non-matches.

- [ ] **Step 1: Write tests** `TestSUMIFCriteriaExactAndCaseInsensitive`, `TestSUMIFCriteriaNumericComparisons`, `TestSUMIFCriteriaTextComparisons`, `TestSUMIFCriteriaBlank`, and `TestSUMIFCriteriaWildcardsAndEscapes`. Assert true/false boundaries, case folding, blank handling, numeric-versus-text handling, and literal wildcard escapes including non-ASCII text.
- [ ] **Step 2: Implement criteria parsing/matching** in `internal/workbook/formula_sumif.go`; reject malformed operators or trailing escapes with an error.
- [ ] **Step 3: Run `go test ./internal/workbook -run 'TestSUMIFCriteria' -count=1`; expected: PASS.**
- [ ] **Step 4: Commit** matcher and tests.

### Task 2: Range aggregation and formula recalculation

**Files:**
- Modify: `internal/workbook/formula_sumif.go`
- Modify: `internal/workbook/formula.go`
- Modify: `internal/workbook/formula_test.go`
- Create: `internal/workbook/formula_sumif_xlsx_test.go`
- Modify: `internal/ui/formula_picker.go`
- Modify: `internal/ui/formula_picker_test.go`
- Modify: `docs/usage.md`

**Interfaces:**
- Consumes: `matchesSUMIFCriteria(value, criteria string) (bool, error)` from Task 1.
- Produces: `evaluateSUMIF(session *Session, formulaSheet, formula string) (result string, handled bool, err error)`; `handled=false` for formulas that are not a top-level SUMIF, preserving the existing Excelize path.

- [ ] **Step 1: Write workbook tests** `TestSUMIFUsesCriteriaAndSumRanges`, `TestSUMIFDefaultsSumRange`, `TestSUMIFMismatchedSumRangeUsesCriteriaShape`, `TestSUMIFSourceEditRecalculatesTotal`, and `TestSUMIFLoadedFromXLSXRecalculates`, plus UI test `TestFormulaPickerSupportsSUMIFArguments`. Assert exact totals, formula source, error/stale states, dependent recalculation after source edits, and the picker creates a valid SUMIF formula from three arguments. For unequal `sum_range` dimensions, follow Excel: start at its top-left and use the criteria range's height/width even if that extends beyond the declared sum range; ignore excess declared cells.
- [ ] **Step 2: Implement `evaluateSUMIF`** for 2- or 3-argument formulas using existing cell-reference parsing conventions; map criteria-range cells to corresponding sum-range cells, sum numeric values only, and preserve exact formula input.
- [ ] **Step 3: Integrate SUMIF before the unsupported-function stale branch** in `Recalculate`; continue recalculating downstream formulas through the existing workbook loop.
- [ ] **Step 4: Document SUMIF syntax, criteria support, and limits** in `docs/usage.md`.
- [ ] **Step 5: Run `go test ./internal/workbook -run 'Test(SUMIF|OpenXLSX.*Formula)' -count=1` and `go test ./internal/ui -run '^TestFormulaPickerSupportsSUMIFArguments$' -count=1`; expected: PASS.**
- [ ] **Step 6: Smoke SUMIF in the real TUI** by entering category and amount data, adding a SUMIF formula, editing a matched cell, and observing the total update.
- [ ] **Step 7: Commit** formula, tests, and docs.
