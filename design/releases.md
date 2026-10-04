# Versioning and releases

## Versioning

- [Semantic Versioning](https://semver.org/), tags prefixed with `v`
  (`v1.0.0`). History in [CHANGELOG.md](../CHANGELOG.md) (Keep a Changelog).
- `v1.0.0` = codec before the 2.0 work (tagged on `main` at `1e428f0`).
- Pre-releases use a suffix: `v2.0.0-alpha.1`, `-beta.1`.
- A `release/vN` branch is created from a tag only if an old major
  version needs a hotfix.
- Since v2 the Go module path is
  `github.com/mahasenabheetha/codec/v2` (required for
  `go install ...@v2.x`).

## Version in the binary

- `internal/version` holds `Version`, `Commit`, `Date`, stamped by
  `-ldflags -X` in release builds.
- Unstamped builds fall back to Go build info: a real tag if built at
  one, otherwise `dev` plus the git commit.
- Exposed via `codec version`, `codec --version`, `GET /api/version`,
  and the web UI footer.

## Cutting a release

1. Merge the release changes to `main` via PR; update `CHANGELOG.md`.
2. Tag the merge commit on `main` and push the tag:
   ```bash
   git tag -a v1.2.3 -m "codec v1.2.3"
   git push origin v1.2.3
   ```
3. `.github/workflows/release.yml` runs vet + tests, then GoReleaser
   (`.goreleaser.yaml`) builds linux/windows/darwin × amd64/arm64,
   archives them (`zip` on Windows, `tar.gz` elsewhere), and publishes
   a GitHub Release with `checksums.txt`. Suffixed tags become
   pre-releases automatically. The same run pushes the Docker image.
   A second job on Windows then builds the desktop app
   (`scripts/desktop.sh`) and adds `codec-desktop_<v>_windows_amd64.exe`
   (portable), `…_setup.exe` (per-user installer) and
   `codec-desktop_<v>_checksums.txt` to the release. They are unsigned
   until SignPath signing is set up ([roadmap](roadmap.md#code-signing)),
   so Windows shows "unknown publisher" on first run.
4. First release with an image only: the GHCR package starts private.
   Make it public once: GitHub → Packages → codec → Package settings →
   Change visibility.

Local dry runs (output in `dist/`, gitignored):

- `goreleaser build --snapshot --clean --single-target`: one binary.
- `goreleaser release --snapshot --clean --skip=archive`: all six
  binaries plus the Docker images, loaded into the local daemon and not
  pushed (~10 min on Windows). The before-hook runs `npm ci`, which
  fails if a running vite dev server holds `node_modules`; stop it, or
  build the frontend yourself and add `before` to `--skip`.
- `scripts/desktop.sh`: the desktop exe and installer in
  `dist/desktop/` (NSIS 3 needed: `makensis` on PATH or `MAKENSIS=`;
  the portable zip from SourceForge needs no admin).

## Docker image

- `dockers_v2` in `.goreleaser.yaml` builds `ghcr.io/mahasenabheetha/codec`
  for linux/amd64 and linux/arm64 from the already-compiled binaries.
  The `Dockerfile` only copies `<os>/<arch>/codec`: no Go build, no QEMU.
- Base `gcr.io/distroless/static-debian12:nonroot`: no shell, user
  65532, HOME `/home/nonroot` (settings land in `.config/codec` there).
  About 70 MB, almost all of it the binary.
- Entrypoint `codec serve --host 0.0.0.0 --root /work`, `EXPOSE 8765`.
  codec detects the container (`/.dockerenv`, `/run/.containerenv`) and
  polls for file changes; native events don't cross bind mounts from
  Windows or macOS hosts.
- Tags: `{{ .Version }}` (e.g. `2.0.0-alpha.1`) always; `latest` only
  when the tag has no pre-release suffix.
- The release workflow has `packages: write`, logs in to ghcr.io with
  `GITHUB_TOKEN` and sets up buildx.
- Documented run: `-p 127.0.0.1:8765:8765` (not reachable from the
  network) and `-v <repo>:/work:ro`; `-v codec-home:/home/nonroot`
  keeps settings between runs.
- GHCR storage and bandwidth are free for public images.

## GitHub Releases: cost and limits

- Free on all plans. Assets do not count toward repo size, LFS, or
  Packages quotas, and never expire.
- Each file must be under 2 GiB; up to 1,000 assets per release; no
  total size or stated bandwidth cap.
- Actions minutes: unlimited for public repos; 2,000 min/month on the
  free plan for private repos. A codec release takes ~1–2 min.
- Actions *artifacts* are different: they use storage quota and expire
  (90 days default). codec publishes releases, not artifacts.
- Stable download URLs:
  - `.../releases/download/v1.0.0/<file>`
  - `.../releases/latest/download/<file>` (skips pre-releases)
- Deleting a release does not delete its tag.

Limits as understood in 2026-09; confirm in GitHub's "About releases"
and "Billing for GitHub Actions" docs if it matters.

## Documentation site

- `docs/` is served by GitHub Pages from `main` (Settings → Pages →
  Deploy from a branch → `main`, `/docs`; `docs/.nojekyll` keeps files
  as they are). It updates when changes merge; no workflow is needed.
- Before a release, check that pages describing changed behaviour and
  their screenshots are current (design/workflow.md, "What to update").
