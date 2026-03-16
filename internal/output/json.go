package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// RenderJSON renders a value as pretty-printed JSON to the writer.
func RenderJSON(w io.Writer, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
