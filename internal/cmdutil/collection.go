package cmdutil

import "github.com/aarondpn/redmine-cli/internal/output"

// RowBuilder builds one output row for an item. When styled is true, callers
// may apply ANSI/color formatting suitable for table output.
type RowBuilder[T any] func(item T, styled bool) []string

// RenderCollection renders a simple collection in JSON, CSV, or table form.
// It is intended for list-style commands whose formats differ only in row
// styling, not in schema.
func RenderCollection[T any](printer output.Printer, items []T, headers []string, rowBuilder RowBuilder[T]) {
	switch printer.Format() {
	case output.FormatJSON:
		printer.JSON(items)
	case output.FormatCSV:
		printer.CSV(headers, buildRows(items, rowBuilder, false))
	default:
		printer.Table(headers, buildRows(items, rowBuilder, true))
	}
}

func buildRows[T any](items []T, rowBuilder RowBuilder[T], styled bool) [][]string {
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = rowBuilder(item, styled)
	}
	return rows
}
