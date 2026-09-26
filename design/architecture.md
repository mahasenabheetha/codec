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

Every YAML domain implements one interface in `internal/provider`.
Adding a file type = adding a provider (or replacing a built-in by
registering one with the same ID); no core changes. The interface grows
phase by phase, always LSP-shaped:

```go
// Since phase 02 (built-ins: detection + generic outline, 13 types + plain YAML)
type Provider interface {
    ID() string                          // "kubernetes", "helm-template", ...
    Title() string
    Detect(f *File) Confidence           // 0 = not mine … 100 = certain
    Symbols(d *yamlkit.Document) []Symbol
}
// Phase 04 adds: Diagnostics, Hover, Definition, Complete (with an Index).
// Optional extras via separate interfaces: Renderer, Grapher, Generator.
```

`provider.Default` holds the built-ins; `yamlkit` provides the tree
(`File` → `Document` → `Node` with exact `Range`s), expressions and
diagnostics every provider works from.

Concepts deliberately mirror LSP so a future `codec lsp` can reuse them.
`Index` is the workspace-wide entity graph (charts, templates by name,
resources, jobs, references) built by the workspace adapter.

## Web API

- Existing: `POST /api/transform`, `GET /api/version` (keep working).
- New endpoints under `/api/v2/…`, JSON in/out, grouped by feature
  (`/workspace`, `/files`, `/yaml`, `/helm`, …). Each phase defines its own.
- Errors: `{"error": "...", "line"?, "column"?, "file"?}` with 400 for
  caller bugs, 422 for bad input, 404 missing, 500 unexpected.
- Push updates via Server-Sent Events, `GET /api/v2/events`: `workspace`
  (folder opened / watch mode known), `files` (added/changed/removed
  paths), `tree` (file types classified; refetch the tree).
- The frontend calls the backend only through `frontend/src/lib/api/`,
  so Wails bindings can replace HTTP in v3.

## Security (local server that reads files)

Implemented in `internal/web/security.go`; every route passes the guard.

- Bind `127.0.0.1` by default; `--host` only for Docker.
- Reject requests whose `Host` isn't a loopback name (`localhost`,
  `127.0.0.1`, `::1`, any port: Docker may remap it) or the `--host`
  address (DNS-rebinding defense).
- Per-run random token injected into `index.html`; required as header
  `X-Codec-Token` on `/api/v2/*` (the SSE stream alone may pass
  `?token=`, since EventSource can't set headers). Non-GET requests with
  a foreign `Origin` are rejected.
- Client-supplied paths are slash paths validated with `fs.ValidPath`
  and read through `os.Root`, which refuses `..` and symlink escapes.
  The tree walk lists with `os.ReadDir` (no symlinked directories
  followed); background classification reads walked paths directly and
  exposes only the file type (decision #26). Never write to workspace files.

## Runtime modes

- Native: `codec serve [--port 8765] [--root DIR] [--open]` — primary;
  fsnotify watches every directory, polling if that fails.
- Docker: `codec serve --host 0.0.0.0 --root /work`, repo mounted `:ro`;
  polling is automatic in containers (bind-mount events are unreliable
  on Windows), or forced with `--poll`.
