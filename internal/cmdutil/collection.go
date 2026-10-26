package cmdutil

import "github.com/aarondpn/redmine-cli/v2/internal/output"

// RowBuilder builds one table row for an item; it may apply ANSI/color
// formatting.
type RowBuilder[T any] func(item T) []string

// RenderCollection renders a simple collection in JSON or table form. It is
// intended for list-style commands whose table rows map one-to-one to items.
func RenderCollection[T any](printer output.Printer, items []T, headers []string, rowBuilder RowBuilder[T]) {
	if printer.Format() == output.FormatJSON {
		printer.JSON(items)
		return
	}
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = rowBuilder(item)
	}
	printer.Table(headers, rows)
}
