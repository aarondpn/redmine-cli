package mcpserver

import (
	"reflect"
	"strings"
)

// secretJSONFields lists the JSON keys that must never reach an MCP client.
// Redmine returns the caller's API key as "api_key" on /my/account.json (and,
// depending on version and permissions, on the /users endpoints); the CLI
// shows it to its owner, but over MCP it would hand the server's long-lived
// credential to the host and anything that can prompt it.
var secretJSONFields = map[string]bool{
	"api_key": true,
}

// redactSecrets clears every secret field reachable from v in place. v must
// be a pointer; anything else is left untouched. It is the last step before
// a tool result or resource body is serialized, and the only place MCP output
// is redacted: the ops functions return users unchanged because the CLI shows
// the key to its owner, so every tool and resource, including ones added
// later, is covered here.
//
// String and *string struct fields whose JSON name is in secretJSONFields are
// zeroed (they are all omitempty, so the key disappears from the output), and
// matching keys are deleted from string-keyed maps. Pointers, interfaces,
// slices, arrays, maps and nested structs are walked recursively.
func redactSecrets(v any) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return
	}
	redactValue(rv.Elem(), make(map[visitKey]bool))
}

// visitKey identifies a pointer target already walked. The type is part of
// the key because a struct and its first field share an address: keying by
// address alone would skip the struct after its first field was visited.
type visitKey struct {
	ptr uintptr
	typ reflect.Type
}

func redactValue(v reflect.Value, seen map[visitKey]bool) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		key := visitKey{v.Pointer(), v.Type()}
		if seen[key] {
			return
		}
		seen[key] = true
		redactValue(v.Elem(), seen)
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		elem := v.Elem()
		if elem.Kind() == reflect.Pointer {
			redactValue(elem, seen)
			return
		}
		if !v.CanSet() || !mayHoldSecrets(elem.Type()) {
			return
		}
		// Values stored in an interface are not addressable; redact a copy
		// and store it back.
		cp := reflect.New(elem.Type()).Elem()
		cp.Set(elem)
		redactValue(cp, seen)
		v.Set(cp)
	case reflect.Struct:
		t := v.Type()
		for i := range v.NumField() {
			sf := t.Field(i)
			if !sf.IsExported() {
				continue
			}
			f := v.Field(i)
			if secretJSONFields[jsonFieldName(sf)] && f.CanSet() {
				switch {
				case f.Kind() == reflect.String:
					f.SetString("")
					continue
				case f.Kind() == reflect.Pointer && f.Type().Elem().Kind() == reflect.String:
					f.SetZero()
					continue
				}
			}
			redactValue(f, seen)
		}
	case reflect.Slice, reflect.Array:
		if !mayHoldSecrets(v.Type().Elem()) {
			return
		}
		for i := range v.Len() {
			redactValue(v.Index(i), seen)
		}
	case reflect.Map:
		if v.IsNil() {
			return
		}
		keyIsString := v.Type().Key().Kind() == reflect.String
		walkValues := mayHoldSecrets(v.Type().Elem())
		if !keyIsString && !walkValues {
			return
		}
		for _, key := range v.MapKeys() {
			if keyIsString && secretJSONFields[key.String()] {
				v.SetMapIndex(key, reflect.Value{})
				continue
			}
			if !walkValues {
				continue
			}
			// Map values are not addressable either.
			cp := reflect.New(v.Type().Elem()).Elem()
			cp.Set(v.MapIndex(key))
			redactValue(cp, seen)
			v.SetMapIndex(key, cp)
		}
	}
}

// mayHoldSecrets reports whether a value of type t can (transitively) contain
// a string field or map entry that redactValue would clear. Scalars and
// strings cannot, so slices of them (e.g. []byte, []string) are skipped.
func mayHoldSecrets(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Struct, reflect.Slice, reflect.Array, reflect.Map:
		return true
	}
	return false
}

// jsonFieldName returns the name encoding/json uses for sf.
func jsonFieldName(sf reflect.StructField) string {
	tag := sf.Tag.Get("json")
	if name, _, _ := strings.Cut(tag, ","); name != "" && name != "-" {
		return name
	}
	return sf.Name
}
