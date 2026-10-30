package detect

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stalane/toolcheck/internal/registry"
)

func testOptions(home string, lookups map[string][]string, versions map[string]string) Options {
	return Options{
		Timeout: time.Second,
		LookPathAll: func(name string) []string {
			return lookups[name]
		},
		RunVersion: func(_ context.Context, path string, _ ...string) (string, error) {
			return versions[path], nil
		},
		DpkgOwner: func(string) bool { return false },
		Writable:  func(string) bool { return true },
		HomeDir:   func() (string, error) { return home, nil },
		Getenv:    func(string) string { return "" },
	}
}

func entry(name string) registry.Tool {
	tool, ok := registry.ByName(name)
	if !ok {
		panic("unknown test tool " + name)
	}
	return tool
}

func TestDetectActiveAndInactiveDuplicates(t *testing.T) {
	home := t.TempDir()
	first := filepath.Join(home, ".local/bin/node")
	second := filepath.Join(home, ".nvm/versions/node/v24/bin/node")
	opts := testOptions(home, map[string][]string{"node": {first, second}}, map[string]string{
		first:  "v24.22.1",
		second: "v24.22.1",
	})
	opts.Getenv = func(key string) string {
		if key == "NVM_DIR" {
			return filepath.Join(home, ".nvm")
		}
		return ""
	}
	tools := New(opts).Detect(context.Background(), []registry.Tool{entry("node")})
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(tools))
	}
	if !tools[0].Active || tools[1].Active {
		t.Fatalf("active flags = %v, %v", tools[0].Active, tools[1].Active)
	}
	if tools[1].Origin != "nvm" || tools[1].ManagedBy != "version-manager" {
		t.Fatalf("nvm tool = %+v", tools[1])
	}
}

func TestDetectClassifiesSelfManagedNPMAsUpdateable(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".npm-global/lib/node_modules/npm/bin/npm-cli.js")
	tools := New(testOptions(home, map[string][]string{"npm": {path}}, map[string]string{path: "12.2.1"})).
		Detect(context.Background(), []registry.Tool{entry("npm")})
	if len(tools) != 1 {
		t.Fatalf("tools = %d", len(tools))
	}
	tool := tools[0]
	if tool.Origin != "npm-global" || tool.ManagedBy != "self" {
		t.Fatalf("origin = %s, managed = %s", tool.Origin, tool.ManagedBy)
	}
	if !tool.Updateable || tool.Risk != "safe" {
		t.Fatalf("updateable = %v, risk = %s", tool.Updateable, tool.Risk)
	}
	if tool.Version != "12.2.1" {
		t.Fatalf("version = %q", tool.Version)
	}
}

func TestDetectAptWinsOverHome(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "fake-bin/pip3")
	opts := testOptions(home, map[string][]string{"pip3": {path}}, map[string]string{path: "pip 25.1.1"})
	opts.DpkgOwner = func(string) bool { return true }
	tools := New(opts).Detect(context.Background(), []registry.Tool{entry("pip")})
	if len(tools) != 1 || tools[0].ManagedBy != "apt" || tools[0].Risk != "system" {
		t.Fatalf("tools = %+v", tools)
	}
	if tools[0].Updateable {
		t.Fatal("apt tool must not be updateable")
	}
}

func TestDetectCorepackIsAlertOnly(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join("/usr/local/lib/node_modules/corepack/dist/pnpm.js")
	opts := testOptions(home, map[string][]string{"pnpm": {path}}, map[string]string{path: "11.22.0"})
	tools := New(opts).Detect(context.Background(), []registry.Tool{entry("pnpm")})
	if len(tools) != 1 || tools[0].Origin != "corepack" || tools[0].ManagedBy != "corepack" {
		t.Fatalf("tools = %+v", tools)
	}
	if tools[0].Updateable {
		t.Fatal("corepack tool must be alert-only")
	}
}

func TestDetectRootOwnedManual(t *testing.T) {
	home := t.TempDir()
	path := "/usr/local/bin/node"
	opts := testOptions(home, map[string][]string{"node": {path}}, map[string]string{path: "v24.22.1"})
	opts.Writable = func(string) bool { return false }
	tools := New(opts).Detect(context.Background(), []registry.Tool{entry("node")})
	if len(tools) != 1 || tools[0].ManagedBy != "manual" {
		t.Fatalf("tools = %+v", tools)
	}
	if tools[0].Updateable {
		t.Fatal("root-owned manual tool must not be updateable")
	}
}

func TestSyntheticNVM(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".nvm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"version":"0.40.6"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := testOptions(home, nil, nil)
	tools := New(opts).Detect(context.Background(), []registry.Tool{entry("nvm")})
	if len(tools) != 1 {
		t.Fatalf("tools = %d", len(tools))
	}
	if tools[0].Version != "0.40.6" || tools[0].Origin != "nvm" {
		t.Fatalf("tool = %+v", tools[0])
	}
}

func TestSyntheticMissing(t *testing.T) {
	home := t.TempDir()
	tools := New(testOptions(home, nil, nil)).Detect(context.Background(), []registry.Tool{entry("nvm")})
	if len(tools) != 0 {
		t.Fatalf("tools = %+v", tools)
	}
}

func TestParseVersion(t *testing.T) {
	cases := []struct {
		output  string
		pattern string
		want    string
	}{
		{"v24.22.1", `v(\d+\.\d+\.\d+)`, "24.22.1"},
		{"go version go1.26.8 linux/amd64", `go(\d+\.\d+(?:\.\d+)?)`, "1.26.8"},
		{"Python 3.14.4", `Python (\d+\.\d+\.\d+)`, "3.14.4"},
		{"uv 0.12.5 (x86_64-unknown-linux-gnu)", `uv (\d+\.\d+\.\d+)`, "0.12.5"},
		{"rustc 1.93.1 (01f6ddf75 2026-02-11)", `rustc (\d+\.\d+\.\d+)`, "1.93.1"},
		{"Composer version 2.11.1 2026-01-01", `Composer version (\d+\.\d+\.\d+)`, "2.11.1"},
		{"Homebrew 4.6.4", `Homebrew (\d+\.\d+\.\d+)`, "4.6.4"},
		{"no version here", ``, ""},
		{"something 2.4", ``, "2.4"},
	}
	for _, tc := range cases {
		if got := parseVersion(tc.output, tc.pattern); got != tc.want {
			t.Errorf("parseVersion(%q, %q) = %q, want %q", tc.output, tc.pattern, got, tc.want)
		}
	}
}

func TestUnder(t *testing.T) {
	cases := []struct {
		path, dir string
		want      bool
	}{
		{"/home/u/.nvm/versions/node", "/home/u/.nvm", true},
		{"/home/u/.nvm", "/home/u/.nvm", true},
		{"/home/u/.nvmalso", "/home/u/.nvm", false},
		{"/usr/bin/npm", "/home/u/.nvm", false},
	}
	for _, tc := range cases {
		if got := under(tc.path, tc.dir); got != tc.want {
			t.Errorf("under(%q, %q) = %v, want %v", tc.path, tc.dir, got, tc.want)
		}
	}
}
