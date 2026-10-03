# Architecture

The contributor view of how codec is built. The illustrated overview,
flow charts and performance numbers are in `docs/architecture.html` and
`docs/flows.html`; decisions behind these rules are in
[decisions.md](decisions.md).

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
  textdiff/           ENGINE Myers diff, git-applicable patches, loose
                      (whitespace/case) text diffs, side-by-side rows
                      with changed characters
  search/             ENGINE Find in Files: phrase/regex, globs, capped,
                      deterministic; the caller supplies files and reads
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
  ansiblelog/         ENGINE whole Ansible logs from pipelines: clean (CI
                      prefixes, colours, learned wrappers), segment into runs and
                      other output, read results; the analysis model
  compose/            ENGINE Compose lens: files merged with the Compose rules and
                      origins, extends/include, ${VAR} from .env + what-if,
                      services graph, ports, volumes, checks
  scaffold/           ENGINE starters (embedded text/template sets, personal ones
                      as an fs.FS), clone-with-rename, editor snippets
  sample/             ADAPTER the embedded sample repository, written to the
                      user cache folder and opened like any folder
  workspace/          ADAPTER read-only folder access, index, watcher
  config/             ADAPTER user settings in os.UserConfigDir()/codec
                      (personal starters in its templates/ folder)
  schemacache/        ADAPTER schema fetch + cache (os.UserCacheDir()/codec/
                      schemas), offline mode, custom schema folder
  check/              ADAPTER runs yamlkit + provider + lint + schema: one
                      path for editor, workspace lint, Helm output and CLI
                      (knows Kustomize patches, which may be partial)
  version/            build identity
  cli/                ADAPTER cobra commands
  web/                ADAPTER HTTP API + embedded frontend (web/dist/app)
```

Engine packages take bytes/structs and return structs. They never touch
the filesystem, network, clock, or env. Adapters load files and hand
content to the engine.

## Provider model

Every YAML domain implements one interface in `internal/provider`.
Adding a file type = adding a provider (or replacing a built-in by
registering one with the same ID); no core changes. Always LSP-shaped:

```go
type Provider interface {
    ID() string                          // "kubernetes", "helm-template", ...
    Title() string
    Detect(f *File) Confidence           // 0 = not mine … 100 = certain
    Symbols(d *yamlkit.Document) []Symbol
}
// Optional capabilities, found by type assertion; generic
// implementations answer when a provider has none (decision #29):
type Hoverer   interface{ Hover(f *File, pos yamlkit.Pos) *Hover }
type Definer   interface{ Definition(f *File, pos yamlkit.Pos) []Location }
type Completer interface{ Complete(f *File, pos yamlkit.Pos) []Completion }
type Diagnoser interface{ Diagnostics(f *File) []yamlkit.Diagnostic }
type Expresser interface{ Expressions(f *File) []yamlkit.Expression }
```

`provider.Default` holds the built-ins; lens packages register their
providers over the built-in detectors [50]. `Registry.Analyze` bundles
type, outline (trimmed past 20,000 symbols [70]), diagnostics and
expressions for the editor; `check.Analyze` adds lint and schema
findings. `yamlkit` provides the tree (`File` → `Document` → `Node` with
exact `Range`s), expressions and diagnostics every provider works from.

Concepts deliberately mirror LSP so a future `codec lsp` can reuse them.

## Lenses

A lens is a view of a whole project, not one file (Helm, Kubernetes,
Argo, CI, Compose, Ansible). Each follows the same shape:

1. **Engine package** (`internal/<lens>`): reads the files it needs
   through a small reader interface or from bytes, resolves them, and
   returns a structured result plus problems with file/line sources.
2. **Provider** in the same package: outline, hover, definition,
   diagnostics for single files of that type; registered in
   `provider.Default`.
3. **Endpoint** `POST /api/v2/<lens>/analyze` in `internal/web/<lens>_api.go`,
   taking what-if `overrides` (open tabs' buffers) and the view's inputs.
4. **CLI** `codec <lens> …` with `--json` and exit 2 on problems.
5. **View** in `frontend/src/features/<lens>/` using `LensView` +
   `LensGrid`, opened by route `/<lens>/<path>`, a toolbar button on
   files of that type and palette entries (`App.svelte`).

## Frontend

```
frontend/src/
  App.svelte          routes → views, app-wide commands and shortcuts
  lib/api/            the only place that talks to the backend (one file per feature)
  lib/components/     shared building blocks (see design-language.md)
  lib/shell/          rail, tab bar, status bar, palette, shortcuts dialog
  lib/stores/         router (hash routes), layout/tabs, commands, shortcuts, toasts, persisted()
  lib/styles/         tokens.css (design tokens), global.css
  lib/tools.ts        encode/decode tools registry (rail, home, palette)
  features/<name>/    one folder per view or tool; cross-feature pieces in features/shared
```

Views are mounted per open tab and receive `active`; stores that
outlive a tab are `*.svelte.ts` modules using runes.

## Performance

Budgets (measured with `scripts/perf.sh`, fixtures from
`scripts/perfgen`): 10k-file folder open and listed ≤ 1 s, every file
typed ≤ 10 s, Go to file ≤ 100 ms per key, lint of 6,000 files ≤ 30 s;
5 MB file shown ≤ 2 s, typing ≤ 16 ms per key; 500-document chart render
≤ 2 s. Results at 2.0: `docs/architecture.html#performance`. Mechanisms:
parallel document parsing (≥ 16 docs), windowed trees (> 300 rows),
outline trimming, background classification, schema downloads started
together and never blocking a render (`schemasPending`) [70, 71].
Files over 8 MB are not listed or read.

## Web API

- Existing: `POST /api/transform`, `GET /api/version` (keep working).
- New endpoints under `/api/v2/…`, JSON in/out, grouped by feature
  (`/workspace`, `/files`, `/yaml`, `/helm`, …), one `*_api.go` per feature.
  Editor calls (`/yaml/analyze|hover|definition|complete|path`,
  `/files/diff`) take `{path, content?, line, col}`; no content = the file
  on disk. Requests are cancelled via `r.Context()` when the editor moves on.
  Helm: `GET /helm/charts`, `POST /helm/render` {chart, values[], set[],
  overrides{path: content}, release, namespace, kubeVersion} (→ the
  render; `schemasPending` = schema findings still to come, ask again),
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
  Scaffolding: `GET /scaffold/starters` (built-in + personal, the
  personal folder); `POST /scaffold/render` {id, values} → files, each
  linted and schema-checked, charts rendered, or field errors; `POST
  /scaffold/check` {files} after what-if edits; `POST /scaffold/zip`
  {name, files} → a zip download; `POST /scaffold/clone` {path,
  content?, doc, from, to, flip} → the renamed text, changes, checks.
  `/yaml/complete` adds snippets for the file type.
  Settings: `GET /settings` (codec's folders, Helm profiles of every
  chart, recent folders); `POST /settings/forget` {helm?: {chart,
  profile?}, recent?}. `POST /workspace/sample` opens the sample.
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
