package mcpserver

import (
	"reflect"
	"regexp"
	"strings"
	"unicode"
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

// redactedText replaces a secret value in text.
const redactedText = "[REDACTED]"

// secretTextRe finds an API key with its value in free text: JSON
// ("api_key": "..."), JSON quoted inside a string (\"api_key\":\"...\"),
// YAML or env style (api_key: ..., REDMINE_API_KEY=...), a Ruby hash
// (:api_key => "..."), an HTTP header (X-Redmine-API-Key: ...) and the
// CLI's own detail output ("API Key:  ...", possibly colored). Group 1 is
// the key with its separator, groups 2-4 the value: escaped string, JSON
// string or bare word.
var secretTextRe = regexp.MustCompile(`(?i)((?:\b|_|\x1b\[[0-9;]*m)api[ _-]?key\b\\?"?(?:\x1b\[[0-9;]*m)*\s*(?:=>|[:=])(?:\x1b\[[0-9;]*m)*\s*)` +
	`(?:(\\"(?:[^\\]|\\[^"])*?\\")|("(?:[^"\\]|\\.)*")|([^\s,;})\]'"\\<]+))`)

// secretXMLRe finds an API key element, as in Redmine's XML API
// (<api_key>...</api_key>). Groups 1 and 2 are the tags.
var secretXMLRe = regexp.MustCompile(`(?i)(<api[_-]?key(?:\s[^>]*)?>)[^<]*(</api[_-]?key\s*>)`)

// redactSecretText masks API keys in text that is not a JSON document the
// structured redaction (redactSecrets) can walk: an extension's plain text
// or mixed output, the string values of its JSON output, or its stderr in
// an error message. It errs on the side of masking, since text has no
// structure to tell a key from prose that happens to look like one;
// quoting is kept, so JSON stays JSON.
func redactSecretText(s string) string {
	if !strings.Contains(strings.ToLower(s), "key") {
		return s
	}
	s = secretXMLRe.ReplaceAllString(s, "${1}"+redactedText+"${2}")
	return secretTextRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := secretTextRe.FindStringSubmatch(m)
		switch {
		case sub[2] != "":
			return sub[1] + `\"` + redactedText + `\"`
		case sub[3] != "":
			return sub[1] + `"` + redactedText + `"`
		default:
			return sub[1] + redactedText
		}
	})
}

// extensionSecretTokens are the normalized keys (see normalizeSecretKey)
// of tokens and credentials an extension result must not carry, besides
// every key ending in "apikey" or "password".
var extensionSecretTokens = map[string]bool{
	"token":         true,
	"authtoken":     true,
	"accesstoken":   true,
	"apitoken":      true,
	"bearertoken":   true,
	"refreshtoken":  true,
	"authorization": true,
}

// normalizeSecretKey lowercases k and drops the separators that tell
// api_key, api-key, apiKey and "API Key" apart.
func normalizeSecretKey(k string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '_', '-', ' ', '.':
			return -1
		}
		return unicode.ToLower(r)
	}, k)
}

// isExtensionSecretKey reports whether the JSON key k of an extension
// result names a secret: an API key in any spelling (api_key, apiKey,
// X-Redmine-API-Key, REDMINE_API_KEY), a password, a token or an
// Authorization header. An extension's output has no schema the server
// knows, so this is broader than secretJSONFields, which describes the
// fields of Redmine's own responses.
func isExtensionSecretKey(k string) bool {
	n := normalizeSecretKey(k)
	return strings.HasSuffix(n, "apikey") || strings.HasSuffix(n, "password") || extensionSecretTokens[n]
}

// redactExtensionJSON returns v, a JSON value decoded from an extension's
// output, with every secret removed: entries whose key names a secret are
// dropped (see isExtensionSecretKey) and every string, keys included, is
// masked with redactSecretText, so a secret quoted inside a string (a raw
// response, a log line) does not get through either. Maps and slices are
// changed in place.
func redactExtensionJSON(v any) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		for _, k := range keys {
			val := x[k]
			delete(x, k)
			if isExtensionSecretKey(k) {
				continue
			}
			x[redactSecretText(k)] = redactExtensionJSON(val)
		}
		return x
	case []any:
		for i := range x {
			x[i] = redactExtensionJSON(x[i])
		}
		return x
	case string:
		return redactSecretText(x)
	default:
		return v
	}
}
