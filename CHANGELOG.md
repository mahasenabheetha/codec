# Changelog

All notable changes to codec are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and codec uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added (lint and schemas)

- Lint rules with a why and a fix for every finding: YAML 1.1 traps
  (yes/no/on/off, octal-looking numbers), duplicate keys, indentation
  consistency, empty values, trailing spaces, and optional document
  start and line length.
- Kubernetes checks: unpinned images, missing requests/limits, missing
  readiness probe, privileged containers, hostPath, hostNetwork, running
  as root; removed and deprecated APIs for a target Kubernetes version
  (1.30–1.37, default 1.34).
- Schema validation for Kubernetes (per version), CRDs (Argo, Argo CD,
  cert-manager, … from the CRD catalog), GitHub Actions, GitLab CI,
  Azure Pipelines, Compose and Kustomization — errors on the right line,
  with "did you mean" for misspelled fields. Schemas download on first
  use and are cached; offline mode and a custom schema folder.
- Editor: schema key and value completion (also mid-edit, with docs),
  field descriptions on hover, a badge naming the schema in use; problems
  show why they matter and can turn their rule off.
- Workspace problems view (lint every YAML file), Lint settings view
  (levels per rule, Kubernetes version, schema options; kept in your
  settings), and lint + schema findings on Helm's rendered output.
- `codec yaml lint [paths] [--k8s-version] [--offline] [--no-schemas]
  [--json]`: exit 2 when there are warnings or errors.

## [2.0.0-alpha.1] - 2026-09-26

First 2.0 pre-release: the read-only YAML workbench (workspace, editor,
Helm) on top of the v1 tools, as native binaries and a Docker image.

### Added (release)

- Docker image `ghcr.io/mahasenabheetha/codec` for linux/amd64 and
  linux/arm64 (distroless, non-root): mount a repository read-only at
  `/work`; file changes are detected by polling. Version tags on every
  release, `latest` on stable ones only.
- README rewritten for 2.0 with screenshots, Docker quick start and
  notes for Windows SmartScreen and macOS Gatekeeper.

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

### Added (Helm)

- Render charts with the embedded Helm 4.3 SDK — byte-for-byte the same
  output as `helm template`, no helm install, no cluster, no downloads
  (dependencies must be vendored in `charts/`, `.tgz` or folders).
- Helm view per chart: values layers (add, reorder, toggle), `--set`,
  release/namespace/Kubernetes version, saved profiles (kept in your
  settings), rendered manifests grouped by template, NOTES, problems
  with jump-to-line, and live re-render including what-if edits from
  open tabs.
- Values provenance: every merged value says which layer set it and what
  it overrode (hover in the Values view, or on `.Values.*` in a
  template); `null` deletions, subchart defaults and globals included.
- Checks: `.Values` paths used but set nowhere (unguarded uses only),
  defaults no template uses, template errors mapped to file:line, and a
  note where `lookup` returns empty without a cluster.
- `codec helm render <chart> -f … --set …` and
  `codec helm values <chart> -f … --provenance`.
- Helm templates get template-aware syntax highlighting.

### Added (editor)

- YAML files open in an editor: Helm/Jinja (template-time) and Argo/
  GitHub (runtime) expressions tinted, errors squiggled with plain-English
  hints, hover cards (path, type, value, anchors, YAML 1.1 gotchas,
  expression meaning), go to definition for aliases (F12, Ctrl/⌘+click),
  alias completion after `*`, go to line (Ctrl+G).
- Outline and Problems panel synced with the cursor; status-bar path
  breadcrumb that copies the path as dot, yq, JSONPath, Helm or `--set`;
  document switcher for multi-document files.
- What-if edits: change anything, never saved; Reset, Copy content and
  Copy diff (a patch that `git apply` accepts, keeping CRLF/BOM). A disk
  change during edits asks whether to reload or keep them.
- Open the file at the cursor in VS Code or Cursor.
- Friendly error for a line indented under a key that already has a
  value (the parser used to blame the wrong line).

### Added (workspace)

- Open a repository folder read-only (Ctrl+O, recent folders, folder
  browser with drives on Windows, or `codec serve --root`): file tree
  that honours `.gitignore`, file-type logos and a type filter, Ctrl+P
  fuzzy Go to file, files in read-only tabs that refresh on disk change.
- `codec serve --host --root --poll --open`. Native file watching with a
  polling fallback (automatic inside containers).
- Security for the local server: Host-header check (DNS rebinding),
  per-run token on `/api/v2`, Origin check on state-changing requests,
  and file access confined to the opened folder (symlink escapes too).
- Settings (recent folders) live in the user profile, never in a repo.

### Fixed

- A link straight to a file (`#/file/…`) opened on a first visit no
  longer lands on the home page.
- "File not found" errors name the path instead of the raw OS message.
- Hover on a Helm template expression no longer mentions Ansible.
- A YAML file using Go templates without Helm's objects (e.g.
  `.goreleaser.yaml`) is no longer labelled a Helm template.

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

[Unreleased]: https://github.com/mahasenabheetha/codec/compare/v2.0.0-alpha.1...HEAD
[2.0.0-alpha.1]: https://github.com/mahasenabheetha/codec/compare/v1.0.0...v2.0.0-alpha.1
[1.0.0]: https://github.com/mahasenabheetha/codec/releases/tag/v1.0.0
