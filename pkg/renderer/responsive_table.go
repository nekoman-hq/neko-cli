package renderer

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

type responsiveColumn struct {
	key       string
	label     string
	essential bool
	width     int
}

func renderResponsiveTable(
	response *plugin.Response,
	writer io.Writer,
	wide bool,
	widthProvider OutputWidthProvider,
) (bool, error) {
	columns, ok := responsiveColumns(response.HumanTable)
	if !ok {
		return false, nil
	}
	items := any(response.HumanTable.Rows)
	if response.HumanTable.Rows == nil {
		items = findListInData(response.Data)
	}
	if items == nil {
		return false, nil
	}
	rows, ok := responsiveRows(items, columns)
	if !ok {
		return false, nil
	}
	if len(rows) == 0 {
		_, _ = fmt.Fprintf(writer, "%sNo resources found.%s\n", log.ColorBrightBlack, log.ColorReset)
		return true, nil
	}

	outputWidth, available := widthProvider.Width(writer)
	if !available || outputWidth <= 0 {
		return true, renderVerticalRecords(writer, columns, rows, 0, false)
	}

	columns = calculateResponsiveColumnWidths(columns, rows, outputWidth)
	selected, tableFits := selectResponsiveColumns(columns, outputWidth, wide)
	if !tableFits {
		return true, renderVerticalRecords(writer, columns, rows, outputWidth, true)
	}
	return true, renderResponsiveRows(writer, selected, rows)
}

func responsiveColumns(table *plugin.HumanTable) ([]responsiveColumn, bool) {
	if table == nil || len(table.Columns) == 0 {
		return nil, false
	}
	columns := make([]responsiveColumn, 0, len(table.Columns))
	seen := make(map[string]struct{}, len(table.Columns))
	for _, declaration := range table.Columns {
		key := strings.TrimSpace(declaration.Key)
		label := strings.TrimSpace(declaration.Label)
		if key == "" || label == "" {
			return nil, false
		}
		if _, exists := seen[key]; exists {
			return nil, false
		}
		seen[key] = struct{}{}
		columns = append(columns, responsiveColumn{key: key, label: label, essential: declaration.Essential})
	}
	return columns, true
}

func responsiveRows(items any, columns []responsiveColumn) ([]map[string]string, bool) {
	slice := reflect.ValueOf(items)
	if slice.Kind() != reflect.Slice {
		return nil, false
	}
	rows := make([]map[string]string, 0, slice.Len())
	for index := 0; index < slice.Len(); index++ {
		item, ok := slice.Index(index).Interface().(map[string]any)
		if !ok {
			return nil, false
		}
		row := make(map[string]string, len(columns))
		for _, column := range columns {
			row[column.key] = formatValue(item[column.key])
		}
		rows = append(rows, row)
	}
	return rows, true
}

func calculateResponsiveColumnWidths(columns []responsiveColumn, rows []map[string]string, outputWidth int) []responsiveColumn {
	calculated := make([]responsiveColumn, len(columns))
	copy(calculated, columns)
	for index := range calculated {
		column := &calculated[index]
		naturalWidth := visibleWidth(column.label)
		for _, row := range rows {
			if width := visibleWidth(row[column.key]); width > naturalWidth {
				naturalWidth = width
			}
		}
		naturalWidth += 2
		if column.essential {
			column.width = naturalWidth
			continue
		}
		column.width = boundedOptionalColumnWidth(naturalWidth, visibleWidth(column.label)+2, outputWidth)
	}
	return calculated
}

func boundedOptionalColumnWidth(naturalWidth, minimumWidth, outputWidth int) int {
	budget := outputWidth / 3
	if budget < minimumWidth {
		budget = minimumWidth
	}
	if naturalWidth < budget {
		return naturalWidth
	}
	return budget
}

func selectResponsiveColumns(columns []responsiveColumn, outputWidth int, wide bool) ([]responsiveColumn, bool) {
	selectedKeys := make(map[string]struct{}, len(columns))
	total := 0
	for _, column := range columns {
		if !column.essential {
			continue
		}
		selectedKeys[column.key] = struct{}{}
		total += column.width
	}
	if total > outputWidth {
		return nil, false
	}

	for _, column := range columns {
		if column.essential {
			continue
		}
		if total+column.width > outputWidth {
			if wide {
				return nil, false
			}
			break
		}
		selectedKeys[column.key] = struct{}{}
		total += column.width
	}

	selected := make([]responsiveColumn, 0, len(selectedKeys))
	for _, column := range columns {
		if _, ok := selectedKeys[column.key]; ok {
			selected = append(selected, column)
		}
	}
	if len(selected) == 0 {
		return nil, false
	}
	return selected, true
}

func renderResponsiveRows(writer io.Writer, columns []responsiveColumn, rows []map[string]string) error {
	_, _ = fmt.Fprintf(writer, "%s%s", log.ColorCyan, log.ColorBold)
	for _, column := range columns {
		_, _ = fmt.Fprintf(writer, "%s%s", column.label, strings.Repeat(" ", column.width-visibleWidth(column.label)))
	}
	_, _ = fmt.Fprintf(writer, "%s\n", log.ColorReset)

	width := 0
	for _, column := range columns {
		width += column.width
	}
	log.PrintTableSeparatorTo(writer, width)

	for _, row := range rows {
		for _, column := range columns {
			value := row[column.key]
			contentWidth := column.width - 2
			if !column.essential && visibleWidth(value) > contentWidth {
				value = truncateVisible(value, contentWidth)
			}
			padding := column.width - visibleWidth(value)
			_, _ = fmt.Fprintf(writer, "%s%s", colorizeValue(column.key, value), strings.Repeat(" ", padding))
		}
		_, _ = fmt.Fprintln(writer)
	}
	return nil
}

func renderVerticalRecords(
	writer io.Writer,
	columns []responsiveColumn,
	rows []map[string]string,
	outputWidth int,
	widthAvailable bool,
) error {
	for rowIndex, row := range rows {
		if rowIndex > 0 {
			_, _ = fmt.Fprintln(writer)
		}
		if len(rows) > 1 {
			heading := fmt.Sprintf("Record %d", rowIndex+1)
			if widthAvailable && visibleWidth(heading) > outputWidth {
				heading = truncateVisible(heading, outputWidth)
			}
			_, _ = fmt.Fprintf(writer, "%s%s%s%s\n", log.ColorCyan, log.ColorBold, heading, log.ColorReset)
		}
		for _, column := range columns {
			renderVerticalField(writer, column, row[column.key], outputWidth, widthAvailable)
		}
	}
	return nil
}

func renderVerticalField(writer io.Writer, column responsiveColumn, value string, outputWidth int, widthAvailable bool) {
	prefix := column.label + ": "
	if !widthAvailable || visibleWidth(prefix)+visibleWidth(value) <= outputWidth {
		_, _ = fmt.Fprintf(writer, "%s%s:%s %s\n", log.ColorCyan, column.label, log.ColorReset, colorizeValue(column.key, value))
		return
	}

	prefixWidth := visibleWidth(prefix)
	valueWidth := outputWidth - prefixWidth
	if valueWidth < 4 {
		_, _ = fmt.Fprintf(writer, "%s%s:%s\n", log.ColorCyan, column.label, log.ColorReset)
		indentWidth := 2
		if outputWidth <= indentWidth {
			indentWidth = 0
		}
		writeWrappedVerticalValue(writer, column.key, value, strings.Repeat(" ", indentWidth), outputWidth-indentWidth)
		return
	}
	writeWrappedVerticalValue(writer, column.key, value, prefix, valueWidth)
}

func writeWrappedVerticalValue(writer io.Writer, key, value, prefix string, valueWidth int) {
	if valueWidth <= 0 {
		return
	}
	lines := wrapVisibleLines(value, valueWidth)
	continuation := strings.Repeat(" ", visibleWidth(prefix))
	for index, line := range lines {
		linePrefix := continuation
		if index == 0 {
			linePrefix = prefix
		}
		if strings.HasSuffix(prefix, ": ") && index == 0 {
			label := strings.TrimSuffix(prefix, ": ")
			_, _ = fmt.Fprintf(writer, "%s%s:%s %s\n", log.ColorCyan, label, log.ColorReset, colorizeValue(key, line))
			continue
		}
		_, _ = fmt.Fprintf(writer, "%s%s\n", linePrefix, colorizeValue(key, line))
	}
}
