# Security policy

## Reporting a vulnerability

Email **security@stalane.com**. Do not open a public issue for an unpatched vulnerability.

Please include the toolcheck version (`toolcheck version`), your Linux distribution, and reproduction steps.

## What toolcheck does with your data

- Outbound requests go only to fixed official release endpoints and contain tool
  names, never paths or installed package data.
- The web UI binds to `127.0.0.1` and rejects non-loopback `Host` and foreign
  `Origin` headers.
- Mutating web actions require a per-process CSRF token.
- Update commands run as your user; toolcheck never elevates privileges, never
  invokes sudo, and never uses a shell string.
- apt-managed and root-owned installs are alert-only.

## Supported versions

The latest release receives security fixes.
