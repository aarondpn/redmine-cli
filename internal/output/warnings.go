package output

// SupportsWarnings reports whether human-readable warnings should be emitted
// for the selected output format.
func SupportsWarnings(format string) bool {
	return format != FormatJSON
}
