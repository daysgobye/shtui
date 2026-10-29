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
