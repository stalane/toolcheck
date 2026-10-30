package store

import (
	"context"
	"testing"
	"time"
)

func TestReplaceToolsPreservesIDAndDeletesStale(t *testing.T) {
	st, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	checked := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	first := []Tool{
		{Name: "uv", Path: "/home/u/.local/bin/uv", RealPath: "/home/u/.local/bin/uv", Version: "0.12.5", ManagedBy: "self", Active: true, LatestVersion: "0.13.6", CheckedAt: &checked},
		{Name: "bun", Path: "/home/u/.bun/bin/bun", RealPath: "/home/u/.bun/bin/bun", Version: "1.4.2", ManagedBy: "self", Active: true},
	}
	if err := st.ReplaceTools(ctx, first); err != nil {
		t.Fatal(err)
	}
	stored, err := st.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 {
		t.Fatalf("tools = %d", len(stored))
	}
	var uvID int64
	for _, tool := range stored {
		if tool.Name == "uv" {
			uvID = tool.ID
			if tool.LatestVersion != "0.13.6" || tool.CheckedAt == nil {
				t.Fatalf("uv cache lost: %+v", tool)
			}
		}
	}

	second := []Tool{{Name: "uv", Path: "/home/u/.local/bin/uv", RealPath: "/home/u/.local/bin/uv", Version: "0.13.6", ManagedBy: "self", Active: true}}
	if err := st.ReplaceTools(ctx, second); err != nil {
		t.Fatal(err)
	}
	stored, _ = st.ListTools(ctx)
	if len(stored) != 1 {
		t.Fatalf("stale tool not removed: %+v", stored)
	}
	if stored[0].ID != uvID {
		t.Fatalf("id changed: %d -> %d", uvID, stored[0].ID)
	}
	if err := st.SetToolVersion(ctx, uvID, "0.13.6"); err != nil {
		t.Fatal(err)
	}
	updated, _ := st.GetTool(ctx, uvID)
	if updated.Version != "0.13.6" {
		t.Fatalf("version = %q", updated.Version)
	}
}

func TestCheckAndUpdateRecords(t *testing.T) {
	st, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	id, err := st.RecordCheck(ctx, CheckRecord{StartedAt: now, FinishedAt: now.Add(time.Second), Status: "ok", Tools: 4, Updates: 1})
	if err != nil || id == 0 {
		t.Fatalf("record check: %d, %v", id, err)
	}
	checks, err := st.ListChecks(ctx, 10)
	if err != nil || len(checks) != 1 || checks[0].Updates != 1 {
		t.Fatalf("checks = %+v, %v", checks, err)
	}

	updateID, err := st.RecordUpdate(ctx, UpdateRecord{ToolID: 1, ToolName: "uv", FromVersion: "0.12.5", ToVersion: "0.13.6", Command: "uv self update 0.13.6", Status: "applied", StartedAt: now, FinishedAt: now})
	if err != nil || updateID == 0 {
		t.Fatalf("record update: %d, %v", updateID, err)
	}
	updates, err := st.ListUpdates(ctx, 1, 10)
	if err != nil || len(updates) != 1 || updates[0].Status != "applied" {
		t.Fatalf("updates = %+v, %v", updates, err)
	}
}

func TestNotifications(t *testing.T) {
	st, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	notified, err := st.WasNotified(ctx, 7, "1.2.3")
	if err != nil || notified {
		t.Fatalf("notified = %v, %v", notified, err)
	}
	if err := st.RecordNotification(ctx, 7, "1.2.3", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordNotification(ctx, 7, "1.2.3", time.Now()); err != nil {
		t.Fatal(err)
	}
	notified, err = st.WasNotified(ctx, 7, "1.2.3")
	if err != nil || !notified {
		t.Fatalf("notified = %v, %v", notified, err)
	}
}
