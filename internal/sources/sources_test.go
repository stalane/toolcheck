package sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := New("toolcheck/test")
	c.NPMBase = server.URL
	c.PyPIBase = server.URL
	c.NodeBase = server.URL
	c.GoBase = server.URL
	c.DenoBase = server.URL
	c.ComposerBase = server.URL
	c.RustupBase = server.URL
	c.RustBase = server.URL
	c.GitHubBase = server.URL
	c.AnacondaBase = server.URL
	c.RubyGemsBase = server.URL
	c.DotNetBase = server.URL
	return c
}

func TestFetchAllSources(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/npm", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"dist-tags":{"latest":"12.2.1"},"time":{"12.2.1":"2026-09-01T10:00:00Z"}}`))
	})
	mux.HandleFunc("/pypi/uv/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"info":{"version":"0.13.6"},"releases":{"0.13.6":[{"upload_time_iso_8601":"2026-10-20T12:00:00Z"}]}}`))
	})
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"version":"v27.0.0-alpha","lts":false,"date":"2026-10-01"},{"version":"v26.13.0","lts":"Lithium","date":"2026-10-28","security":false}]`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`[{"version":"go1.27.3"}]`))
		case "/release-latest.txt":
			_, _ = w.Write([]byte("v2.9.8\n"))
		case "/versions":
			_, _ = w.Write([]byte(`{"stable":[{"version":"2.11.1"}]}`))
		case "/rustup/release-stable.toml":
			_, _ = w.Write([]byte("schema-version = '1'\nversion = '1.29.2'\n"))
		case "/dist/channel-rust-stable.toml":
			_, _ = w.Write([]byte("[pkg.rust]\nversion = \"1.99.1 (58cf8f9c3 2026-10-13)\"\n"))
		case "/package/conda/conda":
			_, _ = w.Write([]byte(`{"versions":["4.0.1","4.1.0rc3","4.0.2"]}`))
		case "/api/v1/versions/bundler/latest.json":
			_, _ = w.Write([]byte(`{"version":"4.0.23"}`))
		case "/dotnet/release-metadata/releases-index.json":
			_, _ = w.Write([]byte(`{"releases-index":[{"channel-version":"11.0","latest-sdk":"11.0.100-rc.3","latest-release-date":"2026-10-25","support-phase":"go-live"},{"channel-version":"10.0","latest-sdk":"10.0.403","latest-release-date":"2026-10-25","support-phase":"active","security":true}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/owner/repo/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/owner/repo/releases/tag/v2.0.2", http.StatusFound)
	})

	c := testClient(t, mux)
	ctx := context.Background()

	cases := []struct {
		spec    string
		version string
	}{
		{"npm:npm", "12.2.1"},
		{"pypi:uv", "0.13.6"},
		{"node:lts", "26.13.0"},
		{"node:current", "26.13.0"},
		{"go", "1.27.3"},
		{"deno", "2.9.8"},
		{"composer", "2.11.1"},
		{"rustup", "1.29.2"},
		{"rust", "1.99.1"},
		{"github:owner/repo", "2.0.2"},
		{"anaconda:conda", "4.0.2"},
		{"rubygems:bundler", "4.0.23"},
		{"dotnet", "10.0.403"},
	}
	for _, tc := range cases {
		result, err := c.Fetch(ctx, tc.spec)
		if err != nil {
			t.Errorf("%s: %v", tc.spec, err)
			continue
		}
		if result.Version != tc.version {
			t.Errorf("%s: version = %q, want %q", tc.spec, result.Version, tc.version)
		}
	}
}

func TestNodeLTSSelection(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"version":"v27.0.0-alpha","lts":false},{"version":"v26.13.0","lts":"Lithium"},{"version":"v24.22.1","lts":"Krypton"}]`))
	})
	c := testClient(t, mux)
	result, err := c.Fetch(context.Background(), "node:lts")
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "26.13.0" {
		t.Fatalf("version = %q", result.Version)
	}
	if result.Published.IsZero() {
		t.Log("no publish date in fixture (allowed)")
	}
}

func TestNoSource(t *testing.T) {
	c := New("toolcheck/test")
	if _, err := c.Fetch(context.Background(), "none"); !errors.Is(err, ErrNoSource) {
		t.Fatalf("err = %v", err)
	}
	if _, err := c.Fetch(context.Background(), "unknown:x"); err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestHTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/npm", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	})
	c := testClient(t, mux)
	if _, err := c.Fetch(context.Background(), "npm:npm"); err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestResponseSizeLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/npm", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 1000)))
	})
	c := testClient(t, mux)
	c.MaxBytes = 10
	if _, err := c.Fetch(context.Background(), "npm:npm"); err == nil {
		t.Fatal("expected size error")
	}
}

func TestPublishedTimeParsed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pypi/uv/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"info":{"version":"0.13.6"},"releases":{"0.13.6":[{"upload_time_iso_8601":"2026-10-20T12:00:00Z"}]}}`))
	})
	c := testClient(t, mux)
	result, err := c.Fetch(context.Background(), "pypi:uv")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	if !result.Published.Equal(want) {
		t.Fatalf("published = %v, want %v", result.Published, want)
	}
}
