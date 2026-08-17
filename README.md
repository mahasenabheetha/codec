# codec

[![CI](https://github.com/mahasenabheetha/codec/actions/workflows/ci.yml/badge.svg)](https://github.com/mahasenabheetha/codec/actions/workflows/ci.yml)

A fast, single-binary CLI for the transformations a DevOps engineer does all day: **base64 encode/decode, JSON pretty-print/minify/validate, JWT inspection** — with auto-detection and clipboard integration.

Copy a Kubernetes secret, get readable JSON. Paste a JSON payload, get base64. Decode a JWT without pasting it into a website.

## Features

- **Auto-detect** — `codec auto` figures out whether input is JSON, base64, or a JWT and applies the obvious transformation. Base64 that decodes to JSON is pretty-printed automatically (the Kubernetes secret case).
- **Watch mode** — `codec watch` monitors the clipboard: copy JSON anywhere, paste base64; copy base64, paste decoded JSON. No terminal round-trip.
- **Clipboard flag** — `-c` on any command copies the output to the system clipboard as well as printing it.
- **Lenient input** — tolerates wrapped lines, surrounding whitespace, and missing base64 padding (as found in JWTs and k8s manifests).
- **Positioned JSON errors** — invalid JSON reports `line 3, column 8`, not a useless byte offset.
- **Pipe-friendly** — data on stdout, commentary on stderr, meaningful exit codes. Drops cleanly into CI jobs and shell pipelines.
- **Single static binary** — no runtime, no dependencies, no installer. Cross-compiles to any OS/arch.

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

Input comes from an argument or stdin — both work everywhere:

```bash
codec auto '{"a":1}'
echo '{"a":1}' | codec auto
```

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

### Base64

```bash
codec b64 encode 'some text'          # standard alphabet
codec b64 encode --url 'some text'    # URL-safe alphabet (-_ instead of +/)
codec b64 decode aGVsbG8=             # tolerates missing padding & wrapped lines
```

### JSON

```bash
codec json pretty '{"a":{"b":1}}'     # two-space indent (--indent to change)
codec json min    "$(cat big.json)"   # single line
codec json validate '{"a":}'          # exit code 1 + "line 1, column 6: ..."
```

### JWT

```bash
codec jwt decode "$TOKEN"
```

Prints the decoded header and payload. **Does not verify the signature** — this is an inspection tool, not an authentication library.

### Watch mode

```bash
codec watch                # poll every 300ms; Ctrl+C to stop
codec watch --interval 1s
```

While running, anything recognizable you copy is transformed and placed back on the clipboard, ready to paste. Content it doesn't recognize (prose, URLs, passwords) is left untouched and unlogged.

> **Note:** while watch mode is active, *all* recognizable clipboard content is transformed — including JSON you may have wanted to paste as JSON. Run it deliberately during batch work rather than all day. An on-demand hotkey mode is planned (see Roadmap).

### Clipboard flag

```bash
kubectl get secret db -o jsonpath='{.data.password}' | codec b64 decode -c
```

Prints the result *and* copies it. Clipboard failures (e.g. headless environments) are warnings, never errors.

### PowerShell users

Windows PowerShell 5.1 strips inner double quotes from arguments passed to native executables. Pipe instead:

```powershell
'{"name":"mahasen"}' | .\codec.exe auto -c
```

PowerShell 7+ (`pwsh`) does not have this problem.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | invalid input, unrecognized content, or usage error |

This makes `codec json validate` usable as a CI gate:

```yaml
validate-payloads:
  script:
    - codec json validate < payload.json
```

## Architecture

```
cmd/codec/          main() — 5 lines, calls the CLI layer
internal/cli/       presentation: cobra commands, flags, stdin/stdout,
                    clipboard, exit codes. Contains no encoding logic.
internal/codec/     the engine: base64, JSON, JWT, detection. Pure
                    functions, no I/O, no knowledge of how it's invoked.
```

The split is deliberate: `internal/codec` is fully unit-tested and UI-agnostic, so future front-ends (web UI, desktop app, system tray) reuse it unchanged. `internal/` is compiler-enforced private — nothing outside this module can import the engine.

## Development

```bash
go test ./...        # run all tests
go vet ./...         # static analysis
go build ./...       # compile everything
```

Tests are table-driven and live next to the code they test. CI runs vet, tests, and a full build on every push and pull request.

Contributions follow a PR workflow: branch from `main`, open a pull request, let CI pass, merge. No direct pushes to `main`.

## Roadmap

- [x] Stage 1 — CLI with auto-detect, clipboard flag, watch mode
- [ ] Stage 2 — local web UI (Go + embedded static page)
- [ ] Stage 3 — native desktop app (Wails, reusing the web UI)
- [ ] Stage 4 — system tray + global hotkey + on-demand clipboard transform

## Acknowledgements

This project was developed as a hands-on Go learning journey by [Mahasen Abheetha](https://github.com/mahasenabheetha), with development assistance from **Claude (Anthropic)** used as a pair-programming and teaching tool. Design decisions, code review, and explanations were AI-assisted; the learning was not.
