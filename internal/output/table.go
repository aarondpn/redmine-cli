package output

import (
	"io"

	"github.com/pterm/pterm"
)

// RenderTable renders a table with headers and rows to the writer. Cells are
// sanitized with SanitizeTerminal unless they are the output of Style.Render.
func RenderTable(w io.Writer, headers []string, rows [][]string, noColor bool) {
	if noColor {
		pterm.DisableColor()
		defer pterm.EnableColor()
	}

	tableData := make(pterm.TableData, 0, len(rows)+1)
	tableData = append(tableData, sanitizeRow(headers))
	for _, row := range rows {
		tableData = append(tableData, sanitizeRow(row))
	}

	table := pterm.DefaultTable.WithHasHeader().WithData(tableData).WithWriter(w)
	table.Render()
}

func sanitizeRow(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		out[i] = sanitizeValue(cell)
	}
	return out
}
