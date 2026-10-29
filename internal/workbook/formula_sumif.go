package workbook

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// matchesSUMIFCriteria reports whether a cell value satisfies an Excel-style SUMIF criterion.
func matchesSUMIFCriteria(value, criteria string) (bool, error) {
	operator := "="
	operand := criteria
	for _, candidate := range []string{">=", "<=", "<>", ">", "<", "="} {
		if strings.HasPrefix(criteria, candidate) {
			operator = candidate
			operand = criteria[len(candidate):]
			break
		}
	}
	if (operator == ">" || operator == "<" || operator == ">=" || operator == "<=") && operand == "" {
		return false, fmt.Errorf("SUMIF criterion %q has no comparison value", criteria)
	}

	if operator == "=" && operand == "" {
		return value == "", nil
	}
	if operator == "<>" && operand == "" {
		return value != "", nil
	}

	if operator != "=" && operator != "<>" {
		left, leftErr := strconv.ParseFloat(value, 64)
		right, rightErr := strconv.ParseFloat(operand, 64)
		if leftErr == nil && rightErr == nil {
			switch operator {
			case ">":
				return left > right, nil
			case "<":
				return left < right, nil
			case ">=":
				return left >= right, nil
			case "<=":
				return left <= right, nil
			}
		}
		leftText, rightText := simpleFoldKey(value), simpleFoldKey(operand)
		switch operator {
		case ">":
			return leftText > rightText, nil
		case "<":
			return leftText < rightText, nil
		case ">=":
			return leftText >= rightText, nil
		case "<=":
			return leftText <= rightText, nil
		}
	}

	pattern, err := sumifWildcardPattern(operand)
	if err != nil {
		return false, fmt.Errorf("invalid SUMIF criterion %q: %w", criteria, err)
	}
	matched, err := regexp.MatchString("(?i)^(?:"+pattern+")$", value)
	if err != nil {
		return false, fmt.Errorf("invalid SUMIF criterion %q: %w", criteria, err)
	}
	if operator == "<>" {
		return !matched, nil
	}
	return matched, nil
}

func simpleFoldKey(value string) string {
	var key strings.Builder
	for _, r := range value {
		minimum := r
		for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
			if folded < minimum {
				minimum = folded
			}
		}
		key.WriteRune(minimum)
	}
	return key.String()
}

func sumifWildcardPattern(operand string) (string, error) {
	var pattern strings.Builder
	runes := []rune(operand)
	for index := 0; index < len(runes); index++ {
		switch runes[index] {
		case '~':
			index++
			if index == len(runes) {
				return "", fmt.Errorf("trailing escape")
			}
			pattern.WriteString(regexp.QuoteMeta(string(runes[index])))
		case '*':
			pattern.WriteString(".*")
		case '?':
			pattern.WriteByte('.')
		default:
			pattern.WriteString(regexp.QuoteMeta(string(runes[index])))
		}
	}
	return pattern.String(), nil
}

// evaluateSUMIF handles only a top-level SUMIF call; all other formulas remain
// on the existing Excelize calculation path.
func evaluateSUMIF(session *Session, formulaSheet, formula string) (string, bool, error) {
	name := strings.TrimSpace(formula)
	open := strings.IndexByte(name, '(')
	if open < 0 || !strings.EqualFold(strings.TrimSpace(name[:open]), "SUMIF") {
		return "", false, nil
	}
	close, err := sumifCallClose(name, open)
	if err != nil {
		return "", true, err
	}
	if strings.TrimSpace(name[close+1:]) != "" {
		return "", false, nil
	}
	if session == nil || session.Workbook == nil {
		return "", true, fmt.Errorf("SUMIF requires an active workbook")
	}
	arguments, err := splitSUMIFArguments(name[open+1 : close])
	if err != nil {
		return "", true, err
	}
	if len(arguments) < 2 || len(arguments) > 3 {
		return "", true, fmt.Errorf("SUMIF requires two or three arguments")
	}
	criteriaRange, err := parseSUMIFRange(arguments[0], formulaSheet)
	if err != nil {
		return "", true, fmt.Errorf("invalid SUMIF criteria range: %w", err)
	}
	if _, ok := sheetIndex(session.Workbook, criteriaRange.sheet); !ok {
		return "", true, fmt.Errorf("SUMIF criteria sheet %q does not exist", criteriaRange.sheet)
	}
	criteria, err := sumifCriteriaValue(session, formulaSheet, arguments[1])
	if err != nil {
		return "", true, err
	}
	sumRange := criteriaRange
	if len(arguments) == 3 {
		sumRange, err = parseSUMIFRange(arguments[2], formulaSheet)
		if err != nil {
			return "", true, fmt.Errorf("invalid SUMIF sum range: %w", err)
		}
		if _, ok := sheetIndex(session.Workbook, sumRange.sheet); !ok {
			return "", true, fmt.Errorf("SUMIF sum sheet %q does not exist", sumRange.sheet)
		}
	}
	rows := criteriaRange.end.Row - criteriaRange.start.Row + 1
	cols := criteriaRange.end.Col - criteriaRange.start.Col + 1
	if rows <= 0 || cols <= 0 {
		return "", true, fmt.Errorf("invalid SUMIF criteria range dimensions")
	}
	var total float64
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			criteriaValue := sumifCellValue(session, criteriaRange.sheet, criteriaRange.start.Row+row, criteriaRange.start.Col+col)
			matches, matchErr := matchesSUMIFCriteria(criteriaValue, criteria)
			if matchErr != nil {
				return "", true, matchErr
			}
			if !matches {
				continue
			}
			sumRow, sumCol := sumRange.start.Row+row, sumRange.start.Col+col
			value := sumifCellValue(session, sumRange.sheet, sumRow, sumCol)
			number, parseErr := strconv.ParseFloat(value, 64)
			if parseErr == nil {
				total += number
			}
		}
	}
	return strconv.FormatFloat(total, 'f', -1, 64), true, nil
}

type sumifRange struct {
	sheet      string
	start, end Coord
}

func parseSUMIFRange(reference, defaultSheet string) (sumifRange, error) {
	reference = strings.TrimSpace(strings.ReplaceAll(reference, "$", ""))
	sheet := defaultSheet
	if bang := strings.LastIndexByte(reference, '!'); bang >= 0 {
		sheet = strings.TrimSpace(reference[:bang])
		if len(sheet) >= 2 && sheet[0] == '\'' && sheet[len(sheet)-1] == '\'' {
			sheet = strings.ReplaceAll(sheet[1:len(sheet)-1], "''", "'")
		}
		reference = strings.TrimSpace(reference[bang+1:])
	}
	if sheet == "" {
		return sumifRange{}, fmt.Errorf("missing sheet name")
	}
	parts := strings.Split(reference, ":")
	if len(parts) == 1 {
		parts = append(parts, parts[0])
	}
	if len(parts) != 2 {
		return sumifRange{}, fmt.Errorf("invalid range %q", reference)
	}
	startCol, startRow, err := excelize.CellNameToCoordinates(strings.TrimSpace(parts[0]))
	if err != nil {
		return sumifRange{}, err
	}
	endCol, endRow, err := excelize.CellNameToCoordinates(strings.TrimSpace(parts[1]))
	if err != nil {
		return sumifRange{}, err
	}
	start := Coord{Row: startRow - 1, Col: startCol - 1}
	end := Coord{Row: endRow - 1, Col: endCol - 1}
	if start.Row > end.Row || start.Col > end.Col {
		return sumifRange{}, fmt.Errorf("reversed range %q", reference)
	}
	return sumifRange{sheet: sheet, start: start, end: end}, nil
}

func sumifCellValue(session *Session, sheet string, row, col int) string {
	if session == nil || session.Workbook == nil {
		return ""
	}
	index, ok := sheetIndex(session.Workbook, sheet)
	if !ok {
		return ""
	}
	cell, ok := session.Workbook.Sheets[index].Cells[Coord{Row: row, Col: col}]
	if !ok {
		return ""
	}
	if strings.HasPrefix(cell.Input, "=") {
		return cell.Result
	}
	return cell.Input
}

func sumifCriteriaValue(session *Session, formulaSheet, argument string) (string, error) {
	argument = strings.TrimSpace(argument)
	if len(argument) >= 2 && argument[0] == '"' && argument[len(argument)-1] == '"' {
		return strings.ReplaceAll(argument[1:len(argument)-1], `""`, `"`), nil
	}
	if strings.HasPrefix(argument, "'") || strings.Contains(argument, "!") || isSUMIFCellReference(argument) {
		reference := argument
		if !strings.Contains(reference, "!") {
			reference = formulaSheet + "!" + reference
		}
		cell, err := parseSUMIFRange(reference, formulaSheet)
		if err != nil {
			return "", fmt.Errorf("invalid SUMIF criterion reference: %w", err)
		}
		if cell.start != cell.end {
			return "", fmt.Errorf("SUMIF criterion must be a single cell")
		}
		if _, ok := sheetIndex(session.Workbook, cell.sheet); !ok {
			return "", fmt.Errorf("SUMIF criterion sheet %q does not exist", cell.sheet)
		}
		return sumifCellValue(session, cell.sheet, cell.start.Row, cell.start.Col), nil
	}
	return argument, nil
}

func isSUMIFCellReference(value string) bool {
	rangeValue, err := parseSUMIFRange(value, "Sheet1")
	return err == nil && rangeValue.start == rangeValue.end
}

func sumifCallClose(formula string, open int) (int, error) {
	depth := 0
	inString := false
	for index := open; index < len(formula); index++ {
		switch formula[index] {
		case '"':
			if inString && index+1 < len(formula) && formula[index+1] == '"' {
				index++
			} else {
				inString = !inString
			}
		case '(':
			if !inString {
				depth++
			}
		case ')':
			if !inString {
				depth--
				if depth == 0 {
					return index, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("malformed SUMIF call")
}

func splitSUMIFArguments(value string) ([]string, error) {
	arguments := make([]string, 0, 3)
	start, depth := 0, 0
	inString := false
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '"':
			if inString && index+1 < len(value) && value[index+1] == '"' {
				index++
			} else {
				inString = !inString
			}
		case '(':
			if !inString {
				depth++
			}
		case ')':
			if !inString {
				depth--
				if depth < 0 {
					return nil, fmt.Errorf("malformed SUMIF arguments")
				}
			}
		case ',':
			if !inString && depth == 0 {
				arguments = append(arguments, strings.TrimSpace(value[start:index]))
				start = index + 1
			}
		}
	}
	if inString || depth != 0 {
		return nil, fmt.Errorf("malformed SUMIF arguments")
	}
	arguments = append(arguments, strings.TrimSpace(value[start:]))
	return arguments, nil
}
