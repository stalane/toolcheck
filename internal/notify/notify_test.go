package notify

import (
	"context"
	"testing"

	"github.com/stalane/toolcheck/internal/store"
)

func TestNotifyDedupes(t *testing.T) {
	st, err := store.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tools := []store.Tool{{
		Name:          "uv",
		Path:          "/home/test/.local/bin/uv",
		RealPath:      "/home/test/.local/bin/uv",
		Version:       "0.12.5",
		LatestVersion: "0.13.6",
		ManagedBy:     "self",
		Active:        true,
	}}
	if err := st.ReplaceTools(context.Background(), tools); err != nil {
		t.Fatal(err)
	}
	stored, err := st.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	sends := 0
	n := &Notifier{
		Enabled: true,
		Store:   st,
		Send: func(title, body string) error {
			sends++
			return nil
		},
	}
	if got := n.Notify(context.Background(), stored); got != 1 {
		t.Fatalf("first notify = %d, want 1", got)
	}
	if got := n.Notify(context.Background(), stored); got != 0 {
		t.Fatalf("second notify = %d, want 0", got)
	}
	if sends != 1 {
		t.Fatalf("sends = %d, want 1", sends)
	}
}

func TestNotifySkipsSystemAndCurrent(t *testing.T) {
	st, err := store.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tools := []store.Tool{
		{Name: "pip", Path: "/usr/bin/pip3", RealPath: "/usr/bin/pip3", Version: "25.1.1", LatestVersion: "26.2.2", ManagedBy: "apt", Active: true},
		{Name: "npm", Path: "/home/test/npm", RealPath: "/home/test/npm", Version: "12.2.1", LatestVersion: "12.2.1", ManagedBy: "self", Active: true},
	}
	if err := st.ReplaceTools(context.Background(), tools); err != nil {
		t.Fatal(err)
	}
	stored, _ := st.ListTools(context.Background())
	n := &Notifier{Enabled: true, Store: st, Send: func(string, string) error { return nil }}
	if got := n.Notify(context.Background(), stored); got != 0 {
		t.Fatalf("notify = %d, want 0", got)
	}
}
