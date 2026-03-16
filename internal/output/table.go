package output

import (
	"io"

	"github.com/pterm/pterm"
)

// RenderTable renders a table with headers and rows to the writer.
func RenderTable(w io.Writer, headers []string, rows [][]string, noColor bool) {
	if noColor {
		pterm.DisableColor()
		defer pterm.EnableColor()
	}

	tableData := pterm.TableData{headers}
	for _, row := range rows {
		tableData = append(tableData, row)
	}

	table := pterm.DefaultTable.WithHasHeader().WithData(tableData).WithWriter(w)
	table.Render()
}
