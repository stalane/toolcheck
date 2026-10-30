# toolcheck

Local-first freshness checks for the package managers and runtimes on a Linux
developer machine that apt does not manage. `toolcheck` detects npm, pnpm, yarn,
bun, deno, uv, node, and friends, compares each install with its upstream release
channel, and — only for a small safe set — applies updates after confirmation.

## Why

apt keeps the OS and distro-packaged tools patched. It does not touch tools you
installed with `npm -g`, corepack, bun's installer, `uv`'s installer, or a
version manager. Those drift quietly. toolcheck shows what you have, what is
newer, how it was installed, and whether it is safe for toolcheck to update it.

## Install

```sh
go install github.com/stalane/toolcheck/cmd/toolcheck@latest
```

Or download a Linux release binary from GitHub. Linux only: amd64 and arm64.

## Quick start

```sh
toolcheck check          # detect tools and fetch upstream versions
toolcheck list           # show installed tools and their state
toolcheck serve          # dashboard on http://127.0.0.1:8788
toolcheck service install # systemd user unit
```

The dashboard only shows tools that are installed. Duplicate copies on `PATH`
are collapsed to the active one; `toolcheck list --all` shows the rest.

## Coverage

| Family | Tools |
|---|---|
| Node/JS | node, npm, pnpm, yarn, corepack, bun, deno |
| Version managers | nvm, fnm, volta, asdf, mise, pyenv, rbenv, RVM, sdkman, jenv, g, gvm, goenv |
| Python | python3, pip, pipx, uv, poetry, pdm, pixi, hatch, conda, micromamba |
| Rust | rustup, cargo, rustc, cargo-binstall |
| Go | go |
| Ruby | ruby, gem, bundler |
| PHP | composer |
| .NET | dotnet |
| AI | ollama |
| Alternative package managers | brew (Linuxbrew), nix, snap, flatpak |

Tools with no reliable upstream endpoint show the installed version and a
manual/system-managed state; toolcheck never invents a latest version.

## Update policy

toolcheck updates **only** user-writable, self-managed installs, after an
explicit confirmation, and only with these adapters:

| Tool | Command |
|---|---|
| npm | `npm install -g npm@<version>` |
| uv | `uv self update <version>` |
| bun | `bun upgrade` (latest only) |
| deno | `deno upgrade --version <version>` |
| composer | `composer self-update <version>` |
| rustup | `rustup self update` (latest only) |

Everything else is alert-only: apt-managed tools, corepack shims (pnpm/yarn),
node and all version managers, pip/pipx, poetry/pdm/pixi/hatch, conda, go,
cargo/rustc, ruby/gem, sdkman, dotnet, ollama, and brew/nix/snap/flatpak.

Safety rules:

- Never uses `sudo` or elevates; a non-writable install is refused.
- Never runs a shell string; commands are argument arrays.
- Re-reads the installed version before applying and aborts on drift.
- Refuses a release younger than `update.min_release_age` (default 72h) when the
  upstream publish time is known; `--force` overrides.
- Verifies the new version and rolls back where the tool supports it.
- Never auto-applies. `--yes` or the web confirmation is required.

## Configuration

`~/.config/toolcheck/config.yaml`:

```yaml
check:
  interval: 24h
  timeout: 15s
  cache_ttl: 24h
tools:
  include: []            # empty = all
  exclude: []
  hide_os_managed: false
  show_inactive: false
  node_channel: lts      # lts or current
update:
  enabled: true
  min_release_age: 72h
notify:
  desktop: true
server:
  addr: 127.0.0.1:8788
```

Data lives in `~/.local/share/toolcheck/` (or `$XDG_DATA_HOME/toolcheck`).
`toolcheck config path` prints the config location.

## Privacy

toolcheck contacts official release endpoints only: the npm registry, PyPI,
nodejs.org, go.dev, deno.land, getcomposer.org, rust-lang.org, GitHub release
redirects, anaconda.org, rubygems.org, and Microsoft's .NET release index. It
sends tool names, never paths or installed package data. No telemetry, account,
or API key.

## Limitations

- Linux only, with Debian/Ubuntu apt detection. On other distros, system
  packages are detected but treated as externally managed.
- apt-managed tools are shown for information; apt handles their updates.
- Version freshness is not malware detection. A newer manager is not a promise
  that every package it installs is safe.
- npm's abbreviated registry metadata omits publish timestamps, so the minimum
  release age is unknown for npm-family tools; those updates still require
  confirmation and are verified.
- Some upstream projects publish only tags, not releases; those show no latest
  version rather than a guess.

## Development

```sh
make check       # go vet + go test + go build
go test -race ./...
```

Design and implementation notes live under `docs/superpowers/`.

## License

Apache-2.0. Release data belongs to its upstream projects.
