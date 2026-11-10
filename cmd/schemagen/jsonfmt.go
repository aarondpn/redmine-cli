package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// reindent pretty-prints JSON with two-space indentation, keeping the key
// order of data except that the top-level keys in first move to the front.
// encoding/json cannot do either on its own: maps lose order and the
// encoder HTML-escapes strings produced by nested MarshalJSON calls.
func reindent(data []byte, first []string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	root, err := readOrdered(dec)
	if err != nil {
		return nil, err
	}
	if obj, ok := root.(*orderedObject); ok {
		obj.moveToFront(first)
	}
	var b bytes.Buffer
	if err := writeOrdered(&b, root, ""); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

type orderedObject struct {
	keys []string
	vals []any
}

func (o *orderedObject) moveToFront(first []string) {
	var keys []string
	var vals []any
	for _, k := range first {
		if i := slices.Index(o.keys, k); i >= 0 {
			keys = append(keys, k)
			vals = append(vals, o.vals[i])
		}
	}
	for i, k := range o.keys {
		if !slices.Contains(first, k) {
			keys = append(keys, k)
			vals = append(vals, o.vals[i])
		}
	}
	o.keys, o.vals = keys, vals
}

func readOrdered(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := &orderedObject{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := kt.(string)
				if !ok {
					return nil, fmt.Errorf("unexpected token %v", kt)
				}
				v, err := readOrdered(dec)
				if err != nil {
					return nil, err
				}
				obj.keys = append(obj.keys, key)
				obj.vals = append(obj.vals, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return obj, nil
		case '[':
			list := []any{}
			for dec.More() {
				v, err := readOrdered(dec)
				if err != nil {
					return nil, err
				}
				list = append(list, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return list, nil
		}
		return nil, fmt.Errorf("unexpected delimiter %v", t)
	default:
		return tok, nil
	}
}

func writeOrdered(w *bytes.Buffer, v any, indent string) error {
	inner := indent + "  "
	switch t := v.(type) {
	case *orderedObject:
		if len(t.keys) == 0 {
			w.WriteString("{}")
			return nil
		}
		w.WriteString("{\n")
		for i, k := range t.keys {
			w.WriteString(inner)
			if err := writeScalar(w, k); err != nil {
				return err
			}
			w.WriteString(": ")
			if err := writeOrdered(w, t.vals[i], inner); err != nil {
				return err
			}
			if i < len(t.keys)-1 {
				w.WriteByte(',')
			}
			w.WriteByte('\n')
		}
		w.WriteString(indent + "}")
	case []any:
		if len(t) == 0 {
			w.WriteString("[]")
			return nil
		}
		w.WriteString("[\n")
		for i, item := range t {
			w.WriteString(inner)
			if err := writeOrdered(w, item, inner); err != nil {
				return err
			}
			if i < len(t)-1 {
				w.WriteByte(',')
			}
			w.WriteByte('\n')
		}
		w.WriteString(indent + "]")
	default:
		return writeScalar(w, t)
	}
	return nil
}

func writeScalar(w io.Writer, v any) error {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := io.WriteString(w, strings.TrimSuffix(b.String(), "\n"))
	return err
}
