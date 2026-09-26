# Versioning and releases

## Versioning

- [Semantic Versioning](https://semver.org/), tags prefixed with `v`
  (`v1.0.0`). History in [CHANGELOG.md](../CHANGELOG.md) (Keep a Changelog).
- `v1.0.0` = codec before the 2.0 work (tagged on `main` at `1e428f0`).
- Pre-releases use a suffix: `v2.0.0-alpha.1`, `-beta.1`.
- A `release/vN` branch is created from a tag only if an old major
  version needs a hotfix.
- From v2 the Go module path must become
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
   pre-releases automatically.

Local dry run: `goreleaser build --snapshot --clean --single-target`
(output in `dist/`, gitignored).

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
