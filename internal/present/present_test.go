package present

import (
	"testing"

	"github.com/stalane/toolcheck/internal/store"
)

func TestState(t *testing.T) {
	cases := []struct {
		tool  store.Tool
		state string
	}{
		{store.Tool{ManagedBy: "self", Version: "1.0.0", LatestVersion: "1.1.0"}, StateUpdate},
		{store.Tool{ManagedBy: "self", Version: "1.1.0", LatestVersion: "1.1.0"}, StateCurrent},
		{store.Tool{ManagedBy: "apt", Version: "1.0.0", LatestVersion: "2.0.0"}, StateSystem},
		{store.Tool{ManagedBy: "manual", Version: "1.0.0", LatestVersion: "2.0.0"}, StateManual},
	}
	for _, tc := range cases {
		if got := State(tc.tool); got != tc.state {
			t.Errorf("State(%+v) = %q, want %q", tc.tool, got, tc.state)
		}
	}
}
