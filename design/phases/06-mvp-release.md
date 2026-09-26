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

- [x] Snapshot release builds binaries + Docker image locally
  (`goreleaser release --snapshot --clean --skip=archive`, 10 min;
  amd64 + arm64 images, labels, non-root user).
- [x] Docker: open `/work`, browse, render a chart, live refresh via
  polling (edit + new file on the host), Host/token guards, settings
  kept in a `/home/nonroot` volume. ~21 MB RAM idle.
- [ ] Native on Windows (no admin) and macOS arm64 verified by the user.
- [ ] After merge, user tags `v2.0.0-alpha.1`; release marked pre-release.

## Manual test checklist (for the PR)

Native (Windows without admin, and macOS arm64):
- [ ] Download/unpack; Windows SmartScreen or macOS `xattr` note works;
  `codec version` runs.
- [ ] `codec serve --open --root <repo>`: tree honours `.gitignore`,
  type labels and logos, Ctrl+P, a file tab refreshes after an edit
  in VS Code.
- [ ] Editor: hover, F12 on an alias, Outline/Problems, path copy,
  what-if edit → Copy diff → `git apply --check` accepts it; typing
  stays smooth on a ~5k-line file.
- [ ] Helm view on a real chart: add a values file, `--set`, save and
  delete a profile, provenance hover, what-if edit re-renders; output
  equals `helm template` for the same inputs.
- [ ] `codec helm render|values --provenance` and `codec yaml …` on a
  real repo.
- [ ] v1 tools: smart paste, base64, JSON, JWT, Ansible log.

Docker:
- [ ] `docker run --rm -p 127.0.0.1:8765:8765 -v "${PWD}:/work:ro"
  ghcr.io/mahasenabheetha/codec:2.0.0-alpha.1` on a real repo: browse,
  render a chart, edit a file on the host and see the tab refresh.

## Notes

GHCR packages start private — make it public once in GitHub settings.

**Go concepts:** none new; release engineering.
