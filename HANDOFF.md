# toolcheck — Handoff

Updated: 2026-10-31
Status: v0.1.0 green

## Read first

1. `docs/superpowers/specs/2026-10-31-toolcheck-design.md`
2. `docs/superpowers/plans/2026-10-31-toolcheck-implementation.md`
3. `README.md`

## Project

toolcheck — Linux-only freshness checker for package managers and runtimes
installed outside apt. Separate app from alertme; same Go/HTMX/xwllz kit.
Repo: `/home/david/toolcheck` · module `github.com/stalane/toolcheck`
Go 1.26, no cgo, SQLite (`modernc.org/sqlite`), Go templates.

## Current state

- Detects 40+ tools from a registry; dashboard shows only installed tools,
  collapsed to the active PATH copy.
- Classifies origin: apt via `dpkg-query`, corepack, npm-global, bun, uv, pipx,
  version managers, Linuxbrew, nix, snap, flatpak, manual root-owned.
- Fetch sources: npm registry (abbreviated packument), PyPI, nodejs.org LTS,
  go.dev, deno.land, getcomposer.org, rust-lang.org, GitHub release redirects,
  anaconda.org, rubygems.org, Microsoft .NET release index.
- Safe update adapters: npm, uv, bun, deno, composer, rustup. Everything else
  is alert-only.
- CLI: `check`, `list`, `update`, `serve`, `service`, `config`, `version`,
  `help`. Web dashboard on 127.0.0.1:8788 with CSRF and Host/Origin guards.
- 24h scheduler, desktop notifications deduped per tool+latest version.

## Verification

- `make check` → PASS
- `go test -race ./...` → PASS
- `toolcheck check --refresh` on this host: 22 detected, 5 active updates,
  0 source errors.
- Dashboard/tool/activity/settings pages return 200.

## Known traps

- npm's abbreviated packument has `dist-tags` but no publish times, so the 72h
  release-age gate cannot be applied to npm-family tools. Keep the abbreviated
  request; the full npm document is over 8 MiB.
- dpkg detection must run on the resolved real path; use `dpkg-query -S`.
- Never trust the tool name alone for apt: `/usr/local/bin/node` here is a
  manual root-owned install, not apt.
- Update commands use the detected absolute path, not `PATH`, so a shell shadow
  cannot redirect an update.
- No sudo, ever. Non-writable installs are alert-only.
- `bun upgrade` and `rustup self update` are latest-only and have no rollback;
  warn before applying.
- The web check-now action is synchronous (3-minute cap).
- The systemd unit must point at the installed binary, not a `go run` temp file.

## Next steps

1. Tag `v0.1.0` and verify the private release artifacts.
2. Consider a "captured output" view and per-tool mute in the dashboard.
3. If npm release-age gating matters, add a lightweight per-version publish-time
   lookup (not the full packument).
4. Extend safe adapters only after the current six have run in the field.
