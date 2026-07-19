package renderer

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

type responsiveColumn struct {
	key       string
	label     string
	roleKey   string
	essential bool
	width     int
}

func renderResponsiveTable(
	response *plugin.Response,
	writer io.Writer,
	wide bool,
	widthProvider OutputWidthProvider,
	styler presentationStyler,
) (bool, error) {
	if response.PresentationTable == nil {
		return false, nil
	}
	columns, ok := responsiveColumns(response.PresentationTable)
	if !ok {
		return true, fmt.Errorf("table presentation declaration is invalid")
	}
	items := any(response.PresentationTable.Rows)
	if response.PresentationTable.Rows == nil {
		items = findListInData(response.Data)
	}
	if items == nil {
		return false, nil
	}
	rows, ok := responsiveRows(items, columns)
	if !ok {
		return true, fmt.Errorf("table presentation rows do not satisfy the declaration")
	}
	if len(rows) == 0 {
		_, _ = fmt.Fprintln(writer, styler.semantic(presentation.StyleMuted, false, "No resources found."))
		return true, nil
	}

	outputWidth, available := widthProvider.Width(writer)
	if response.PresentationTable.Title != "" {
		printPresentationTitle(writer, response.PresentationTable.Title, styler)
		_, _ = fmt.Fprintln(writer)
	}
	if !available || outputWidth <= 0 {
		return true, renderVerticalRecords(writer, columns, rows, 0, false, styler)
	}

	columns = calculateResponsiveColumnWidths(columns, rows, outputWidth)
	selected, tableFits := selectResponsiveColumns(columns, outputWidth, wide)
	if !tableFits {
		return true, renderVerticalRecords(writer, columns, rows, outputWidth, true, styler)
	}
	return true, renderResponsiveRows(writer, selected, rows, styler)
}

func responsiveColumns(table *presentation.Table) ([]responsiveColumn, bool) {
	if table == nil || len(table.Columns) == 0 {
		return nil, false
	}
	if strings.TrimSpace(table.Title) != table.Title {
		return nil, false
	}
	columns := make([]responsiveColumn, 0, len(table.Columns))
	seen := make(map[string]struct{}, len(table.Columns))
	for _, declaration := range table.Columns {
		key := strings.TrimSpace(declaration.Key)
		label := strings.TrimSpace(declaration.Label)
		roleKey := strings.TrimSpace(declaration.RoleKey)
		if key == "" || label == "" || roleKey != declaration.RoleKey {
			return nil, false
		}
		if _, exists := seen[key]; exists {
			return nil, false
		}
		seen[key] = struct{}{}
		columns = append(columns, responsiveColumn{
			key: key, label: label, roleKey: roleKey, essential: declaration.Essential,
		})
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
			if column.roleKey != "" {
				role, present := item[column.roleKey].(string)
				if !present || !validStyleRole(presentation.StyleRole(role)) {
					return nil, false
				}
				row[column.roleKey] = role
			}
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

func renderResponsiveRows(writer io.Writer, columns []responsiveColumn, rows []map[string]string, styler presentationStyler) error {
	var heading strings.Builder
	for _, column := range columns {
		_, _ = fmt.Fprintf(&heading, "%s%s", column.label, strings.Repeat(" ", column.width-visibleWidth(column.label)))
	}
	_, _ = fmt.Fprintln(writer, styler.semantic(presentation.StyleInfo, true, heading.String()))

	width := 0
	for _, column := range columns {
		width += column.width
	}
	printTableSeparator(writer, width, styler)

	for _, row := range rows {
		for _, column := range columns {
			value := row[column.key]
			contentWidth := column.width - 2
			if !column.essential && visibleWidth(value) > contentWidth {
				value = truncateVisible(value, contentWidth)
			}
			padding := column.width - visibleWidth(value)
			styledValue := styleResponsiveValue(styler, column, row, value)
			_, _ = fmt.Fprintf(writer, "%s%s", styledValue, strings.Repeat(" ", padding))
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
	styler presentationStyler,
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
			_, _ = fmt.Fprintln(writer, styler.semantic(presentation.StyleEmphasis, false, heading))
		}
		for _, column := range columns {
			renderVerticalField(writer, column, row, outputWidth, widthAvailable, styler)
		}
	}
	return nil
}

func renderVerticalField(
	writer io.Writer,
	column responsiveColumn,
	row map[string]string,
	outputWidth int,
	widthAvailable bool,
	styler presentationStyler,
) {
	value := row[column.key]
	prefix := column.label + ": "
	if !widthAvailable || visibleWidth(prefix)+visibleWidth(value) <= outputWidth {
		_, _ = fmt.Fprintf(writer, "%s %s\n",
			styler.semantic(presentation.StyleInfo, false, column.label+":"),
			styleResponsiveValue(styler, column, row, value),
		)
		return
	}

	prefixWidth := visibleWidth(prefix)
	valueWidth := outputWidth - prefixWidth
	if valueWidth < 4 {
		_, _ = fmt.Fprintln(writer, styler.semantic(presentation.StyleInfo, false, column.label+":"))
		indentWidth := 2
		if outputWidth <= indentWidth {
			indentWidth = 0
		}
		writeWrappedVerticalValue(writer, column, row, value, strings.Repeat(" ", indentWidth), outputWidth-indentWidth, styler)
		return
	}
	writeWrappedVerticalValue(writer, column, row, value, prefix, valueWidth, styler)
}

func writeWrappedVerticalValue(
	writer io.Writer,
	column responsiveColumn,
	row map[string]string,
	value string,
	prefix string,
	valueWidth int,
	styler presentationStyler,
) {
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
			_, _ = fmt.Fprintf(writer, "%s %s\n",
				styler.semantic(presentation.StyleInfo, false, label+":"),
				styleResponsiveValue(styler, column, row, line),
			)
			continue
		}
		_, _ = fmt.Fprintf(writer, "%s%s\n", linePrefix, styleResponsiveValue(styler, column, row, line))
	}
}

func styleResponsiveValue(
	styler presentationStyler,
	column responsiveColumn,
	row map[string]string,
	value string,
) string {
	if column.roleKey == "" {
		return colorizeValue(styler, column.key, value)
	}
	return styler.semantic(presentation.StyleRole(row[column.roleKey]), false, value)
}
