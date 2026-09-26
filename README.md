# codec

[![CI](https://github.com/mahasenabheetha/codec/actions/workflows/ci.yml/badge.svg)](https://github.com/mahasenabheetha/codec/actions/workflows/ci.yml)

A fast, single-binary toolbox for the transformations a DevOps engineer does all day: **base64 encode/decode, JSON pretty-print/minify/validate, JWT inspection, and Ansible log analysis** — available as a CLI, a clipboard watcher, and a local web app you can install like a desktop application.

Copy a Kubernetes secret, get readable JSON. Paste an Ansible `-vv` failure, get the root cause in a banner. Everything runs locally; nothing ever leaves your machine.

## Features

- **Auto-detect** — paste anything; codec figures out whether it's JSON, base64, a JWT, or an Ansible task log and applies the obvious transformation. Base64 containing JSON is pretty-printed automatically.
- **Explicit modes** — encode/decode (standard or URL-safe alphabet), pretty/minify/validate JSON, decode JWTs, parse Ansible logs. Explicit modes accept *any* text, no detection required.
- **Ansible log analysis** — parses `-vv` task output into a structured view: status badge, probable-cause banner (the engine's diagnosis of *why* it failed), summary chips (`rc`, `msg`, timings) with problems highlighted, prettified commands (one `--flag` per line), severity-colored stderr/stdout, an errors-only filter, and support for loop items, retries, skipped tasks, and wrapper-prefixed logs (Packer, CI pipelines).
- **Web UI** — `codec serve` hosts a fast, dark, keyboard-first app: a collapsible sidebar of tools, tabs that keep each tool's state, a Ctrl+K command palette, live transform-as-you-type with syntax-highlighted editors, resizable panes, click-to-jump JSON errors, rich JWT (claims, expiry) and Ansible views. Installable as a PWA: it gets its own window and taskbar icon.
- **Watch mode** — `codec watch` monitors the clipboard: copy JSON anywhere, paste base64; copy base64, paste decoded JSON.
- **Clipboard flag** — `-c` on any CLI command also copies the output.
- **Lenient input** — tolerates wrapped lines, whitespace, missing base64 padding, ANSI color codes in logs.
- **Positioned JSON errors** — invalid JSON reports `line 3, column 8` and the web UI jumps your cursor there on click.
- **Pipe-friendly** — data on stdout, commentary on stderr, meaningful exit codes; drops cleanly into CI jobs.
- **Single static binary** — the web frontend is embedded via `go:embed`; ship one file, no runtime, no installer.

## Install

Download a prebuilt binary for Windows, Linux or macOS from [GitHub Releases](https://github.com/mahasenabheetha/codec/releases), unzip, and run. Check what you have with `codec version`.

Or build from source (Go 1.26+, Node.js 22+ for the web UI):

```bash
git clone https://github.com/mahasenabheetha/codec.git
cd codec
npm --prefix frontend ci && npm --prefix frontend run build
go build -o codec ./cmd/codec
```

Cross-compile for other platforms:

```bash
GOOS=windows GOARCH=amd64 go build -o codec.exe ./cmd/codec
GOOS=linux   GOARCH=arm64 go build -o codec-arm  ./cmd/codec
```

Or open the repo in VS Code with the Dev Containers extension — a ready-made Go + Node environment is included under `.devcontainer/`.

## Usage

### Web UI

```bash
codec serve                     # http://localhost:8765
codec serve --open              # …and open it in the browser
codec serve --root ~/repos/app  # open a folder right away
codec serve --port 9000 --poll  # other port; poll for file changes
```

**Workspace (read-only).** Open a repository folder (Ctrl+O, or `--root`) to browse it: a file tree that honours `.gitignore` (and skips `.git`, `node_modules`, binaries and files over 2 MB), with each YAML file labelled by type (Kubernetes, Helm, Argo, GitHub Actions, GitLab CI, Compose, Ansible, …). Files open in tabs as YAML-aware editors: template and runtime expressions are tinted, errors are squiggled with a fix hint, hover shows a value's path and type, F12 or Ctrl+click jumps from an alias to its anchor, and an Outline/Problems panel follows the cursor. The status bar shows the path at the cursor; click it to copy it as dot, yq, JSONPath, Helm or `--set`. Edits are **what-if only**: never saved, with Reset, Copy content and Copy diff (a patch for `git apply`). Tabs refresh by themselves when the file changes on disk, and "Open in VS Code/Cursor" jumps to the same line. codec never writes to the folder; recent folders are remembered in your profile (`%AppData%\codec`, `~/Library/Application Support/codec`, `~/.config/codec`), never in the repo.

**Tools.** Pick a tool from the sidebar (or press Ctrl+K and type), then paste or type — output updates as you go. **Smart paste** auto-detects; the other tools are explicit.

| Shortcut | Action |
|---|---|
| Ctrl+K | Search tools and actions |
| Ctrl+P | Go to file |
| Ctrl+O | Open folder |
| Ctrl+Shift+E | Toggle the explorer |
| F12 / Ctrl+click | Go to definition (YAML alias → anchor) |
| Ctrl+G · Ctrl+F | Go to line · find in file |
| Ctrl+Enter | Run the transform |
| Alt+C | Copy output |
| Alt+S | Use output as input |
| Esc | Clear the tool |
| Ctrl+B | Toggle the sidebar |
| ? | Show all shortcuts |

On macOS, Ctrl is ⌘. Nothing you paste is stored — only layout preferences are remembered.

**Install as an app:** in Edge, menu → Apps → *Install this site as an app* (Chrome: install icon in the address bar). codec gets its own window, taskbar icon, and Start-menu entry. Note the server (`codec serve`) must be running for the app to work — there is deliberately no offline cache, because the "site" *is* the local binary.

The server binds to `127.0.0.1` only, rejects requests for other host names (DNS rebinding) and requires a per-run token on the workspace API, so web pages you visit can't use it. In Docker, run `codec serve --host 0.0.0.0 --root /work` with the repo mounted read-only; file changes are then detected by polling.

### Ansible log analysis

Paste a failed (or successful) `ansible -vv` task block into the web UI — auto-detect handles it — or pipe it through the CLI:

```bash
codec auto < failed-task.log
```

You get: probable cause up top, `rc`/`msg` chips, the command prettified one flag per line, and stderr with SEVERE/ERROR lines highlighted. Lines that declare their own level (`- INFO:`) are trusted over keyword guessing, so an INFO line mentioning "not found" stays neutral.

### Auto-detect

```bash
$ codec auto '{"name":"mahasen"}'
detected: json
eyJuYW1lIjoibWFoYXNlbiJ9

$ codec auto eyJuYW1lIjoibWFoYXNlbiJ9
detected: base64
{
  "name": "mahasen"
}
```

### Base64 / JSON / JWT

```bash
codec b64 encode 'any text at all'     # --url for the URL-safe alphabet
codec b64 decode aGVsbG8=              # tolerates missing padding
codec json pretty '{"a":{"b":1}}'      # --indent to customize
codec json min < big.json
codec json validate '{"a":}'           # exit 1 + "line 1, column 6"
codec jwt decode "$TOKEN"              # does NOT verify the signature
```

### YAML

Works on Kubernetes manifests, Helm templates, CI pipelines, Compose and Ansible files — template expressions (`{{ … }}`, `${{ … }}`, `{% … %}`) are understood rather than reported as errors. Output goes to stdout; files are never modified.

```bash
codec yaml identify deploy.yaml             # file type, documents, problems (file:line:col)
codec yaml outline deploy.yaml              # key/item tree with line numbers
codec yaml path deploy.yaml --line 26 --col 12 --style all
#   dot       spec.template.spec.containers[0].image
#   yq        .spec.template.spec.containers[0].image
#   jsonpath  $.spec.template.spec.containers[0].image
#   helm      (index .Values.spec.template.spec.containers 0).image
#   set       spec.template.spec.containers[0].image=
codec yaml path values.yaml --get image.tag # value and its position
codec yaml fmt messy.yaml --k8s-order       # re-indent, comments kept
codec yaml fmt anchors.yaml --resolve       # expand anchors and << merges
codec yaml convert values.yaml --to json    # key order kept (and --to yaml)
codec yaml flatten values.yaml              # path: value lines; --reverse to undo
```

Syntax errors are explained in plain English with a fix, e.g. `Tab character used for indentation` or `A value contains ": " but isn't quoted`, and duplicate keys are flagged.

### Helm

Renders charts with the embedded Helm 4 SDK — the same output as `helm template`, without installing helm. Nothing is downloaded: vendor dependencies into `charts/` first.

```bash
codec helm render ./charts/app -f values-prod.yaml --set image.tag=1.27   # like helm template
codec helm values ./charts/app -f values-prod.yaml --provenance           # merged values, each line annotated
#   replicaCount: 3  # ← values-prod.yaml:1 (overrides values.yaml:6)
```

In the web UI, open a chart from the palette (**Helm: render …**) or the **Render** button on any chart file: pick values layers, `--set` and release options (save them as a profile), and see the rendered manifests, the merged values with where each came from, NOTES and problems. Edits in open tabs render live as what-if.

### Watch mode

```bash
codec watch                # poll every 300ms; Ctrl+C to stop
codec watch --interval 1s
```

While running, anything recognizable you copy is transformed and placed back on the clipboard. Content it doesn't recognize is left untouched. Run it deliberately during batch work — while active, *all* recognizable clipboard content is transformed.

### PowerShell note

Windows PowerShell 5.1 strips inner double quotes from arguments to native executables. Pipe instead: `'{"a":1}' | .\codec.exe auto`. PowerShell 7+ doesn't have this problem.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | invalid input, unrecognized content, or usage error |

Usable as a CI gate:

```yaml
validate-payloads:
  script:
    - codec json validate < payload.json
```

## Architecture

```
cmd/codec/          main() — calls the CLI layer
frontend/           Svelte 5 + TypeScript web UI (built into internal/web/dist)
internal/codec/     engine: base64, JSON, JWT, ansible parsing, detection
internal/yamlkit/   engine: template-aware YAML parsing, positions, paths
internal/provider/  engine: file-type providers (Kubernetes, Helm, …)
internal/workspace/ adapter: read-only folder access, .gitignore, watcher
internal/config/    adapter: settings in the user profile
internal/cli/       presentation: cobra commands, flags, exit codes
internal/web/       presentation: HTTP API, security guard, events, embedded UI
```

The engine defines *what things are* (including per-line severity of log output); the presentation layers decide what that looks like (colors, exit codes, HTTP statuses). New front-ends reuse the engine unchanged — that's how the CLI, web UI, and watch mode share one implementation. `internal/` is compiler-enforced private.

The v1 API is `POST /api/transform` (`{"input", "mode", "urlSafe", "indent"}` → `{"output", "kind"}` plus structured `task`/`jwt`). The workspace API lives under `/api/v2/` (workspace, folder browser, file tree, file content, and a Server-Sent Events stream of file changes); see `design/architecture.md`.

## Development

```bash
go test ./...        # unit tests, incl. real-log ansible fixtures
go vet ./...
go build ./...

npm --prefix frontend ci         # once: install frontend dependencies
npm --prefix frontend run dev    # new UI with hot reload (API proxied to codec serve)
npm --prefix frontend run build  # build into internal/web/dist, embedded by go build
```

Or let `scripts/dev.sh` (bash; Git Bash on Windows) do it all: `build` (frontend + `./codec-dev`), `run [port]` (build and serve, default 8766), `ui [port]` (build, serve, and the hot-reload UI at http://localhost:5173/).

The web UI (Svelte 5 + TypeScript, in `frontend/`) is embedded into the binary. A binary built without the frontend still compiles and shows a "frontend not built" page instead.

Tests are table-driven; the Ansible parser's test suite is built from real logs, and every parsing bug fixed becomes a named regression test. CI builds the frontend, then runs vet, tests and build on Linux and Windows for every push and PR. Changes go through pull requests — no direct pushes to `main`.

Design notes, conventions and plans for contributors and AI agents live in [design/](design/README.md).

### Versioning and releases

codec follows [Semantic Versioning](https://semver.org/); every release is listed in [CHANGELOG.md](CHANGELOG.md). A release is cut by tagging a commit that is already on `main`:

```bash
git tag -a v1.2.3 -m "codec v1.2.3"
git push origin v1.2.3
```

The tag triggers the Release workflow, which runs vet and tests, then uses [GoReleaser](https://goreleaser.com/) to build Linux, Windows and macOS binaries with the version stamped in, and publishes them with checksums as a GitHub Release. Tags with a suffix (`v2.0.0-alpha.1`) are published as pre-releases. Local builds report `dev` plus their git commit.

## Roadmap

- [x] Stage 1 — CLI with auto-detect, clipboard flag, watch mode
- [x] Stage 2 — local web UI: explicit modes, Ansible log analysis, PWA
- [ ] Stage 3 — native desktop app (Wails, reusing the web UI)
- [ ] Stage 4 — system tray + global hotkey + on-demand clipboard transform

Parked ideas: Kubernetes Secret manifest decoder, JWT claims view with expiry countdown, recursive decode (base64-in-base64, gzip), multi-task Ansible runs with PLAY RECAP scoreboard, duration-gap timing analysis, user-defined error-hint rules, copy-as-markdown error summaries, URL/hex/YAML codecs, JSON diff.

## Acknowledgements

This project was developed as a hands-on Go learning journey by [Mahasen Abheetha](https://github.com/mahasenabheetha), with development assistance from **Claude (Anthropic)** used as a pair-programming and teaching tool. Design decisions, code review, and explanations were AI-assisted; the learning was not.
