# codec

[![CI](https://github.com/mahasenabheetha/codec/actions/workflows/ci.yml/badge.svg)](https://github.com/mahasenabheetha/codec/actions/workflows/ci.yml)

A fast, single-binary toolbox for the transformations a DevOps engineer does all day: **base64 encode/decode, JSON pretty-print/minify/validate, JWT inspection, and Ansible log analysis** — available as a CLI, a clipboard watcher, and a local web app you can install like a desktop application.

Copy a Kubernetes secret, get readable JSON. Paste an Ansible `-vv` failure, get the root cause in a banner. Everything runs locally; nothing ever leaves your machine.

## Features

- **Auto-detect** — paste anything; codec figures out whether it's JSON, base64, a JWT, or an Ansible task log and applies the obvious transformation. Base64 containing JSON is pretty-printed automatically.
- **Explicit modes** — encode/decode (standard or URL-safe alphabet), pretty/minify/validate JSON, decode JWTs, parse Ansible logs. Explicit modes accept *any* text, no detection required.
- **Ansible log analysis** — parses `-vv` task output into a structured view: status badge, probable-cause banner (the engine's diagnosis of *why* it failed), summary chips (`rc`, `msg`, timings) with problems highlighted, prettified commands (one `--flag` per line), severity-colored stderr/stdout, an errors-only filter, and support for loop items, retries, skipped tasks, and wrapper-prefixed logs (Packer, CI pipelines).
- **Web UI** — `codec serve` hosts a local page with side-by-side input/output panes, segmented mode selector, transform-on-paste, click-to-jump JSON error positions, swap, and keyboard-complete flow. Installable as a PWA: it gets its own window and taskbar icon.
- **Watch mode** — `codec watch` monitors the clipboard: copy JSON anywhere, paste base64; copy base64, paste decoded JSON.
- **Clipboard flag** — `-c` on any CLI command also copies the output.
- **Lenient input** — tolerates wrapped lines, whitespace, missing base64 padding, ANSI color codes in logs.
- **Positioned JSON errors** — invalid JSON reports `line 3, column 8` and the web UI jumps your cursor there on click.
- **Pipe-friendly** — data on stdout, commentary on stderr, meaningful exit codes; drops cleanly into CI jobs.
- **Single static binary** — the web frontend is embedded via `go:embed`; ship one file, no runtime, no installer.

## Install

Build from source (Go 1.23+):

```bash
git clone https://github.com/mahasenabheetha/codec.git
cd codec
go build -o codec ./cmd/codec
```

Cross-compile for other platforms:

```bash
GOOS=windows GOARCH=amd64 go build -o codec.exe ./cmd/codec
GOOS=linux   GOARCH=arm64 go build -o codec-arm  ./cmd/codec
```

Or open the repo in VS Code with the Dev Containers extension — a ready-made Go environment is included under `.devcontainer/`.

## Usage

### Web UI

```bash
codec serve            # http://localhost:8765
codec serve --port 9000
```

Paste into the input pane — transformation runs instantly on paste. Pick an explicit mode from the segmented control when auto-detect isn't what you want. Shortcuts: Ctrl+Enter run, Alt+C copy, Esc clear.

**Install as an app:** in Edge, menu → Apps → *Install this site as an app* (Chrome: install icon in the address bar). codec gets its own window, taskbar icon, and Start-menu entry. Note the server (`codec serve`) must be running for the app to work — there is deliberately no offline cache, because the "site" *is* the local binary.

The server binds to `127.0.0.1` only: nothing on your network can reach it.

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
cmd/codec/          main() — 5 lines, calls the CLI layer
internal/cli/       presentation: cobra commands, flags, stdin/stdout,
                    clipboard, exit codes
internal/web/       presentation: HTTP handlers, JSON API, embedded
                    frontend (vanilla JS, zero dependencies, no build step)
internal/codec/     the engine: base64, JSON, JWT, ansible parsing,
                    detection, mode dispatch. Pure functions, no I/O,
                    fully unit-tested, no knowledge of how it's invoked
```

The engine defines *what things are* (including per-line severity of log output); the presentation layers decide what that looks like (colors, exit codes, HTTP statuses). New front-ends reuse the engine unchanged — that's how the CLI, web UI, and watch mode share one implementation. `internal/` is compiler-enforced private.

The API is one endpoint: `POST /api/transform` with `{"input", "mode", "urlSafe"}` returning `{"output", "kind"}` plus a structured `task` object for Ansible results. Unknown mode → 400; untransformable input → 422 with `line`/`column` for JSON syntax errors.

## Development

```bash
go test ./...        # unit tests, incl. real-log ansible fixtures
go vet ./...
go build ./...
```

Tests are table-driven; the Ansible parser's test suite is built from real logs, and every parsing bug fixed becomes a named regression test. CI runs vet, tests, build, and a Windows cross-compile on every push and PR. Changes go through pull requests — no direct pushes to `main`.

## Roadmap

- [x] Stage 1 — CLI with auto-detect, clipboard flag, watch mode
- [x] Stage 2 — local web UI: explicit modes, Ansible log analysis, PWA
- [ ] Stage 3 — native desktop app (Wails, reusing the web UI)
- [ ] Stage 4 — system tray + global hotkey + on-demand clipboard transform

Parked ideas: Kubernetes Secret manifest decoder, JWT claims view with expiry countdown, recursive decode (base64-in-base64, gzip), multi-task Ansible runs with PLAY RECAP scoreboard, duration-gap timing analysis, user-defined error-hint rules, copy-as-markdown error summaries, URL/hex/YAML codecs, JSON diff.

## Acknowledgements

This project was developed as a hands-on Go learning journey by [Mahasen Abheetha](https://github.com/mahasenabheetha), with development assistance from **Claude (Anthropic)** used as a pair-programming and teaching tool. Design decisions, code review, and explanations were AI-assisted; the learning was not.
