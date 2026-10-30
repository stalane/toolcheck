# toolcheck Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Linux-only CLI and localhost dashboard that detects package managers/runtimes installed outside apt, compares them with upstream releases, and safely updates six user-managed tools after confirmation.

**Architecture:** A static tool registry feeds a detector that probes PATH, resolves real paths, classifies origin (apt via `dpkg-query`, corepack, npm-global, bun, uv, pipx, version managers, brew/nix/snap/flatpak, manual root-owned), and records everything in SQLite. A source client fetches official release endpoints with a 24h cache; a small update engine provides six adapters with pre-flight, age, verification, and rollback guards.

**Tech Stack:** Go 1.26, `modernc.org/sqlite`, `gopkg.in/yaml.v3`, `gen2brain/beeep`, `Masterminds/semver`, embedded `html/template`, HTMX-free server-rendered UI using alertme's xwllz CSS.

**Spec:** `docs/superpowers/specs/2026-10-31-toolcheck-design.md`

## Global Constraints

- Linux only: GoReleaser emits linux/amd64 and linux/arm64.
- Never invoke sudo, a shell string, or auto-apply an update.
- apt-managed and non-writable installs are alert-only.
- Dashboard shows only installed tools; absent registry entries stay invisible.
- Port 8788; XDG config/data dirs; single static binary.
- All outbound requests use HTTPS and fixed official endpoints.

## Review Focus

- A root-owned `/usr/local` install (node here) must never become updateable.
- A corepack shim (pnpm/yarn) must remain alert-only even when newer.
- npm's full packument is too large; the abbreviated request must not lose the
  version, and the missing publish time must not fake a release date.
- Duplicate PATH copies must not double-count updates or notifications.
- A failed verification must roll back when an adapter supports it and must
  never report success.

---

### Task 1: Scaffold and core kit

**Files:** `go.mod`, `Makefile`, `cmd/toolcheck/main.go`, `internal/{config,version,lock,logging}`, `LICENSE`, `.gitignore`

- [x] Config defaults/validation, XDG paths, loopback validation.
- [x] Version string, file lock, JSON logger.
- [x] `go build ./...` green.

### Task 2: Store

**Files:** `internal/store/{types,schema,store,queries}.go`

- [x] Schema: tools, checks, tool_updates, tool_notifications.
- [x] `ReplaceTools` preserves IDs, removes stale rows, keeps cache columns.
- [x] Tests for round-trip, stale deletion, checks, updates, notification dedupe.

### Task 3: Registry and detection

**Files:** `internal/registry/registry.go`, `internal/detect/{detect.go,detect_test.go}`

- [x] 40+ entries with command, version parser, source, optional update adapter,
  synthetic managers (nvm, sdkman, RVM, gvm), and hint text.
- [x] PATH scanning with active/inactive dedupe; synthetic directory detection.
- [x] Origin classification and writability; apt/corepack/version-manager guards.
- [x] Tests for duplicates, npm-global, apt, corepack, manual root-owned, nvm,
  version parsing.

### Task 4: Release sources

**Files:** `internal/sources/*.go`

- [x] HTTP client: 15s timeout, UA, 8 MiB cap, HTTPS bases overridable in tests.
- [x] npm (abbreviated), PyPI, node LTS/current, go.dev, deno, composer, rustup,
  rust, GitHub release redirect, anaconda, rubygems, .NET.
- [x] `ErrNoSource` / `ErrNoRelease` treated as non-fatal.
- [x] httptest coverage for every source, size cap, HTTP errors, publish time.

### Task 5: Check engine, notifier, scheduler

**Files:** `internal/check/*`, `internal/notify/*`, `internal/scheduler/*`, `internal/present/*`

- [x] Cache per source, one fetch per spec, active-only update counts.
- [x] Version comparison via semver with numeric fallback.
- [x] Desktop notifications deduped per tool+latest; skip system/inactive.
- [x] Scheduler runs immediately then on interval.

### Task 6: CLI and web

**Files:** `internal/cli/*`, `internal/web/*`, `internal/service/*`

- [x] Commands: check, list, update, serve, service, config, version, help.
- [x] Dashboard, tool detail, activity, settings; CSRF and Host/Origin guards.
- [x] systemd user unit install/uninstall.
- [x] Web smoke tests; manual 200 checks on all pages.

### Task 7: Update engine

**Files:** `internal/update/*`

- [x] Plan/apply for npm, uv, bun, deno, composer, rustup.
- [x] Refusals: unsupported, non-self, non-writable, not newer, too new.
- [x] Pre-flight drift abort, post-apply verification, rollback where supported.
- [x] Update log; tests for success, rollback, drift, latest-only, age gate.

### Task 8: Docs and release

**Files:** `README.md`, `SECURITY.md`, `HANDOFF.md`, `.goreleaser.yaml`

- [x] Usage, coverage, policy, privacy, limitations.
- [x] Linux-only release config and ldflags version injection.
- [x] `make check`, `go test -race ./...`, live host check, private repo push.
