package workbook

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// SnapshotParts hashes each uncompressed package part by its path.
func SnapshotParts(path string) (map[string][32]byte, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open workbook package %q: %w", path, err)
	}
	defer reader.Close()

	parts := make(map[string][32]byte, len(reader.File))
	for _, part := range reader.File {
		rc, err := part.Open()
		if err != nil {
			return nil, fmt.Errorf("open package part %q: %w", part.Name, err)
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read package part %q: %w", part.Name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close package part %q: %w", part.Name, closeErr)
		}
		parts[part.Name] = sha256.Sum256(data)
	}
	return parts, nil
}

// AuditPackage rejects package entry, relationship, or unsupported XML loss before replacement.
func AuditPackage(sourcePath, candidatePath string, allowedChanges map[string]struct{}) error {
	sourceParts, err := SnapshotParts(sourcePath)
	if err != nil {
		return err
	}
	candidateParts, err := SnapshotParts(candidatePath)
	if err != nil {
		return err
	}
	for name, originalHash := range sourceParts {
		candidateHash, exists := candidateParts[name]
		if !exists {
			return fmt.Errorf("package part %q was removed", name)
		}
		if candidateHash == originalHash {
			continue
		}
		if strings.HasSuffix(name, ".rels") {
			return fmt.Errorf("relationship part %q changed", name)
		}
		if _, allowed := allowedChanges[name]; !allowed {
			return fmt.Errorf("package part %q changed outside the edited content", name)
		}
		if strings.HasSuffix(strings.ToLower(name), ".xml") {
			before, err := readPackagePart(sourcePath, name)
			if err != nil {
				return err
			}
			after, err := readPackagePart(candidatePath, name)
			if err != nil {
				return err
			}
			beforeUnknown, err := unsupportedXMLContent(name, before)
			if err != nil {
				return fmt.Errorf("audit source XML part %q: %w", name, err)
			}
			afterUnknown, err := unsupportedXMLContent(name, after)
			if err != nil {
				return fmt.Errorf("audit candidate XML part %q: %w", name, err)
			}
			if !equalStrings(beforeUnknown, afterUnknown) {
				return fmt.Errorf("unsupported XML content changed in package part %q", name)
			}
		}
	}
	for name := range candidateParts {
		if _, existed := sourceParts[name]; !existed {
			return fmt.Errorf("package part %q was added", name)
		}
	}
	return nil
}

func safeWorkbookChanges(original map[string][32]byte) map[string]struct{} {
	allowed := make(map[string]struct{})
	for name := range original {
		switch name {
		case "xl/workbook.xml", "xl/styles.xml", "xl/sharedStrings.xml", "docProps/core.xml", "docProps/app.xml":
			allowed[name] = struct{}{}
		default:
			if isWorksheetXML(name) {
				allowed[name] = struct{}{}
			}
		}
	}
	return allowed
}

func isWorksheetXML(name string) bool {
	const prefix = "xl/worksheets/sheet"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".xml") {
		return false
	}
	number := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".xml")
	if number == "" {
		return false
	}
	for _, digit := range number {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func readPackagePart(path, name string) ([]byte, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open workbook package %q: %w", path, err)
	}
	defer reader.Close()
	for _, part := range reader.File {
		if part.Name != name {
			continue
		}
		rc, err := part.Open()
		if err != nil {
			return nil, fmt.Errorf("open package part %q: %w", name, err)
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read package part %q: %w", name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close package part %q: %w", name, closeErr)
		}
		return data, nil
	}
	return nil, fmt.Errorf("package part %q is missing from %q", name, path)
}

func unsupportedXMLContent(part string, data []byte) ([]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	stack := make([]xml.Name, 0, 8)
	content := make([]string, 0)
	var captured strings.Builder
	captureDepth := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			stack = append(stack, token.Name)
			if captureDepth == 0 && unsupportedElement(part, stack, token.Name.Local) {
				captured.Reset()
				captureDepth = 1
			} else if captureDepth > 0 {
				captureDepth++
			}
			if captureDepth > 0 {
				captured.WriteString(canonicalXMLToken(token))
			}
		case xml.EndElement:
			if captureDepth > 0 {
				captured.WriteString(canonicalXMLToken(token))
				captureDepth--
				if captureDepth == 0 {
					content = append(content, captured.String())
				}
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if captureDepth > 0 && strings.TrimSpace(string(token)) != "" {
				captured.WriteString("text:")
				captured.WriteString(strconv.Quote(string(token)))
			}
		case xml.Comment:
			if captureDepth > 0 {
				captured.WriteString("comment:")
				captured.WriteString(strconv.Quote(string(token)))
			}
		}
	}
	return content, nil
}

func unsupportedElement(part string, stack []xml.Name, local string) bool {
	if local == "extLst" {
		return true
	}
	if !strings.HasPrefix(part, "xl/worksheets/") || len(stack) != 2 {
		return false
	}
	_, known := worksheetRootElements[local]
	return !known
}

var worksheetRootElements = map[string]struct{}{
	"sheetPr": {}, "dimension": {}, "sheetViews": {}, "sheetFormatPr": {}, "cols": {}, "sheetData": {},
	"sheetCalcPr": {}, "sheetProtection": {}, "protectedRanges": {}, "scenarios": {}, "autoFilter": {},
	"sortState": {}, "dataConsolidate": {}, "customSheetViews": {}, "mergeCells": {}, "phoneticPr": {},
	"conditionalFormatting": {}, "dataValidations": {}, "hyperlinks": {}, "printOptions": {}, "pageMargins": {},
	"pageSetup": {}, "headerFooter": {}, "rowBreaks": {}, "colBreaks": {}, "customProperties": {},
	"cellWatches": {}, "ignoredErrors": {}, "smartTags": {}, "drawing": {}, "legacyDrawing": {},
	"legacyDrawingHF": {}, "picture": {}, "oleObjects": {}, "controls": {}, "webPublishItems": {},
	"tableParts": {}, "extLst": {},
}

func canonicalXMLToken(token xml.Token) string {
	switch token := token.(type) {
	case xml.StartElement:
		attributes := make([]string, 0, len(token.Attr))
		for _, attribute := range token.Attr {
			attributes = append(attributes, attribute.Name.Space+":"+attribute.Name.Local+"="+strconv.Quote(attribute.Value))
		}
		sort.Strings(attributes)
		return "<" + token.Name.Space + ":" + token.Name.Local + " " + strings.Join(attributes, " ") + ">"
	case xml.EndElement:
		return "</" + token.Name.Space + ":" + token.Name.Local + ">"
	default:
		return ""
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
