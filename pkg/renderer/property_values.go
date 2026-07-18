package renderer

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

const (
	propertyColumnGap         = 2
	minimumPropertyLabelWidth = 8
	minimumPropertyValueWidth = 20
	verticalPropertyIndent    = 2
)

type propertyValue struct {
	label string
	value string
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
) (bool, error) {
	properties, declared, err := responsePropertyValues(response)
	if err != nil {
		return declared, err
	}
	if !declared {
		return false, nil
	}
	if len(properties) == 0 {
		_, _ = fmt.Fprintf(writer, "%sNo resources found.%s\n", log.ColorBrightBlack, log.ColorReset)
		return true, nil
	}

	outputWidth, widthAvailable := widthProvider.Width(writer)
	layout := calculatePropertyLayout(properties, outputWidth, widthAvailable)
	if layout.twoColumn {
		return true, renderTwoColumnProperties(writer, properties, layout)
	}
	return true, renderVerticalProperties(writer, properties, layout.outputWidth, widthAvailable)
}

func responsePropertyValues(response *plugin.Response) ([]propertyValue, bool, error) {
	if response.HumanProperties != nil {
		properties, err := declaredPropertyValues(response)
		return properties, true, err
	}
	properties, ok := itemPropertyValues(findListInData(response.Data))
	return properties, ok, nil
}

func declaredPropertyValues(response *plugin.Response) ([]propertyValue, error) {
	declarations := response.HumanProperties.Properties
	if len(declarations) == 0 {
		return nil, fmt.Errorf("human property declaration is empty")
	}
	properties := make([]propertyValue, 0, len(declarations))
	seenKeys := make(map[string]struct{}, len(declarations))
	for _, declaration := range declarations {
		key := strings.TrimSpace(declaration.Key)
		label := strings.TrimSpace(declaration.Label)
		hasValue := declaration.Value != nil
		if label == "" || label != declaration.Label || key != declaration.Key || (key == "") == !hasValue {
			return nil, fmt.Errorf("human property declaration must contain one label and exactly one data key or value")
		}

		value := declaration.Value
		if key != "" {
			if _, duplicate := seenKeys[key]; duplicate {
				return nil, fmt.Errorf("human property declaration repeats data key %q", key)
			}
			var present bool
			value, present = response.Data[key]
			if !present {
				return nil, fmt.Errorf("human property data key %q is missing", key)
			}
			seenKeys[key] = struct{}{}
		}
		properties = append(properties, propertyValue{label: label, value: formatValue(value)})
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

func renderTwoColumnProperties(writer io.Writer, properties []propertyValue, layout propertyLayout) error {
	_, _ = fmt.Fprintf(writer, "%s%s%s%sVALUE%s\n",
		log.ColorCyan,
		log.ColorBold,
		paddedVisibleText("PROPERTY", layout.labelWidth),
		strings.Repeat(" ", propertyColumnGap),
		log.ColorReset,
	)
	log.PrintTableSeparatorTo(writer, layout.outputWidth)

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
			_, _ = fmt.Fprintf(writer, "%s%s%s\n",
				paddedVisibleText(labelLine, layout.labelWidth),
				strings.Repeat(" ", propertyColumnGap),
				colorizeValue("value", valueLine),
			)
		}
	}
	return nil
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
) error {
	for index, property := range properties {
		if index > 0 {
			_, _ = fmt.Fprintln(writer)
		}
		labelWidth := 0
		if widthAvailable {
			labelWidth = outputWidth
		}
		for _, labelLine := range wrapVisibleLines(property.label, labelWidth) {
			_, _ = fmt.Fprintf(writer, "%s%s%s%s\n", log.ColorCyan, log.ColorBold, labelLine, log.ColorReset)
		}
		renderVerticalPropertyValue(writer, property.value, outputWidth, widthAvailable)
	}
	return nil
}

func renderVerticalPropertyValue(writer io.Writer, value string, outputWidth int, widthAvailable bool) {
	indentWidth := verticalPropertyIndent
	valueWidth := 0
	if widthAvailable {
		if outputWidth <= indentWidth {
			indentWidth = 0
		}
		valueWidth = outputWidth - indentWidth
	}
	indent := strings.Repeat(" ", indentWidth)
	for _, valueLine := range wrapVisibleLines(value, valueWidth) {
		_, _ = fmt.Fprintf(writer, "%s%s\n", indent, colorizeValue("value", valueLine))
	}
}
