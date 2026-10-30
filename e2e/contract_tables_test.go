//go:build e2e

package e2e

import "testing"

// TestContractTablesAreConsistent checks the contract case and gate tables
// without a server: case names are unique, every gate row names a real case
// and a path, and every no-case entry carries a reason. Like the version
// comparator tests in version_gate_test.go it deliberately does not call
// requireE2E.
func TestContractTablesAreConsistent(t *testing.T) {
	names := map[string]bool{}
	for _, c := range contractCases {
		if names[c.Name] {
			t.Errorf("duplicate contract case %q", c.Name)
		}
		names[c.Name] = true
		if c.Raw == "" || c.Root == "" {
			t.Errorf("contract case %q needs Raw and Root", c.Name)
		}
		if c.Pick != "" && c.List {
			t.Errorf("contract case %q sets both Pick and List: Pick already selects one element", c.Name)
		}
	}
	for _, g := range gatedKeys {
		if !names[g.Case] {
			t.Errorf("gate row %q/%q references an unknown case", g.Case, g.Path)
		}
		if g.Path == "" {
			t.Errorf("gate row for case %q has no path", g.Case)
		}
	}
	for tool, reason := range contractNoCase {
		if reason == "" {
			t.Errorf("contractNoCase[%q] needs a reason", tool)
		}
	}
}
