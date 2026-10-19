package workbook

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// AddColorScaleRule stores an Excel color-scale rule and includes it in undo/redo history.
func AddColorScaleRule(session *Session, sheetName string, selection CellRange, scale ColorScale) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("add color scale: session is nil")
	}
	if err := validateCellRange(selection); err != nil {
		return fmt.Errorf("add color scale: %w", err)
	}
	if _, exists := sheetIndex(session.Workbook, sheetName); !exists {
		return fmt.Errorf("add color scale: sheet %q does not exist", sheetName)
	}
	if err := validateColorScale(scale); err != nil {
		return err
	}
	if err := ensureCalculationFile(session); err != nil {
		return err
	}
	scale.MinRGB = normalizeStoredColor(scale.MinRGB)
	scale.MaxRGB = normalizeStoredColor(scale.MaxRGB)
	if scale.MidRGB != "" {
		scale.MidRGB = normalizeStoredColor(scale.MidRGB)
	}
	snapshot, err := captureSession(session)
	if err != nil {
		return fmt.Errorf("prepare color-scale undo: %w", err)
	}
	rule := ConditionalRule{Range: selection, ColorScale: &scale}
	if err := addColorScaleRuleRaw(session, sheetName, rule); err != nil {
		return err
	}
	recordHistory(session, historyEntry{
		undo: func() error { return restoreSession(session, snapshot) },
		redo: func() error { return addColorScaleRuleRaw(session, sheetName, rule) },
	})
	return nil
}

func validateColorScale(scale ColorScale) error {
	for label, color := range map[string]string{"minimum": scale.MinRGB, "maximum": scale.MaxRGB} {
		if strings.TrimSpace(color) == "" {
			return fmt.Errorf("color scale requires a %s color", label)
		}
		if _, err := normalizeRGB(color); err != nil {
			return fmt.Errorf("color scale %s color: %w", label, err)
		}
	}
	if scale.MidRGB != "" {
		if _, err := normalizeRGB(scale.MidRGB); err != nil {
			return fmt.Errorf("color scale midpoint color: %w", err)
		}
	}
	return nil
}

func addColorScaleRuleRaw(session *Session, sheetName string, rule ConditionalRule) error {
	scale := rule.ColorScale
	if scale == nil {
		return fmt.Errorf("add color scale: missing scale definition")
	}
	start, end, err := rangeCellNames(rule.Range)
	if err != nil {
		return err
	}
	options := excelize.ConditionalFormatOptions{
		Type: "2_color_scale", Criteria: "=", MinType: "min", MaxType: "max",
		MinColor: "#" + scale.MinRGB, MaxColor: "#" + scale.MaxRGB,
	}
	if scale.MidRGB != "" {
		options.Type = "3_color_scale"
		options.MidType = "percentile"
		options.MidValue = "50"
		options.MidColor = scale.MidRGB
	}
	if err := session.file.SetConditionalFormat(sheetName, start+":"+end, []excelize.ConditionalFormatOptions{options}); err != nil {
		return fmt.Errorf("set color scale %s!%s:%s: %w", sheetName, start, end, err)
	}
	idx, _ := sheetIndex(session.Workbook, sheetName)
	session.Workbook.Sheets[idx].ConditionalRules = append(session.Workbook.Sheets[idx].ConditionalRules, rule)
	session.Workbook.Dirty = true
	return nil
}

// LoadColorScales hydrates the app's color-scale rules from an opened XLSX workbook.
func LoadColorScales(session *Session) error {
	if session == nil || session.Workbook == nil {
		return fmt.Errorf("load color scales: session is nil")
	}
	if session.Format == CSV || session.file == nil {
		return nil
	}
	for sheetIndex := range session.Workbook.Sheets {
		sheet := &session.Workbook.Sheets[sheetIndex]
		formats, err := session.file.GetConditionalFormats(sheet.Name)
		if err != nil {
			return fmt.Errorf("read color scales in sheet %q: %w", sheet.Name, err)
		}
		kept := sheet.ConditionalRules[:0]
		for _, rule := range sheet.ConditionalRules {
			if rule.ColorScale == nil {
				kept = append(kept, rule)
			}
		}
		sheet.ConditionalRules = kept
		for reference, options := range formats {
			selection, err := cellRangeFromExcelize(reference)
			if err != nil {
				continue
			}
			for _, option := range options {
				if option.Type != "2_color_scale" && option.Type != "3_color_scale" {
					continue
				}
				scale := ColorScale{MinRGB: normalizeStoredColor(option.MinColor), MaxRGB: normalizeStoredColor(option.MaxColor)}
				if option.Type == "3_color_scale" {
					scale.MidRGB = normalizeStoredColor(option.MidColor)
				}
				if validateColorScale(scale) != nil {
					continue
				}
				sheet.ConditionalRules = append(sheet.ConditionalRules, ConditionalRule{Range: selection, ColorScale: &scale})
			}
		}
	}
	return nil
}
