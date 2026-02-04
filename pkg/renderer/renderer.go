package renderer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

// OutputFormat defines the supported output formats
type OutputFormat string

const (
	FormatTable OutputFormat = "table" // kubectl-style with Colors (default)
	FormatJSON  OutputFormat = "json"  // Raw JSON output
	FormatWide  OutputFormat = "wide"  // Extended table with more columns if available
)

type RenderOptions struct {
	Format   OutputFormat
	Describe bool // when true, include logs and metadata
}

// RenderWithOptions is the new unified render function
func RenderWithOptions(resp *plugin.Response, opts RenderOptions) error {
	if opts.Describe {
		return RenderDescribe(resp, opts.Format)
	}
	return Render(resp, opts.Format)
}

// Render is the main entry point to render a plugin response to STDOUT
// --output format is controlled via the format parameter
// Supported formats: table (default), json, wide
func Render(resp *plugin.Response, format OutputFormat) error {
	return RenderTo(resp, format, os.Stdout)
}

// RenderTo renders the plugin response to the given writer
func RenderTo(resp *plugin.Response, format OutputFormat, w io.Writer) error {
	switch format {
	case FormatJSON:
		return renderJSON(resp, w)
	case FormatWide:
		return renderTable(resp, w, true)
	case FormatTable:
		return renderTable(resp, w, false)
	default:
		return renderTable(resp, w, false)
	}
}

// RenderDescribe renders both execution logs and command output
func RenderDescribe(resp *plugin.Response, format OutputFormat) error {
	return RenderDescribeTo(resp, format, os.Stdout)
}

// RenderDescribeTo renders describe output to the given writer
func RenderDescribeTo(resp *plugin.Response, format OutputFormat, w io.Writer) error {
	if format == FormatJSON {
		// JSON format includes everything
		return renderJSON(resp, w)
	}

	// Render metadata section
	renderMetadataSection(resp, w)

	// Render execution logs
	if len(resp.Logs) > 0 {
		renderLogsSection(resp.Logs, w)
	}

	// Render output data
	renderOutputSection(resp, format, w)

	return nil
}

func renderMetadataSection(resp *plugin.Response, w io.Writer) {
	log.PrintSectionHeaderTo(w, "Command Metadata", log.ColorCyan)

	_, _ = fmt.Fprintf(w, "%sPlugin:%s     %s\n",
		log.ColorBrightBlack, log.ColorReset, resp.Metadata.Plugin)
	_, _ = fmt.Fprintf(w, "%sCommand:%s    %s\n",
		log.ColorBrightBlack, log.ColorReset, resp.Metadata.Command)
	_, _ = fmt.Fprintf(w, "%sVersion:%s    %s\n",
		log.ColorBrightBlack, log.ColorReset, resp.Metadata.Version)
	_, _ = fmt.Fprintf(w, "%sTimestamp:%s  %s\n",
		log.ColorBrightBlack, log.ColorReset, resp.Metadata.Timestamp.Format("2006-01-02 15:04:05"))
	_, _ = fmt.Fprintf(w, "%sStatus:%s     %s\n",
		log.ColorBrightBlack, log.ColorReset, colorizeStatus(resp.Status))
	_, _ = fmt.Fprintln(w)
}

func renderLogsSection(logs []plugin.LogEntry, w io.Writer) {
	log.PrintSectionHeaderTo(w, fmt.Sprintf("Execution Logs (%d entries)", len(logs)), log.ColorYellow)

	for _, entry := range logs {
		levelColor := getLogLevelColor(entry.Level)
		levelIcon := getLogLevelIcon(entry.Level)

		categoryStr := ""
		if entry.Category != "" && entry.Category != "plugin" {
			categoryStr = fmt.Sprintf("[%s] ", entry.Category)
		}

		_, _ = fmt.Fprintf(w, "%s%s %s%s%s%s\n",
			log.ColorBrightBlack, entry.Timestamp,
			levelColor, levelIcon, categoryStr,
			log.ColorReset+entry.Message)
	}
	_, _ = fmt.Fprintln(w)
}

func renderOutputSection(resp *plugin.Response, format OutputFormat, w io.Writer) {
	log.PrintSectionHeaderTo(w, "Output", log.ColorGreen)

	wide := format == FormatWide
	_ = renderTable(resp, w, wide)
}

func colorizeStatus(status string) string {
	switch strings.ToLower(status) {
	case "success":
		return log.ColorText(log.ColorGreen, "✓ "+status)
	case "error":
		return log.ColorText(log.ColorRed, "✗ "+status)
	default:
		return status
	}
}

func getLogLevelColor(level string) string {
	switch level {
	case "error":
		return log.ColorRed
	case "warn":
		return log.ColorYellow
	case "verbose":
		return log.ColorPurple
	default:
		return log.ColorBrightBlack
	}
}

func getLogLevelIcon(level string) string {
	switch level {
	case "error":
		return "✗ "
	case "warn":
		return "⚠ "
	case "verbose":
		return "V$ "
	default:
		return "• "
	}
}

// renderJSON - raw JSON output
func renderJSON(resp *plugin.Response, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resp)
}

// renderTable - unified kubectl-style output
// Automatically detects lists (any slice in data) and renders as table
// Single objects are rendered as key-value pairs
func renderTable(resp *plugin.Response, w io.Writer, wide bool) error {
	_ = wide // TODO: implement wide output format with additional columns

	if resp.Status == "error" {
		return renderError(resp, w)
	}

	// Find any list in the data (items, releases, pods, etc.)
	listData := findListInData(resp.Data)
	if listData != nil {
		return renderList(listData, w)
	}

	// Single object or key-value data
	return renderKeyValue(resp.Data, w)
}

// findListInData searches for any slice/array in the data map
// Returns the first list found, prioritizing common names like "items"
func findListInData(data map[string]any) any {
	if data == nil {
		return nil
	}

	// Priority keys for lists
	priorityKeys := []string{"items", "releases", "resources", "results", "data", "list"}

	for _, key := range priorityKeys {
		if val, ok := data[key]; ok {
			if reflect.TypeOf(val) != nil && reflect.TypeOf(val).Kind() == reflect.Slice {
				return val
			}
		}
	}

	// Fallback: find any slice in data
	for _, val := range data {
		if val != nil && reflect.TypeOf(val) != nil && reflect.TypeOf(val).Kind() == reflect.Slice {
			return val
		}
	}

	return nil
}

func renderError(resp *plugin.Response, w io.Writer) error {
	_, _ = fmt.Fprintf(w, "%s%s✗ ERROR%s\n", log.ColorRed, log.ColorBold, log.ColorReset)
	_, _ = fmt.Fprintf(w, "%sCode:%s    %s\n", log.ColorBrightBlack, log.ColorReset, resp.Error.Code)
	_, _ = fmt.Fprintf(w, "%sMessage:%s %s\n", log.ColorBrightBlack, log.ColorReset, resp.Error.Message)

	if len(resp.Error.Details) > 0 {
		_, _ = fmt.Fprintf(w, "\n%sDetails:%s\n", log.ColorBrightBlack, log.ColorReset)
		for k, v := range resp.Error.Details {
			_, _ = fmt.Fprintf(w, "  %s%s:%s %v\n", log.ColorCyan, k, log.ColorReset, v)
		}
	}
	return nil
}

func renderList(items any, w io.Writer) error {
	slice := reflect.ValueOf(items)
	if slice.Kind() != reflect.Slice {
		return renderKeyValue(map[string]any{"items": items}, w)
	}

	if slice.Len() == 0 {
		_, _ = fmt.Fprintf(w, "%sNo resources found.%s\n", log.ColorBrightBlack, log.ColorReset)
		return nil
	}

	// Extract all keys from the first item to build headers
	headers, rows := extractTableData(slice)

	if len(headers) == 0 {
		// Fallback for non-map items
		for i := 0; i < slice.Len(); i++ {
			_, _ = fmt.Fprintf(w, "%v\n", slice.Index(i).Interface())
		}
		return nil
	}

	// Calculate column widths
	colWidths := calculateColumnWidths(headers, rows)

	// Print header
	printHeader(w, headers, colWidths)

	// Print separator
	log.PrintTableSeparatorTo(w, totalWidth(colWidths))

	// Print rows
	for _, row := range rows {
		printRow(w, headers, row, colWidths)
	}

	return nil
}

func extractTableData(slice reflect.Value) ([]string, []map[string]string) {
	var headers []string
	headerSet := make(map[string]bool)
	var rows []map[string]string

	// First pass: collect all headers
	for i := 0; i < slice.Len(); i++ {
		item := slice.Index(i).Interface()
		if m, ok := item.(map[string]any); ok {
			for k := range m {
				if !headerSet[k] {
					headerSet[k] = true
					headers = append(headers, k)
				}
			}
		}
	}

	// Sort headers for consistent output
	sort.Strings(headers)

	// Prioritize common fields first
	headers = prioritizeHeaders(headers)

	// Second pass: extract row data
	for i := 0; i < slice.Len(); i++ {
		item := slice.Index(i).Interface()
		row := make(map[string]string)
		if m, ok := item.(map[string]any); ok {
			for _, h := range headers {
				row[h] = formatValue(m[h])
			}
		}
		rows = append(rows, row)
	}

	return headers, rows
}

func prioritizeHeaders(headers []string) []string {
	priority := map[string]int{
		"name": 1, "NAME": 1,
		"id": 2, "ID": 2,
		"metadata": 3, "VERSION": 3,
		"status": 4, "STATUS": 4,
		"type": 5, "TYPE": 5,
		"created": 6, "CREATED": 6,
		"age": 7, "AGE": 7,
	}

	sort.Slice(headers, func(i, j int) bool {
		pi, oki := priority[headers[i]]
		pj, okj := priority[headers[j]]
		if oki && okj {
			return pi < pj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return headers[i] < headers[j]
	})

	return headers
}

func calculateColumnWidths(headers []string, rows []map[string]string) map[string]int {
	widths := make(map[string]int)

	// Start with header lengths
	for _, h := range headers {
		widths[h] = len(strings.ToUpper(h))
	}

	// Check row values
	for _, row := range rows {
		for _, h := range headers {
			if len(row[h]) > widths[h] {
				widths[h] = len(row[h])
			}
		}
	}

	// Add padding
	for k := range widths {
		widths[k] += 2
	}

	return widths
}

func totalWidth(colWidths map[string]int) int {
	total := 0
	for _, w := range colWidths {
		total += w
	}
	return total
}

func printHeader(w io.Writer, headers []string, widths map[string]int) {
	_, _ = fmt.Fprintf(w, "%s%s", log.ColorCyan, log.ColorBold)
	for _, h := range headers {
		_, _ = fmt.Fprintf(w, "%-*s", widths[h], strings.ToUpper(h))
	}
	_, _ = fmt.Fprintf(w, "%s\n", log.ColorReset)
}

func printRow(w io.Writer, headers []string, row map[string]string, widths map[string]int) {
	for _, h := range headers {
		value := row[h]
		coloredValue := colorizeValue(h, value)

		// Calculate visible length (without ANSI codes)
		visibleLen := len(value)
		padding := widths[h] - visibleLen

		_, _ = fmt.Fprintf(w, "%s%s", coloredValue, strings.Repeat(" ", padding))
	}
	_, _ = fmt.Fprintln(w)
}

func colorizeValue(key, value string) string {
	keyLower := strings.ToLower(key)
	valueLower := strings.ToLower(value)

	// Status-based coloring
	if keyLower == "status" || keyLower == "state" {
		switch valueLower {
		case "success", "running", "active", "ready", "healthy", "ok":
			return log.ColorText(log.ColorGreen, value)
		case "error", "failed", "terminated", "unhealthy":
			return log.ColorText(log.ColorRed, value)
		case "pending", "waiting", "unknown":
			return log.ColorText(log.ColorYellow, value)
		}
	}

	// Version coloring
	if keyLower == "metadata" || strings.HasPrefix(value, "v") {
		return log.ColorText(log.ColorPurple, value)
	}

	// Boolean coloring
	if valueLower == "true" || valueLower == "yes" {
		return log.ColorText(log.ColorGreen, value)
	}
	if valueLower == "false" || valueLower == "no" {
		return log.ColorText(log.ColorRed, value)
	}

	// Name highlighting
	if keyLower == "name" || keyLower == "id" {
		return log.ColorText(log.ColorBrightWhite, value)
	}

	return value
}

func formatValue(v any) string {
	if v == nil {
		return "<none>"
	}

	switch val := v.(type) {
	case string:
		if val == "" {
			return "<none>"
		}
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%.2f", val)
	case []any:
		if len(val) == 0 {
			return "<none>"
		}
		// Join array values
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatValue(item)
		}
		return strings.Join(parts, ",")
	case map[string]any:
		// Nested object - show as condensed
		parts := make([]string, 0, len(val))
		for k, v := range val {
			parts = append(parts, fmt.Sprintf("%s=%s", k, formatValue(v)))
		}
		sort.Strings(parts)
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func renderKeyValue(data map[string]any, w io.Writer) error {
	if len(data) == 0 {
		_, _ = fmt.Fprintf(w, "%sNo data.%s\n", log.ColorBrightBlack, log.ColorReset)
		return nil
	}

	// Get sorted keys
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Find max key length for alignment
	maxKeyLen := 0
	for _, k := range keys {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	}

	// Print key-value pairs
	for _, k := range keys {
		v := data[k]
		formattedKey := fmt.Sprintf("%-*s", maxKeyLen, capitalizeFirst(k))
		formattedValue := formatValue(v)
		coloredValue := colorizeValue(k, formattedValue)

		_, _ = fmt.Fprintf(w, "%s%s:%s  %s\n",
			log.ColorCyan, formattedKey, log.ColorReset, coloredValue)
	}

	return nil
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
