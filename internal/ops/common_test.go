package ops

import "testing"

func TestListLimit(t *testing.T) {
	cases := []struct {
		name     string
		input    int
		expected int
	}{
		{"NoLimit translates to API unlimited (0)", NoLimit, 0},
		{"any negative translates to API unlimited", -100, 0},
		{"zero applies MCP-safety default", 0, DefaultListLimit},
		{"positive passes through", 25, 25},
		{"large positive passes through", 500, 500},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ListLimit(tc.input)
			if got != tc.expected {
				t.Fatalf("ListLimit(%d) = %d, want %d", tc.input, got, tc.expected)
			}
		})
	}
}
