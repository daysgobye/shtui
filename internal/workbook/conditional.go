package workbook

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type RuleOperator string

const (
	GreaterThan  RuleOperator = "greater-than"
	LessThan     RuleOperator = "less-than"
	Equal        RuleOperator = "equal"
	Blank        RuleOperator = "blank"
	TextContains RuleOperator = "text-contains"
)

// ConditionalRule stores one range-scoped rule and its display colors.
type ConditionalRule struct {
	Range    CellRange
	Operator RuleOperator
	Value    string
	FillRGB  string
	FontRGB  string
}

// AddConditionalRule applies and stores a supported conditional-format rule.
func AddConditionalRule(session *Session, sheetName string, selection CellRange, rule ConditionalRule) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("add conditional rule: session is nil")
	}
	if err := validateCellRange(selection); err != nil {
		return fmt.Errorf("add conditional rule: %w", err)
	}
	_, exists := sheetIndex(session.Workbook, sheetName)
	if !exists {
		return fmt.Errorf("add conditional rule: sheet %q does not exist", sheetName)
	}
	if err := validateConditionalRule(rule); err != nil {
		return err
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	snapshot, err := captureSession(session)
	if err != nil {
		return fmt.Errorf("prepare conditional-format undo: %w", err)
	}
	rule.Range = selection
	if err := addConditionalRuleRaw(session, sheetName, rule); err != nil {
		return err
	}
	recordHistory(session, historyEntry{
		undo: func() error { return restoreSession(session, snapshot) },
		redo: func() error { return addConditionalRuleRaw(session, sheetName, rule) },
	})
	return nil
}

func addConditionalRuleRaw(session *Session, sheetName string, rule ConditionalRule) error {
	style, err := styleToExcelize(CellStyle{FillRGB: rule.FillRGB, FontRGB: rule.FontRGB})
	if err != nil {
		return err
	}
	format, err := session.file.NewConditionalStyle(&style)
	if err != nil {
		return fmt.Errorf("create conditional style: %w", err)
	}
	start, end, err := rangeCellNames(rule.Range)
	if err != nil {
		return err
	}
	options := excelize.ConditionalFormatOptions{Format: &format}
	switch rule.Operator {
	case GreaterThan:
		options.Type, options.Criteria, options.Value = "cell", ">", rule.Value
	case LessThan:
		options.Type, options.Criteria, options.Value = "cell", "<", rule.Value
	case Equal:
		options.Type, options.Criteria, options.Value = "cell", "==", rule.Value
	case Blank:
		options.Type, options.Criteria = "blanks", "="
	case TextContains:
		options.Type, options.Criteria, options.Value = "text", "containing", rule.Value
	default:
		return fmt.Errorf("unsupported conditional rule operator %q", rule.Operator)
	}
	if err := session.file.SetConditionalFormat(sheetName, start+":"+end, []excelize.ConditionalFormatOptions{options}); err != nil {
		return fmt.Errorf("set conditional format %s!%s:%s: %w", sheetName, start, end, err)
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	session.Workbook.Sheets[idx].ConditionalRules = append(session.Workbook.Sheets[idx].ConditionalRules, rule)
	session.Workbook.Dirty = true
	return nil
}

func validateConditionalRule(rule ConditionalRule) error {
	switch rule.Operator {
	case GreaterThan, LessThan:
		if _, err := strconv.ParseFloat(rule.Value, 64); err != nil {
			return fmt.Errorf("conditional rule %q requires a numeric value", rule.Operator)
		}
	case Equal:
		if rule.Value == "" {
			return fmt.Errorf("equal rule requires a nonempty value")
		}
	case TextContains:
		if rule.Value == "" {
			return fmt.Errorf("text-contains rule requires a nonempty value")
		}
	case Blank:
	default:
		return fmt.Errorf("unsupported conditional rule operator %q", rule.Operator)
	}
	if rule.FillRGB != "" {
		if _, err := normalizeRGB(rule.FillRGB); err != nil {
			return err
		}
	}
	if rule.FontRGB != "" {
		if _, err := normalizeRGB(rule.FontRGB); err != nil {
			return err
		}
	}
	if rule.FillRGB == "" && rule.FontRGB == "" {
		return fmt.Errorf("conditional rule must set a fill or font color")
	}
	return nil
}

// EffectiveStyle combines direct formatting with matching conditional formatting.
func EffectiveStyle(sheet *Sheet, address Coord) CellStyle {
	if sheet == nil {
		return CellStyle{}
	}
	cell := sheet.Cells[address]
	style := cell.Style
	value := cell.Input
	if strings.HasPrefix(value, "=") {
		if cell.CalculationState != Current {
			return style
		}
		value = cell.Result
	}
	for _, rule := range sheet.ConditionalRules {
		if !rangeContains(rule.Range, address) || !conditionalMatches(rule, value) {
			continue
		}
		if rule.FillRGB != "" {
			style.FillRGB = normalizeStoredColor(rule.FillRGB)
		}
		if rule.FontRGB != "" {
			style.FontRGB = normalizeStoredColor(rule.FontRGB)
		}
	}
	return style
}

func conditionalMatches(rule ConditionalRule, value string) bool {
	switch rule.Operator {
	case GreaterThan:
		left, leftErr := strconv.ParseFloat(value, 64)
		right, rightErr := strconv.ParseFloat(rule.Value, 64)
		return leftErr == nil && rightErr == nil && left > right
	case LessThan:
		left, leftErr := strconv.ParseFloat(value, 64)
		right, rightErr := strconv.ParseFloat(rule.Value, 64)
		return leftErr == nil && rightErr == nil && left < right
	case Equal:
		left, leftErr := strconv.ParseFloat(value, 64)
		right, rightErr := strconv.ParseFloat(rule.Value, 64)
		if leftErr == nil && rightErr == nil {
			return left == right
		}
		return strings.EqualFold(value, rule.Value)
	case Blank:
		return value == ""
	case TextContains:
		return strings.Contains(strings.ToLower(value), strings.ToLower(rule.Value))
	default:
		return false
	}
}

func rangeContains(selection CellRange, address Coord) bool {
	return address.Row >= selection.Start.Row && address.Row <= selection.End.Row && address.Col >= selection.Start.Col && address.Col <= selection.End.Col
}
