# 06 — MVP release (`v2.0.0-alpha.1`)

**Goal:** ship phases 00–05 as a usable pre-release, native + Docker.
**Depends on:** 05 · **Branch:** `feature/mab/yaml-tools`
**Read:** releases.md

## Scope

- GoReleaser Docker image: linux amd64+arm64, minimal static base,
  `EXPOSE 8765`, entrypoint `codec serve --host 0.0.0.0 --root /work`,
  pushed to `ghcr.io/mahasenabheetha/codec`. Version tags always;
  `latest` only for stable releases. Release workflow gets
  `packages: write` + GHCR login.
- README rewritten for v2: what it does, screenshots, quick start
  (native binary, Docker run with `:ro` mount), Windows SmartScreen and
  macOS quarantine (`xattr -d com.apple.quarantine codec`) notes.
- CHANGELOG entry; `releases.md` gains the Docker section.
- Bug-bash 00–05; manual test checklist for the user in the PR.

## Acceptance

- [ ] Snapshot release builds binaries + Docker image locally/CI.
- [ ] Docker: open `/work`, browse, render a chart, live refresh via polling.
- [ ] Native on Windows (no admin) and macOS arm64 verified by the user.
- [ ] After merge, user tags `v2.0.0-alpha.1`; release marked pre-release.

## Notes

GHCR packages start private — make it public once in GitHub settings.

**Go concepts:** none new; release engineering.
