# Changelog

All notable changes to codec are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and codec uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added (YAML)

- `codec yaml identify|outline|path|fmt|convert|flatten`: detect 13 file
  types (Kubernetes, Helm chart/template/values, Kustomize, Argo
  Workflows, Argo CD, GitHub Actions, GitLab CI, Azure Pipelines,
  Compose, Ansible playbook/inventory), outline, cursor path in five
  notations, comment-preserving re-indent, YAML↔JSON, flatten/unflatten,
  anchor/merge-key resolution.
- Template-aware parsing: Helm/Go-template, Jinja, GitHub and Argo
  expressions are recognised instead of breaking the parse.
- Plain-English syntax errors with fix hints (tabs, bad indentation,
  unquoted colons, unclosed quotes) and duplicate-key detection.

### Changed

- Go module path is now `github.com/mahasenabheetha/codec/v2`; requires Go 1.26+.
- Building from source needs Node.js for the web UI (see README).

### Changed (web UI)

- Brand-new dark web UI replaces the v1 page: collapsible tool sidebar,
  tabs that keep state, Ctrl+K command palette, live transform-as-you-type
  in syntax-highlighted editors, resizable panes, and a status bar.
- JWT view shows claims as a table with expiry/validity at a glance;
  Ansible view adds line filtering and collapsible sections.
- New shortcuts: Alt+S (use output as input), Ctrl+B (sidebar), ? (help).
  Base64 swap also flips encode/decode.

### Added

- `/api/transform`: optional `indent` for json-pretty; structured `jwt`
  field in responses. `/app/` redirects to `/`.
- CI runs on Windows as well as Linux.

## [1.0.0] - 2026-09-26

First tagged release: codec as it stood before the 2.0 work began.

### Added

- CLI commands: `auto` (detect and transform), `b64 encode|decode` with
  `--url` for the URL-safe alphabet, `json pretty|min|validate`, and
  `jwt decode`. Input comes from an argument or stdin.
- `--copy` / `-c` global flag to also place output on the clipboard.
- `watch` mode: transforms recognizable clipboard content in place.
- Ansible `-vv` task log analysis: status, probable-cause diagnosis,
  summary chips, prettified commands, and severity-colored output.
- `serve`: local web UI on `127.0.0.1` with explicit modes,
  transform-on-paste, click-to-jump JSON errors, and PWA install support.
- `version` command, `--version` flag, and `GET /api/version`; the web
  UI footer shows the running version.
- Release pipeline: pushing a `v*` tag publishes binaries for Linux,
  Windows and macOS (amd64/arm64) to GitHub Releases via GoReleaser.

[Unreleased]: https://github.com/mahasenabheetha/codec/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/mahasenabheetha/codec/releases/tag/v1.0.0
