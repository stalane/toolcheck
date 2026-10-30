package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stalane/toolcheck/internal/config"
	"github.com/stalane/toolcheck/internal/store"
)

func testServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	tools := []store.Tool{
		{Name: "uv", Path: "/home/u/.local/bin/uv", RealPath: "/home/u/.local/bin/uv", Family: "Python", Version: "0.12.5", Origin: "self", ManagedBy: "self", Risk: "safe", Active: true, Updateable: true, Writable: true, LatestVersion: "0.13.6"},
		{Name: "bun", Path: "/home/u/.bun/bin/bun", RealPath: "/home/u/.bun/bin/bun", Family: "Node", Version: "1.4.2", Origin: "bun", ManagedBy: "self", Risk: "safe", Active: false, Updateable: true, Writable: true, LatestVersion: "1.4.4"},
	}
	if err := st.ReplaceTools(context.Background(), tools); err != nil {
		t.Fatal(err)
	}
	return New(st, "127.0.0.1:8788", config.Default(), nil, nil, nil), st
}

func TestDashboardOnlyShowsActive(t *testing.T) {
	s, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1:8788"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "uv") {
		t.Fatal("active tool missing")
	}
	if strings.Contains(body, "1.4.4") {
		t.Fatal("inactive tool should be hidden by default")
	}
}

func TestToolDetail(t *testing.T) {
	s, st := testServer(t)
	tools, _ := st.ListTools(context.Background())
	id := tools[len(tools)-1].ID
	for _, tool := range tools {
		if tool.Name == "uv" {
			id = tool.ID
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/tool/"+itoa(id), nil)
	req.Host = "127.0.0.1:8788"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Update to 0.13.6") {
		t.Fatalf("missing update button:\n%s", body)
	}
}

func TestCSRFRequired(t *testing.T) {
	s, st := testServer(t)
	tools, _ := st.ListTools(context.Background())
	id := tools[0].ID
	req := httptest.NewRequest(http.MethodPost, "/tool/"+itoa(id)+"/update", strings.NewReader(""))
	req.Host = "127.0.0.1:8788"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestHostGuard(t *testing.T) {
	s, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "evil.example"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestStaticAssets(t *testing.T) {
	s, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	req.Host = "127.0.0.1:8788"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("status = %d, bytes = %d", rec.Code, rec.Body.Len())
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
