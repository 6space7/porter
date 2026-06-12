# Decisions

This log records product and engineering decisions that are not already fixed by the project brief.

## 2026-06-12

- The project is named `porter`.
- The initial repository uses `main` as the default branch.
- Phase 1 source installs use `/etc/porter` for config and `/var/lib/porter`
  for data so the binary, installer, and docs share one production path model.
- The installer generates an initial bearer token, stores only its SHA-256 hash
  in `/etc/porter/porter.env`, and prints the plaintext token once. This matches
  the current token-first API while leaving password login for a later UI/auth
  slice.
- Managed Caddy is enabled by default for installed servers and can be disabled
  with `PORTER_MANAGE_CADDY=false` for local development or external proxy
  setups.
