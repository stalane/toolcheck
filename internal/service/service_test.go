package service

import (
	"strings"
	"testing"
)

func TestContent(t *testing.T) {
	unit := Content("/home/test/.local/bin/toolcheck")
	for _, want := range []string{
		"Description=toolcheck",
		`ExecStart="/home/test/.local/bin/toolcheck" serve`,
		"WantedBy=default.target",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
}
