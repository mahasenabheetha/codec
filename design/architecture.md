# Architecture

## Layout

```
cmd/codec/            main → cli.Execute()
frontend/             Svelte 5 + Vite + TS source (Wails-compatible location)
internal/
  codec/              ENGINE v1 transforms (base64, JSON, JWT, ansible)
  yamlkit/            ENGINE YAML core: parse, positions, docs, detect,
                      outline, paths, format, convert, diagnostics
  provider/           ENGINE provider interface + registry
  kube/ helm/ argo/   ENGINE lenses (one package per domain; ci/, ansible/,
  ci/ ansible/        compose/ added in their phases)
  compose/
  workspace/          ADAPTER read-only folder access, index, watcher
  config/             ADAPTER user settings in os.UserConfigDir()/codec
  version/            build identity
  cli/                ADAPTER cobra commands
  web/                ADAPTER HTTP API + embedded frontend (web/dist/app)
```

Engine packages take bytes/structs and return structs. They never touch
the filesystem, network, clock, or env. Adapters load files and hand
content to the engine.

## Provider model (future-proofing)

Every YAML domain implements one interface in `internal/provider`;
methods it doesn't support return "not supported". Adding a file type =
adding a provider, no core changes.

```go
type Provider interface {
    ID() string                                // "kubernetes", "helm", "argo", ...
    Detect(f File) Confidence                  // 0 = not mine
    Symbols(d *Doc) []Symbol                   // outline
    Diagnostics(d *Doc, ix *Index) []Diagnostic
    Hover(d *Doc, pos Pos, ix *Index) *Hover
    Definition(d *Doc, pos Pos, ix *Index) []Location
    Complete(d *Doc, pos Pos, ix *Index) []Completion
}
// Optional extras via separate interfaces: Renderer, Grapher, Generator.
```

Concepts deliberately mirror LSP so a future `codec lsp` can reuse them.
`Index` is the workspace-wide entity graph (charts, templates by name,
resources, jobs, references) built by the workspace adapter.

## Web API

- Existing: `POST /api/transform`, `GET /api/version` (keep working).
- New endpoints under `/api/v2/…`, JSON in/out, grouped by feature
  (`/workspace`, `/files`, `/yaml`, `/helm`, …). Each phase defines its own.
- Errors: `{"error": "...", "line"?, "column"?, "file"?}` with 400 for
  caller bugs, 422 for bad input, 404 missing, 500 unexpected.
- Push updates (file changes) via Server-Sent Events: `GET /api/v2/events`.
- The frontend calls the backend only through `frontend/src/lib/api/`,
  so Wails bindings can replace HTTP in v3.

## Security (local server that reads files)

- Bind `127.0.0.1` by default; `--host` only for Docker.
- Reject requests whose `Host` isn't `127.0.0.1|localhost:<port>`
  (DNS-rebinding defense).
- Per-run random token injected into `index.html`; required as header
  `X-Codec-Token` on `/api/v2/*`. Check `Origin` on non-GET requests.
- File access limited to opened workspace roots; resolve symlinks and
  reject paths escaping the root. Never write to workspace files.

## Runtime modes

- Native: `codec serve [--port 8765]` — primary.
- Docker: `codec serve --host 0.0.0.0 --root /work`, repo mounted `:ro`;
  watcher uses polling (bind-mount events are unreliable on Windows).
