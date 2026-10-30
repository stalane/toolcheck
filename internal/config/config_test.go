package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestLoadMissingUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Check.Interval.Duration() != 24*time.Hour {
		t.Fatalf("interval = %v", cfg.Check.Interval.Duration())
	}
	if cfg.Server.Addr != "127.0.0.1:8788" {
		t.Fatalf("addr = %q", cfg.Server.Addr)
	}
}

func TestLoadOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("server:\n  addr: 127.0.0.1:9999\ncheck:\n  interval: 2h\n  timeout: 5s\n  cache_ttl: 1h\ntools:\n  node_channel: current\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.Addr != "127.0.0.1:9999" || cfg.Check.Interval.Duration() != 2*time.Hour {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.NodeChannel() != "current" {
		t.Fatalf("node channel = %q", cfg.NodeChannel())
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	cases := []Config{
		func() Config { c := Default(); c.Check.Interval = 0; return c }(),
		func() Config { c := Default(); c.Check.Timeout = 0; return c }(),
		func() Config { c := Default(); c.Tools.NodeChannel = "nightly"; return c }(),
		func() Config { c := Default(); c.Server.Addr = "0.0.0.0:8788"; return c }(),
	}
	for i, cfg := range cases {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("case %d: expected validation error", i)
		}
	}
}

func TestLoopbackAddr(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:1", "localhost:8788", "[::1]:9"} {
		if err := ValidateLoopbackAddr(addr); err != nil {
			t.Fatalf("%s: %v", addr, err)
		}
	}
	for _, addr := range []string{"0.0.0.0:8788", "example.com:8788", "127.0.0.1"} {
		if err := ValidateLoopbackAddr(addr); err == nil {
			t.Fatalf("%s: expected error", addr)
		}
	}
}
