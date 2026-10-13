package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// RenderJSON renders a value as pretty-printed JSON to the writer.
func RenderJSON(w io.Writer, v interface{}) error {
	data, err := json.MarshalIndent(normalizeJSONValue(v), "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

// normalizeJSONValue replaces a nil slice with an empty one so empty
// collections encode as [] rather than null.
func normalizeJSONValue(v any) any {
	rv := reflect.ValueOf(v)
	if rv.IsValid() && rv.Kind() == reflect.Slice && rv.IsNil() {
		return reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}
	return v
}
