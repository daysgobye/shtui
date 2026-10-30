# Task 1: SUMIF criteria matcher report

Implemented and committed the matcher and its focused tests (`fa2ce7b`, `feat(workbook): add SUMIF criteria matcher`). Changes are limited to `internal/workbook/formula_sumif.go` and `internal/workbook/formula_sumif_test.go`.

## RED

Command:
```sh
go test ./internal/workbook -run 'TestSUMIFCriteria' -count=1
```

Output:
```text
# shtui/internal/workbook [shtui/internal/workbook.test]
internal/workbook/formula_sumif_test.go:16:15: undefined: matchesSUMIFCriteria
internal/workbook/formula_sumif_test.go:33:15: undefined: matchesSUMIFCriteria
internal/workbook/formula_sumif_test.go:47:15: undefined: matchesSUMIFCriteria
internal/workbook/formula_sumif_test.go:63:15: undefined: matchesSUMIFCriteria
internal/workbook/formula_sumif_test.go:69:16: undefined: matchesSUMIFCriteria
FAIL	shtui/internal/workbook [build failed]
FAIL
```

## GREEN

Command:
```sh
go test ./internal/workbook -run 'TestSUMIFCriteria' -count=1
```

Output:
```text
ok  	shtui/internal/workbook	0.009s
```

Self-review confirmed the implementation covers case-insensitive exact/wildcard text matching, numeric comparison operators, blank criteria, literal `~` escapes, and errors for empty relational operators/trailing escape. No full suite was run.

## Review round 1 fix

Added regression coverage for case-insensitive relational text comparisons and Unicode literals, including escaped Unicode-adjacent literals. Updated wildcard pattern construction to process Unicode runes while retaining `~`, `*`, and `?` semantics. Relational comparisons now compare numerically when both inputs parse as numbers and otherwise use case-insensitive text ordering; text criteria are valid and return a comparison result rather than an error.

### RED

Command:
```sh
go test ./internal/workbook -run 'TestSUMIFCriteria' -count=1
```

Output:
```text
--- FAIL: TestSUMIFCriteriaTextComparisons (0.00s)
    formula_sumif_test.go:55: matchesSUMIFCriteria("pear", ">Apple") = false, <nil>; want true, nil
    formula_sumif_test.go:55: matchesSUMIFCriteria("APPLE", ">=apple") = false, <nil>; want true, nil
    formula_sumif_test.go:55: matchesSUMIFCriteria("Banana", "<pear") = false, <nil>; want true, nil
    formula_sumif_test.go:55: matchesSUMIFCriteria("pear", "<=Pear") = false, <nil>; want true, nil
    formula_sumif_test.go:55: matchesSUMIFCriteria("pear", ">4") = false, <nil>; want true, nil
FAIL
FAIL	shtui/internal/workbook\t0.009s
FAIL
```

### GREEN

Command:
```sh
go test ./internal/workbook -run 'TestSUMIFCriteria' -count=1
```

Output:
```text
ok  \tshtui/internal/workbook\t0.006s
```

Full suite command:
```sh
go test ./...
```

Output:
```text
?   	shtui/cmd/shtui	[no test files]
ok  	shtui/internal/app	0.147s
ok  	shtui/internal/ui	0.231s
ok  	shtui/internal/workbook	0.158s
```

## Review round 2 fix

Text relational comparisons now use a deterministic Unicode simple-fold key: each rune is mapped to the lowest rune in its `unicode.SimpleFold` cycle, then keys are ordered lexically. Wildcard matching is unchanged. Added sigma/final-sigma equivalence comparison boundaries.

### RED

Command:
```sh
go test ./internal/workbook -run 'TestSUMIFCriteria' -count=1
```

Output:
```text
--- FAIL: TestSUMIFCriteriaTextComparisons (0.00s)
    formula_sumif_test.go:58: matchesSUMIFCriteria("σ", "<=ς") = false, <nil>; want true, nil
FAIL
FAIL	shtui/internal/workbook\t0.009s
FAIL
```

### GREEN

Command:
```sh
go test ./internal/workbook -run 'TestSUMIFCriteria' -count=1
```

Output:
```text
ok  \tshtui/internal/workbook\t0.006s
```

Full suite command:
```sh
go test ./...
```

Output:
```text
?   	shtui/cmd/shtui	[no test files]
ok  	shtui/internal/app\t0.158s
ok  	shtui/internal/ui\t0.222s
ok  	shtui/internal/workbook\t0.146s

## Structural deletion range fix

Range references now map the surviving interval across row/column deletion, contract trailing and leading overlaps, preserve endpoint direction/qualifiers, and become `#REF!` only when every referenced cell is deleted. References on unrelated sheets remain unchanged.

### RED

Command:
```sh
go test ./internal/workbook -run 'Test(Insert|Delete)(Rows|Columns)|TestSheetOperations|TestRenameSheet' -count=1
```

Output:
```text
--- FAIL: TestDeleteRowsContractsFirstRangeEndpoint (0.00s)
    operations_test.go:338: cell Other!0,0 input = "=SUM(Sheet1!A2:A3)", want "=SUM(Sheet1!A1:A3)"
FAIL
FAIL	shtui/internal/workbook\t0.070s
FAIL
```

### GREEN

Command:
```sh
go test ./internal/workbook -run 'Test(Insert|Delete)(Rows|Columns)|TestSheetOperations|TestRenameSheet' -count=1
```

Output:
```text
ok  	shtui/internal/workbook\t0.067s
```

Structural change committed as `1d0fedb` (`fix(workbook): map deleted range references`).

## Structural no-overlap regression refinement

Corrected the after-row fixture so its entire range is below the deleted rows: `B7:B9` shifts to `B5:B7`; retained the before-range `A1:A3`. The existing column case remains unchanged.

Focused structural suite:
```sh
go test ./internal/workbook -run 'TestDelete(Rows|Columns)' -count=1
```
Output:
```text
ok  	shtui/internal/workbook	0.071s
```

Structural fix commit: `e17da6da665f4abc855288ba0e73bf91b09a10e8`.

Based on `480311b`.
```
