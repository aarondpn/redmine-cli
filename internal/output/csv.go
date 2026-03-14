package output

import (
	"encoding/csv"
	"io"
)

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
