package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// RenderJSON renders a value as pretty-printed JSON to the writer.
func RenderJSON(w io.Writer, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.IsValid() && rv.Kind() == reflect.Slice && rv.IsNil() {
		v = reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
