package output

import (
	"encoding/csv"
	"io"
	"regexp"
)

// CSVFormulaWarning is the warning printed when a CSV export contains cells
// that spreadsheet applications may evaluate as formulas.
const CSVFormulaWarning = "CSV output contains cells starting with =, +, -, @, tab or carriage return that spreadsheet apps may evaluate as formulas; check the data before opening it in a spreadsheet"

// plainNumber matches cells such as "-1.5" or "+3" that start with a formula
// trigger but are ordinary numbers, which spreadsheets import as values.
var plainNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

// RenderCSV renders headers and rows as CSV to the writer.
func RenderCSV(w io.Writer, headers []string, rows [][]string) error {
	writer := csv.NewWriter(w)
	if err := writer.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

// IsFormulaCell reports whether a spreadsheet application may interpret the
// cell as a formula: it starts with =, +, -, @, tab or carriage return and is
// not a plain number.
func IsFormulaCell(cell string) bool {
	if cell == "" {
		return false
	}
	switch cell[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return !plainNumber.MatchString(cell)
	}
	return false
}

// HasFormulaCells reports whether any cell in rows is a formula cell.
func HasFormulaCells(rows [][]string) bool {
	for _, row := range rows {
		for _, cell := range row {
			if IsFormulaCell(cell) {
				return true
			}
		}
	}
	return false
}
