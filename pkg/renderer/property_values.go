package renderer

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

const (
	propertyColumnGap         = 2
	minimumPropertyLabelWidth = 8
	minimumPropertyValueWidth = 20
	verticalPropertyIndent    = 2
)

type propertyValue struct {
	label      string
	value      string
	role       presentation.StyleRole
	emphasized bool
	heading    bool
}

type propertyLayout struct {
	labelWidth  int
	valueWidth  int
	outputWidth int
	twoColumn   bool
}

func renderPropertyValues(
	response *plugin.Response,
	writer io.Writer,
	widthProvider OutputWidthProvider,
	styler presentationStyler,
) (bool, error) {
	properties, declared, err := responsePropertyValues(response)
	if err != nil {
		return declared, err
	}
	if !declared {
		return false, nil
	}
	if len(properties) == 0 {
		_, _ = fmt.Fprintln(writer, styler.semantic(presentation.StyleMuted, false, "No resources found."))
		return true, nil
	}

	outputWidth, widthAvailable := widthProvider.Width(writer)
	compact := response.PresentationProperties != nil &&
		(response.PresentationProperties.Title != "" || response.PresentationProperties.SectionTitle != "")
	if response.PresentationProperties != nil && response.PresentationProperties.Title != "" {
		printPresentationTitle(writer, response.PresentationProperties.Title, styler)
		_, _ = fmt.Fprintln(writer)
	}
	if response.PresentationProperties != nil && response.PresentationProperties.SectionTitle != "" {
		printPresentationTitle(writer, response.PresentationProperties.SectionTitle, styler)
		_, _ = fmt.Fprintln(writer)
	}
	if containsPropertyHeadings(properties) {
		return true, renderPropertyRecords(writer, properties, outputWidth, widthAvailable, styler)
	}
	layout := calculatePropertyLayout(properties, outputWidth, widthAvailable)
	if layout.twoColumn {
		return true, renderTwoColumnProperties(writer, properties, layout, compact, styler)
	}
	return true, renderVerticalProperties(writer, properties, layout.outputWidth, widthAvailable, compact, styler)
}

func responsePropertyValues(response *plugin.Response) ([]propertyValue, bool, error) {
	if response.PresentationProperties != nil {
		properties, err := declaredPropertyValues(response)
		return properties, true, err
	}
	properties, ok := itemPropertyValues(findListInData(response.Data))
	return properties, ok, nil
}

func declaredPropertyValues(response *plugin.Response) ([]propertyValue, error) {
	if strings.TrimSpace(response.PresentationProperties.Title) != response.PresentationProperties.Title {
		return nil, fmt.Errorf("property presentation title must not have surrounding whitespace")
	}
	if strings.TrimSpace(response.PresentationProperties.SectionTitle) != response.PresentationProperties.SectionTitle {
		return nil, fmt.Errorf("property presentation section title must not have surrounding whitespace")
	}
	declarations := response.PresentationProperties.Properties
	if len(declarations) == 0 {
		return nil, fmt.Errorf("property presentation declaration is empty")
	}
	properties := make([]propertyValue, 0, len(declarations))
	seenKeys := make(map[string]struct{}, len(declarations))
	for _, declaration := range declarations {
		key := strings.TrimSpace(declaration.Key)
		label := strings.TrimSpace(declaration.Label)
		hasValue := declaration.Value != nil
		if !validStyleRole(declaration.Role) {
			return nil, fmt.Errorf("property presentation declaration contains invalid semantic role %q", declaration.Role)
		}
		if declaration.Heading {
			if label == "" || label != declaration.Label || key != "" || hasValue {
				return nil, fmt.Errorf("property presentation heading must contain only one label")
			}
			properties = append(properties, propertyValue{
				label: label, role: declaration.Role, emphasized: declaration.Emphasized, heading: true,
			})
			continue
		}
		if label == "" || label != declaration.Label || key != declaration.Key || (key == "") == !hasValue {
			return nil, fmt.Errorf("property presentation declaration must contain one label and exactly one data key or value")
		}

		value := declaration.Value
		if key != "" {
			if _, duplicate := seenKeys[key]; duplicate {
				return nil, fmt.Errorf("property presentation declaration repeats data key %q", key)
			}
			var present bool
			value, present = response.Data[key]
			if !present {
				return nil, fmt.Errorf("property presentation data key %q is missing", key)
			}
			seenKeys[key] = struct{}{}
		}
		properties = append(properties, propertyValue{
			label: label, value: formatValue(value), role: declaration.Role, emphasized: declaration.Emphasized,
		})
	}
	return properties, nil
}

func itemPropertyValues(items any) ([]propertyValue, bool) {
	if items == nil {
		return nil, false
	}
	slice := reflect.ValueOf(items)
	if slice.Kind() != reflect.Slice || slice.Len() == 0 {
		return nil, false
	}
	properties := make([]propertyValue, 0, slice.Len())
	for index := 0; index < slice.Len(); index++ {
		item, ok := slice.Index(index).Interface().(map[string]any)
		if !ok || len(item) != 2 {
			return nil, false
		}
		label, labelPresent := item["property"].(string)
		value, valuePresent := item["value"]
		if !labelPresent || strings.TrimSpace(label) == "" || !valuePresent {
			return nil, false
		}
		properties = append(properties, propertyValue{label: label, value: formatValue(value)})
	}
	return properties, true
}

func calculatePropertyLayout(properties []propertyValue, outputWidth int, widthAvailable bool) propertyLayout {
	layout := propertyLayout{outputWidth: outputWidth}
	if !widthAvailable || outputWidth < minimumPropertyLabelWidth+propertyColumnGap+minimumPropertyValueWidth {
		return layout
	}

	labelWidth := minimumPropertyLabelWidth
	for _, property := range properties {
		if width := visibleWidth(property.label); width > labelWidth {
			labelWidth = width
		}
	}
	maximumLabelWidth := outputWidth / 3
	if valueBound := outputWidth - propertyColumnGap - minimumPropertyValueWidth; valueBound < maximumLabelWidth {
		maximumLabelWidth = valueBound
	}
	if maximumLabelWidth < minimumPropertyLabelWidth {
		return layout
	}
	if labelWidth > maximumLabelWidth {
		labelWidth = maximumLabelWidth
	}

	layout.labelWidth = labelWidth
	layout.valueWidth = outputWidth - labelWidth - propertyColumnGap
	layout.twoColumn = layout.valueWidth >= minimumPropertyValueWidth
	return layout
}

func renderTwoColumnProperties(
	writer io.Writer,
	properties []propertyValue,
	layout propertyLayout,
	compact bool,
	styler presentationStyler,
) error {
	if !compact {
		heading := paddedVisibleText("PROPERTY", layout.labelWidth) + strings.Repeat(" ", propertyColumnGap) + "VALUE"
		_, _ = fmt.Fprintln(writer, styler.semantic(presentation.StyleInfo, true, heading))
		printTableSeparator(writer, layout.outputWidth, styler)
	}

	for _, property := range properties {
		labelLines := wrapVisibleLines(property.label, layout.labelWidth)
		valueLines := wrapVisibleLines(property.value, layout.valueWidth)
		lineCount := len(labelLines)
		if len(valueLines) > lineCount {
			lineCount = len(valueLines)
		}
		for lineIndex := 0; lineIndex < lineCount; lineIndex++ {
			labelLine := ""
			if lineIndex < len(labelLines) {
				labelLine = labelLines[lineIndex]
			}
			valueLine := ""
			if lineIndex < len(valueLines) {
				valueLine = valueLines[lineIndex]
			}
			styledLabel := labelLine
			if property.emphasized {
				styledLabel = styler.semantic(presentation.StyleEmphasis, false, labelLine)
			}
			_, _ = fmt.Fprintf(writer, "%s%s%s\n",
				paddedStyledText(styledLabel, labelLine, layout.labelWidth),
				strings.Repeat(" ", propertyColumnGap),
				stylePropertyValue(styler, property, valueLine),
			)
		}
	}
	return nil
}

func paddedStyledText(styledValue, plainValue string, width int) string {
	padding := width - visibleWidth(plainValue)
	if padding < 0 {
		padding = 0
	}
	return styledValue + strings.Repeat(" ", padding)
}

func paddedVisibleText(value string, width int) string {
	padding := width - visibleWidth(value)
	if padding < 0 {
		padding = 0
	}
	return value + strings.Repeat(" ", padding)
}

func renderVerticalProperties(
	writer io.Writer,
	properties []propertyValue,
	outputWidth int,
	widthAvailable bool,
	compact bool,
	styler presentationStyler,
) error {
	for index, property := range properties {
		if index > 0 && !compact {
			_, _ = fmt.Fprintln(writer)
		}
		labelWidth := 0
		if widthAvailable {
			labelWidth = outputWidth
		}
		for _, labelLine := range wrapVisibleLines(property.label, labelWidth) {
			role := presentation.StyleInfo
			emphasized := true
			if compact {
				role = presentation.StyleDefault
				emphasized = property.emphasized
			}
			_, _ = fmt.Fprintln(writer, styler.semantic(role, emphasized, labelLine))
		}
		renderVerticalPropertyValue(writer, property, outputWidth, widthAvailable, styler)
	}
	return nil
}

func renderVerticalPropertyValue(
	writer io.Writer,
	property propertyValue,
	outputWidth int,
	widthAvailable bool,
	styler presentationStyler,
) {
	indentWidth := verticalPropertyIndent
	valueWidth := 0
	if widthAvailable {
		if outputWidth <= indentWidth {
			indentWidth = 0
		}
		valueWidth = outputWidth - indentWidth
	}
	indent := strings.Repeat(" ", indentWidth)
	for _, valueLine := range wrapVisibleLines(property.value, valueWidth) {
		_, _ = fmt.Fprintf(writer, "%s%s\n", indent, stylePropertyValue(styler, property, valueLine))
	}
}

func containsPropertyHeadings(properties []propertyValue) bool {
	for _, property := range properties {
		if property.heading {
			return true
		}
	}
	return false
}

func renderPropertyRecords(
	writer io.Writer,
	properties []propertyValue,
	outputWidth int,
	widthAvailable bool,
	styler presentationStyler,
) error {
	records := 0
	for _, property := range properties {
		if property.heading {
			if records > 0 {
				_, _ = fmt.Fprintln(writer)
				if widthAvailable && outputWidth > 0 {
					separatorWidth := outputWidth
					if separatorWidth > 32 {
						separatorWidth = 32
					}
					printTableSeparator(writer, separatorWidth, styler)
					_, _ = fmt.Fprintln(writer)
				}
			}
			for _, line := range wrapVisibleLines(property.label, knownOutputWidth(outputWidth, widthAvailable)) {
				_, _ = fmt.Fprintln(writer, styler.semantic(property.role, true, line))
			}
			records++
			continue
		}
		if records == 0 {
			return fmt.Errorf("property presentation record field appears before its heading")
		}
		_, _ = fmt.Fprintln(writer)
		for _, line := range wrapVisibleLines(property.label, knownOutputWidth(outputWidth, widthAvailable)) {
			_, _ = fmt.Fprintln(writer, styler.semantic(presentation.StyleDefault, true, line))
		}
		renderVerticalPropertyValue(writer, property, outputWidth, widthAvailable, styler)
	}
	return nil
}

func knownOutputWidth(outputWidth int, available bool) int {
	if !available {
		return 0
	}
	return outputWidth
}

func stylePropertyValue(styler presentationStyler, property propertyValue, value string) string {
	if property.role == "" && !property.emphasized {
		return colorizeValue(styler, "value", value)
	}
	return styler.semantic(property.role, property.emphasized, value)
}
