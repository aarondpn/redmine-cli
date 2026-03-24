package cmdutil

import (
	"testing"
)

func TestFilterCompletions(t *testing.T) {
	items := []string{
		"Bug",
		"Feature",
		"Support",
		"buecher\tBücher",
		"backend\tBackend Project",
	}

	tests := []struct {
		name       string
		toComplete string
		want       []string
	}{
		{
			name:       "empty prefix returns all",
			toComplete: "",
			want:       items,
		},
		{
			name:       "exact prefix match",
			toComplete: "Bug",
			want:       []string{"Bug"},
		},
		{
			name:       "case insensitive",
			toComplete: "bug",
			want:       []string{"Bug"},
		},
		{
			name:       "partial prefix",
			toComplete: "b",
			want:       []string{"Bug", "buecher\tBücher", "backend\tBackend Project"},
		},
		{
			name:       "matches value before tab description",
			toComplete: "bue",
			want:       []string{"buecher\tBücher"},
		},
		{
			name:       "no match returns nil",
			toComplete: "xyz",
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterCompletions(items, tt.toComplete)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d results, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
