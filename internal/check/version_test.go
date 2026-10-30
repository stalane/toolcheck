package check

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, installed string
		want              bool
	}{
		{"12.2.1", "12.2.1", false},
		{"12.3.0", "12.2.1", true},
		{"11.22.0", "12.16.1", false},
		{"26.13.0", "24.22.1", true},
		{"v2.9.8", "2.9.7", true},
		{"go1.27.3", "go1.26.8", true},
		{"1.99.1", "1.93.1", true},
		{"4.0.2", "4.0.10", false},
		{"2026.10.24", "2026.9.1", true},
		{"unknown", "1.0.0", false},
		{"1.0.0", "unknown", false},
		{"2.3.1-0", "2.3.1", false},
	}
	for _, tc := range cases {
		if got := IsNewer(tc.latest, tc.installed); got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.latest, tc.installed, got, tc.want)
		}
	}
}
