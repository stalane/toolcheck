package update

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stalane/toolcheck/internal/store"
)

type fakeRunner struct {
	calls   [][]string
	version *string
	onRun   func(args []string)
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
	if f.onRun != nil {
		f.onRun(call)
	}
	return "ok", nil
}

func testUpdater(t *testing.T) (*Updater, *store.Store, *string) {
	t.Helper()
	st, err := store.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	version := "0.12.5"
	return &Updater{
		Store:   st,
		Now:     func() time.Time { return time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC) },
		Access:  func(string) bool { return true },
		Probe:   func(context.Context, Plan) (string, error) { return version, nil },
		Timeout: time.Second,
	}, st, &version
}

func uvTool() store.Tool {
	old := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return store.Tool{
		ID: 1, Name: "uv", Path: "/home/test/.local/bin/uv", RealPath: "/home/test/.local/bin/uv",
		Version: "0.12.5", ManagedBy: "self", Updateable: true, Writable: true,
		LatestVersion: "0.13.6", LatestPublishedAt: &old, Active: true,
	}
}

func TestPlanUV(t *testing.T) {
	u, _, _ := testUpdater(t)
	plan, err := u.Plan(uvTool(), "", 72*time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.To != "0.13.6" {
		t.Fatalf("to = %q", plan.To)
	}
	want := []string{"/home/test/.local/bin/uv", "self", "update", "0.13.6"}
	if len(plan.Command) != len(want) {
		t.Fatalf("command = %v", plan.Command)
	}
	for i := range want {
		if plan.Command[i] != want[i] {
			t.Fatalf("command = %v, want %v", plan.Command, want)
		}
	}
}

func TestPlanRefusals(t *testing.T) {
	u, _, _ := testUpdater(t)

	node := uvTool()
	node.Name = "node"
	node.Updateable = false
	if _, err := u.Plan(node, "", 0, false); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("node err = %v", err)
	}

	apt := uvTool()
	apt.ManagedBy = "apt"
	if _, err := u.Plan(apt, "", 0, false); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("apt err = %v", err)
	}

	u.Access = func(string) bool { return false }
	if _, err := u.Plan(uvTool(), "", 0, false); !errors.Is(err, ErrNotWritable) {
		t.Fatalf("writable err = %v", err)
	}
	u.Access = func(string) bool { return true }

	if _, err := u.Plan(uvTool(), "0.12.5", 0, false); !errors.Is(err, ErrNotNewer) {
		t.Fatalf("not newer err = %v", err)
	}
}

func TestAgeGate(t *testing.T) {
	u, _, _ := testUpdater(t)
	tool := uvTool()
	recent := time.Date(2026, 10, 31, 6, 0, 0, 0, time.UTC)
	tool.LatestPublishedAt = &recent

	if _, err := u.Plan(tool, "", 72*time.Hour, false); !errors.Is(err, ErrTooNew) {
		t.Fatalf("err = %v", err)
	}
	if _, err := u.Plan(tool, "", 72*time.Hour, true); err != nil {
		t.Fatalf("forced plan: %v", err)
	}
}

func TestApplySuccess(t *testing.T) {
	u, st, version := testUpdater(t)
	plan, err := u.Plan(uvTool(), "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	u.Runner = &fakeRunner{version: version, onRun: func(args []string) {
		if args[3] == "0.13.6" {
			*version = "0.13.6"
		}
	}}
	result, err := u.Apply(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "0.13.6" || result.RolledBack {
		t.Fatalf("result = %+v", result)
	}
	records, err := st.ListUpdates(context.Background(), 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Status != "applied" {
		t.Fatalf("records = %+v", records)
	}
}

func TestApplyRollsBackOnVerificationFailure(t *testing.T) {
	u, st, _ := testUpdater(t)
	plan, err := u.Plan(uvTool(), "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	u.Runner = &fakeRunner{}
	result, err := u.Apply(context.Background(), plan)
	if err == nil {
		t.Fatal("expected verification error")
	}
	if !result.RolledBack {
		t.Fatalf("result = %+v", result)
	}
	records, _ := st.ListUpdates(context.Background(), 1, 10)
	if len(records) != 1 || records[0].Status != "rolled_back" {
		t.Fatalf("records = %+v", records)
	}
}

func TestApplyRefusesDrift(t *testing.T) {
	u, _, _ := testUpdater(t)
	plan, err := u.Plan(uvTool(), "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	u.Probe = func(context.Context, Plan) (string, error) { return "0.12.4", nil }
	if _, err := u.Apply(context.Background(), plan); err == nil {
		t.Fatal("expected drift error")
	}
}

func TestLatestOnlyPlan(t *testing.T) {
	u, _, _ := testUpdater(t)
	tool := uvTool()
	tool.Name = "bun"
	tool.Path = "/home/test/.bun/bin/bun"
	tool.RealPath = tool.Path
	tool.Version = "1.4.2"
	tool.LatestVersion = "1.4.4"
	plan, err := u.Plan(tool, "9.9.9", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.To != "1.4.4" {
		t.Fatalf("latest-only target = %q", plan.To)
	}
	if len(plan.Command) != 2 || plan.Command[1] != "upgrade" {
		t.Fatalf("command = %v", plan.Command)
	}
	if len(plan.Rollback) != 0 {
		t.Fatalf("rollback = %v", plan.Rollback)
	}
}
