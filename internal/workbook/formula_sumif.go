package workbook

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
		if leftErr != nil || rightErr != nil {
			return false, nil
		}
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

func sumifWildcardPattern(operand string) (string, error) {
	var pattern strings.Builder
	for index := 0; index < len(operand); index++ {
		switch operand[index] {
		case '~':
			index++
			if index == len(operand) {
				return "", fmt.Errorf("trailing escape")
			}
			pattern.WriteString(regexp.QuoteMeta(operand[index : index+1]))
		case '*':
			pattern.WriteString(".*")
		case '?':
			pattern.WriteByte('.')
		default:
			pattern.WriteString(regexp.QuoteMeta(operand[index : index+1]))
		}
	}
	return pattern.String(), nil
}
