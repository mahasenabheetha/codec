# Architecture

## Layout

```
cmd/codec/            main → cli.Execute()
frontend/             Svelte 5 + Vite + TS source (Wails-compatible location)
internal/
  codec/              ENGINE v1 transforms (base64, JSON, JWT, ansible)
  yamlkit/            ENGINE YAML core: parse, positions, docs, detect,
                      outline, paths, format, convert, diagnostics,
                      semantic diff, layered merge with origins (CI, Compose)
  names/              ENGINE "did you mean" (edit distance) for every lens
  query/              ENGINE jq (gojq) over documents, results mapped to lines
  provider/           ENGINE provider interface + registry
  helm/               ENGINE Helm 4 SDK render + values provenance (from
                      in-memory files; the SDK is the only big dependency)
  textdiff/           ENGINE Myers diff, git-applicable patches
  lint/               ENGINE style rules, Kubernetes checks, API deprecations
  schema/             ENGINE schema choice per doc, JSON Schema validation,
                      schema-driven completion and hover
  kube/               ENGINE Kubernetes lens: objects, relationships graph,
                      inventory, cards, neat, Secrets, Kustomize (memfs);
                      registers its provider over the built-in detector
  argo/               ENGINE Argo lens: workflow specs (incl. Sensor-submitted),
                      cross-file index, parameter resolution, depends parser,
                      when evaluator, checks, Argo CD apps; registers providers
  ci/                 ENGINE CI lens: GitHub Actions (matrix, reusable workflows,
                      composite actions), GitLab CI (includes, !reference,
                      extends, default: merged with origins), Azure Pipelines
                      (templates, ${{ }} evaluator); registers providers
  ansible/            ENGINE Ansible lens: playbooks in execution order, roles
                      (roles_path, dependencies), includes, handlers, variable
                      precedence, YAML inventories, log task → definition
  compose/            ENGINE Compose lens: files merged with the Compose rules and
                      origins, extends/include, ${VAR} from .env + what-if,
                      services graph, ports, volumes, checks
  workspace/          ADAPTER read-only folder access, index, watcher
  config/             ADAPTER user settings in os.UserConfigDir()/codec
  schemacache/        ADAPTER schema fetch + cache (os.UserCacheDir()/codec/
                      schemas), offline mode, custom schema folder
  check/              ADAPTER runs yamlkit + provider + lint + schema: one
                      path for editor, workspace lint, Helm output and CLI
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
registering one with the same ID); no core changes. Always LSP-shaped:

```go
type Provider interface {                // since phase 02
    ID() string                          // "kubernetes", "helm-template", ...
    Title() string
    Detect(f *File) Confidence           // 0 = not mine … 100 = certain
    Symbols(d *yamlkit.Document) []Symbol
}
// Optional capabilities (phase 04), found by type assertion; generic
// implementations answer when a provider has none (decision #29):
type Hoverer   interface{ Hover(f *File, pos yamlkit.Pos) *Hover }
type Definer   interface{ Definition(f *File, pos yamlkit.Pos) []Location }
type Completer interface{ Complete(f *File, pos yamlkit.Pos) []Completion }
type Diagnoser interface{ Diagnostics(f *File) []yamlkit.Diagnostic }
// Later: Renderer, Grapher, Generator; an Index for cross-file lookups (05).
```

`provider.Default` holds the built-ins; `Registry.Analyze` bundles type,
outline, diagnostics and expressions for the editor. `yamlkit` provides
the tree (`File` → `Document` → `Node` with exact `Range`s), expressions
and diagnostics every provider works from.

Concepts deliberately mirror LSP so a future `codec lsp` can reuse them.
`Index` is the workspace-wide entity graph (charts, templates by name,
resources, jobs, references) built by the workspace adapter.

## Web API

- Existing: `POST /api/transform`, `GET /api/version` (keep working).
- New endpoints under `/api/v2/…`, JSON in/out, grouped by feature
  (`/workspace`, `/files`, `/yaml`, `/helm`, …). Each phase defines its own.
  Editor calls (`/yaml/analyze|hover|definition|complete|path`,
  `/files/diff`) take `{path, content?, line, col}`; no content = the file
  on disk. Requests are cancelled via `r.Context()` when the editor moves on.
  Helm: `GET /helm/charts`, `POST /helm/render` {chart, values[], set[],
  overrides{path: content}, release, namespace, kubeVersion},
  `POST /helm/profiles` (saved in user settings, keyed by chart path).
  Lint: `GET|POST /lint/settings` (rules catalogue + user levels, target
  Kubernetes version, schema options), `POST /lint/workspace` (every YAML
  file from disk). `/yaml/analyze` includes lint and schema findings and
  each document's schema status (`pending` = ask again); editor calls may
  pass `type` so schema help works while the buffer doesn't parse.
  Compare/query: `POST /compare` {left, right, ignore[]} where a side is
  {kind: file|helm, path, content?, doc?, chart, profile} → semantic
  changes with ranges on both texts (text diff if a side doesn't parse);
  `GET|POST /compare/ignore` (saved patterns); `POST /query` {expr, scope:
  file|workspace|helm, …side} → results with file, jq path, value, range.
  Kubernetes: `POST /k8s/analyze` {kind: folder|text|kustomize|helm, …} →
  cards, inventory, graph, problems; `POST /k8s/neat`. Argo: `POST
  /argo/analyze` {kind: file|text, path|text, workflow, params, paramFile,
  overrides} → workflows and apps in scope, the chosen workflow resolved
  (nodes, edges, values in parts); `/yaml/hover|definition|analyze` also
  resolve templateRef across the folder's Argo files.
  CI: `POST /ci/analyze` {path, params, overrides} → stages, jobs
  (steps, matrix, effective config with origins), edges, includes,
  problems; `/yaml/hover|definition|analyze` follow includes and
  templates in the repository around the file.
  Compose: `POST /compose/analyze` {path, files, env, envFile, overrides}
  → services (effective config with the file per line), edges, ports,
  mounts, variables, problems. Ansible: `POST /ansible/analyze` {path,
  overrides} → playbook (plays, steps, roles, variables) or inventory;
  `POST /ansible/find-task` {name, path} → definitions of a logged
  task. `/yaml/hover|definition|analyze` read the folder for both.
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
  on Windows), or forced with `--poll`. Image: `ghcr.io/mahasenabheetha/codec`
  (distroless, non-root; run with `-p 127.0.0.1:8765:8765`), see releases.md.
