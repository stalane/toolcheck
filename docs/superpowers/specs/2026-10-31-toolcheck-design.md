# toolcheck — Design

**Date:** 2026-10-31
**Status:** approved in conversation 2026-10-31
**Repo:** `/home/david/toolcheck` · **License:** Apache-2.0
**Platform:** Linux only (amd64/arm64)

## 1. Intent

A local-first dashboard and CLI that answers: *"Which package managers and runtimes
on this machine were installed outside apt, are behind upstream, and can I safely
update them?"*

toolcheck is deliberately narrower than alertme. alertme scans project lockfiles
for CVEs. toolcheck watches the tools that install packages — npm, uv, bun, node,
pnpm, and friends — because Ubuntu's security updates do not cover them.

## 2. Why now

On a developer laptop, the package managers themselves are often installed by curl
scripts, corepack, npm itself, bun's installer, or a version manager. apt never
updates those. They drift for months, and an old manager is both a security
exposure and a source of confusing resolver behaviour. Keeping them current is
hygiene, not attack detection.

## 3. Goals

- **G1** Detect installed package managers/runtimes and their exact versions.
- **G2** Classify how each was installed: apt, corepack, self-managed prefix,
  version manager, root-owned manual install, or external (brew/nix/snap/flatpak).
- **G3** Compare installed versions with upstream release channels, cached daily.
- **G4** Show only installed tools in the dashboard; absent registry entries stay
  invisible.
- **G5** Update a small, safe set after explicit confirmation, verifying the new
  version and logging the run.
- **G6** Never use sudo, never run a shell string, never auto-apply.
- **G7** Reuse alertme's stack, visual language, single-binary distribution, and
  localhost-only posture.

## 4. Non-goals

- Scanning packages managed by npm/pip/uv/etc.
- Updating apt/dnf/pacman packages or the OS.
- macOS/Homebrew-on-macOS and Windows support.
- CLI tools that are not package managers (terraform, kubectl, gh, aws).
- Detecting malicious packages or supply-chain compromise.
- A hosted dashboard, accounts, telemetry, or an API key.

## 5. Anti-goals

- Ever invoking `sudo` or elevating.
- Updating a tool that apt or an external manager owns.
- Auto-updating version managers, node, pnpm/yarn/corepack, or anything that can
  silently change a project's toolchain.
- Guessing a "latest" version when no reliable upstream endpoint exists.
- Sending paths, command output, or project names anywhere.

## 6. Decisions

| Area | Decision |
|---|---|
| Platform | Linux; Debian/Ubuntu apt detection via `dpkg-query` |
| UI | Go `html/template` + HTMX, embedded, xwllz theme copied from alertme |
| Port | `127.0.0.1:8788` (alertme uses 8787) |
| Storage | SQLite (`modernc.org/sqlite`), XDG data dir |
| Check cadence | 24h, on serve startup if stale, manual `check` |
| Update set | npm, uv, bun, deno, composer, rustup |
| Alert-only | node, pnpm, yarn, corepack, version managers, Python/Rust/Go/Ruby system tools, conda, dotnet, ollama, brew/nix/snap/flatpak |
| Notifications | Desktop on new upstream release; deduped per tool+version |
| Repo | `github.com/stalane/toolcheck`, private for now |

## 7. Tool coverage

Registry-driven; detection runs over every entry but only installed tools are
persisted.

| Family | Tools |
|---|---|
| Node/JS | node, npm, pnpm, yarn, corepack, bun, deno |
| JS version managers | nvm, fnm, volta, asdf, mise |
| Python | python3, pip, pipx, uv, poetry, pdm, pixi, hatch, conda, micromamba, pyenv |
| Rust | rustup, cargo, rustc, cargo-binstall |
| Go | go, g, gvm, goenv |
| Ruby | ruby, gem, bundler, rbenv, RVM |
| PHP | composer |
| Java | sdkman, jenv |
| .NET | dotnet |
| AI | ollama |
| Alternative package managers | brew (Linuxbrew), nix, snap, flatpak |

Tools without a reliable release endpoint show the installed version and a
"manual" state; no latest version is fabricated.

## 8. Detection and classification

- Scan every `PATH` entry for each command; dedupe by resolved real path; the
  first PATH hit is active, later duplicates are inactive.
- Synthetic detection for shell-only managers: nvm (`$NVM_DIR`/`~/.nvm`),
  sdkman (`$SDKMAN_DIR`/`~/.sdkman`), RVM (`~/.rvm`), gvm (`~/.gvm`).
- Version by executing the command with `--version` under a timeout, then a
  per-tool regex or a generic numeric fallback. Directory-only managers read a
  version file (`package.json`, `VERSION`) or report `unknown`.
- Origin order:
  1. path under `/nix/store`, `/snap`, flatpak roots, Linuxbrew prefix;
  2. corepack shim path;
  3. version-manager env/path hints (nvm, fnm, volta, asdf, mise, pyenv, rbenv,
     rvm, sdkman, gvm, g);
  4. pipx venv / uv tool / bun / npm global / rustup prefixes;
  5. `dpkg-query -S <realpath>` success → apt;
  6. path under `$HOME` and writable → self;
  7. path under `/usr/local` or `/opt` and not writable → manual-root;
  8. unknown.
- Updateable iff the registry has a safe adapter, managed-by is `self`, and the
  install directory is writable. Everything else is alert-only with the exact
  suggested command in the UI.

## 9. Version sources

| Source | Used for | Endpoint |
|---|---|---|
| npm | npm, pnpm, yarn, bun, corepack | `registry.npmjs.org/<pkg>` (latest + `time`) |
| PyPI | uv, pip, pipx, poetry, pdm, pixi, hatch | `pypi.org/pypi/<pkg>/json` |
| node | node | `nodejs.org/dist/index.json` (LTS by default) |
| go | go | `go.dev/dl/?mode=json` |
| deno | deno | `dl.deno.land/release-latest.txt` |
| composer | composer | `getcomposer.org/versions` |
| rustup | rustup | `static.rust-lang.org/rustup/release-stable.toml` |
| rust | cargo, rustc | `static.rust-lang.org/dist/channel-rust-stable.toml` |
| github | version managers, ollama, brew, nix, micromamba, cargo-binstall, jenv, g/gvm/goenv | `github.com/<owner>/<repo>/releases/latest` redirect |
| anaconda | conda | `api.anaconda.org/package/conda/conda` |
| rubygems | bundler, gem | `rubygems.org/api/v1/versions/<gem>/latest.json` |
| dotnet | dotnet | `dotnetcli.blob.core.windows.net/.../releases-index.json` |

Requests use HTTPS, a 15s timeout, a `toolcheck/<version>` user agent, and an
8 MiB response cap. The DB stores the last result and check time; a check only
fetches when the cache is older than `check.cache_ttl` unless `--refresh`.

## 10. Update policy

**Update after confirmation (exact target where the tool supports it):**

| Tool | Command | Verify | Rollback |
|---|---|---|---|
| npm | `npm install -g npm@<v>` | `npm --version` | reinstall old |
| uv | `uv self update <v>` | `uv --version` | update old |
| bun | `bun upgrade` | `bun --version` | none (warn) |
| deno | `deno upgrade --version <v>` | `deno --version` | upgrade old |
| composer | `composer self-update <v>` | `composer --version` | self-update old |
| rustup | `rustup self update` | `rustup --version` | none (warn) |

**Alert-only:** node, pnpm, yarn, corepack, every version manager, pip/pipx,
poetry/pdm/pixi/hatch, conda/micromamba, go, cargo/rustc, ruby/gem/bundler,
sdkman/jenv, dotnet, ollama, brew/nix/snap/flatpak, and all apt-managed tools.

Safety rules:

- No sudo. Refuse when the install directory is not user-writable.
- Pre-flight: re-read the installed version and abort on drift.
- Minimum release age (default 72h) when an upstream publish time is known;
  `--force` overrides after an explicit warning.
- Run commands as argument arrays, never through a shell.
- Verify the new version after apply; best-effort rollback on failure.
- Record every plan, command, status and output excerpt in `tool_updates`.
- Auto-apply does not exist.

## 11. UI

- Dashboard: KPIs (updates, alert-only, system-managed, last check), one table of
  installed tools grouped by state, filter by family, hide inactive duplicates by
  default.
- Tool detail: version, origin, paths, update state, release date/age, exact
  command, update button (confirm + CSRF) for safe tools, disabled reason for
  others, history.
- Activity: checks and update runs.
- Settings: config path, DB path, listen address, last check, check-now button.
- Same tokens/typography/cards as alertme; no external assets.

## 12. Config

`~/.config/toolcheck/config.yaml`:

```yaml
check:
  interval: 24h
  timeout: 15s
  cache_ttl: 24h
tools:
  include: []
  exclude: []
  hide_os_managed: false
  show_inactive: false
  node_channel: lts
update:
  enabled: true
  min_release_age: 72h
notify:
  desktop: true
server:
  addr: 127.0.0.1:8788
```

Data: `~/.local/share/toolcheck/toolcheck.db` (or `$XDG_DATA_HOME`).

## 13. CLI

`toolcheck version | check [--refresh] [--json] | list [--json] [--all] |
update <tool> [--to V] [--dry-run] [--yes] [--force] | serve [--addr] |
service install|uninstall | config path`. `help`, `-h` and `--help` print usage
and exit 0.

## 14. Security and privacy

- HTTP server binds loopback only; Host/Origin validation and CSRF tokens copied
  from alertme.
- Outbound requests only to fixed official release sources; payloads are tool
  names, never paths or versions of installed packages.
- No telemetry, accounts, API keys, or elevated commands.
- Update commands are executed as the invoking user, with argv arrays.

## 15. Testing

- Table-driven detection tests with temporary PATH trees and fake executables.
- Origin classification tests for apt (injectable dpkg probe), corepack, nvm,
  pipx, uv, bun, root-owned prefixes.
- Source clients tested against `httptest` fixtures, including malformed and
  oversized responses.
- Version comparison tests per scheme (semver, `go1.x`, `v1.x`, prerelease).
- Update engine tests with fake command runners: success, verification failure,
  rollback, non-writable refusal, apt refusal, age gate.
- Web handler/template smoke tests; CSRF/Host guards.
- `go test -race ./...`, `go vet ./...`, linux amd64/arm64 builds.

## 16. Distribution

GoReleaser v2, `linux/amd64` + `linux/arm64` only, CGO disabled, Apache-2.0,
systemd user unit via `toolcheck service install`. GitHub repo private for now.
