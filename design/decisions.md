# Decisions

Settled unless the user reopens them. Add new ones at the bottom:
`N. decision — reason (date)`.

1. codec 2.0 lives in the codec repo; v1 tools stay, YAML workbench is
   the flagship — one tool, one binary (2026-09-26).
2. Positioning: YAML-aware workbench + VS Code/Cursor companion, not an
   editor replacement (2026-09-26).
3. **Read-only.** codec never writes to workspace files; UI edits are
   in-memory what-if changes (2026-09-26).
4. No config files in user repos. Personal settings (recent folders,
   environment/values profiles) live in the OS user config dir
   (`os.UserConfigDir()/codec`) (2026-09-26).
5. Backend: Go. Engine pure, adapters do I/O (2026-09-26).
6. Frontend: Svelte 5 (runes) + Vite + TypeScript, plain SPA — no
   SvelteKit. Source in `frontend/`, built into `internal/web/dist`,
   embedded via `go:embed` (2026-09-26).
7. Editor: CodeMirror 6; language intelligence (diagnostics, hover,
   completion, outline) served by the Go engine, LSP-shaped (2026-09-26).
8. **Dark theme only.** Colors still defined as CSS tokens (2026-09-26).
9. Helm: embed the Helm 4 Go SDK (no helm install needed). Verify the
   SDK's API at phase 05 start; fall back to Helm 3 SDK only if v4 is
   unsuitable (2026-09-26).
10. YAML parsing: `github.com/goccy/go-yaml` v1.19 — exact line+column
    errors, duplicate-key detection, token extents. Display-only output
    (fmt, JSON→YAML, resolved view) uses `go.yaml.in/yaml/v3`, which
    re-indents with comments intact (already an indirect dependency).
    Evaluated side by side at phase 02 start (2026-09-26).
11. Delivery: web UI via `codec serve` for all of v2; Docker image
    (ghcr.io) as secondary distribution; Wails desktop app is v3
    (2026-09-26).
12. Platforms: Windows (no admin rights) and macOS (Apple Silicon) are
    first-class; Linux supported. No installer; single binary
    (2026-09-26).
13. MVP (`v2.0.0-alpha.1`) = UI shell + v1 tools + read-only workspace
    + YAML viewer/intelligence + Helm render with values provenance
    (2026-09-26).
14. Lens order: Kubernetes/Helm → Argo (Workflows, ArgoCD) → CI
    pipelines → Ansible/Compose. Scaffolding comes after the lenses
    (2026-09-26).
15. Brand logos (Kubernetes, Helm, Argo, GitHub Actions, …) are bundled
    locally as SVG, never fetched at runtime; sources recorded in
    `frontend/src/assets/logos/SOURCES.md` (2026-09-26).
16. All v2 work happens on one integration branch, `feature/mab/yaml-tools`;
    the user raises a single PR at the end. Per-phase branches are not
    used (2026-09-26). Superseded by #72 once v2 merges.
17. Icons: `@lucide/svelte` (the Svelte 5 package). Fonts: Inter and
    JetBrains Mono via `@fontsource-variable/*`, bundled (2026-09-26).
18. Rich views get structured data from the API rather than re-parsing
    text in the browser: `/api/transform` returns `task` (Ansible) and
    `jwt` alongside `output` (2026-09-26).
19. Tool input/output is never persisted to browser storage (may contain
    secrets); only UI preferences are (2026-09-26).
20. Positions: `Offset` is a byte offset into the content with any BOM
    removed; `Line`/`Col` are 1-based and `Col` counts runes (what an
    editor shows). Astral characters (emoji) may differ by one column in
    UTF-16 editors; acceptable (2026-09-26).
21. Scalars are typed with the YAML 1.2 core schema: `yes`/`on` are
    strings, not booleans. The Norway problem is a lint warning (phase
    07), not a parse-time reinterpretation (2026-09-26).
22. Template expressions are masked to same-length text before parsing:
    whole-line control flow becomes `#` + spaces (a comment, harmless
    inside block scalars), inline expressions become
    `x…x`; values keep the original expression text and are marked
    `Templated`. Duplicate-key checks are skipped in documents with
    template control flow (branches repeat keys legitimately) (2026-09-26).
23. Host check accepts loopback names on any port (Docker may remap the
    port) plus the `--host` address; Origin check accepts loopback on any
    port (the Vite dev server). The token is what protects `/api/v2`
    (2026-09-26).
24. The SSE stream takes the token as `?token=` because EventSource
    can't send headers; no other endpoint accepts it that way (2026-09-26).
25. Polling is automatic inside containers (`/.dockerenv`,
    `/run/.containerenv`) and when native watches can't be set up, not
    whenever `--root` is given: native users of `--root` keep instant
    events (2026-09-26).
26. The tree is listed with `os.ReadDir` and background classification
    reads walked paths directly: on Windows `os.Root` opens serialise
    (2.3k files: 1 s vs 0.3 s on 22 cores). Every client-requested path
    is still read through `os.Root`. New dependency: fsnotify (2026-09-26).
27. The tree is served before YAML files are classified; types follow
    via the `tree` event. Parse trees are cached only for files opened in
    the UI (caching all cost ~70 MB on a 5k-file repo) (2026-09-26).
28. File-type logos come from Simple Icons (CC0), copied into
    `frontend/src/assets/logos/`; Azure Pipelines (no Microsoft marks
    there) uses a generic icon in the Azure colour (2026-09-26).
29. Editor features are optional provider interfaces (`Hoverer`,
    `Definer`, `Completer`, `Diagnoser`) with generic fallbacks, not
    methods every provider must implement: detection-only built-ins stay
    tiny and lenses add only what they know (2026-09-26).
30. "Copy diff" is computed server-side by `textdiff.Patch` (Myers):
    the editor holds LF text without a BOM, the patch keeps the file's
    BOM and per-line endings so `git apply` works on CRLF/mixed files
    (2026-09-26).
31. The editor applies an analysis only when it describes the current
    text; marks and diagnostics move with edits until the next one.
    Edits reach the session after a 100 ms pause and analysis waits
    150 ms plus up to 450 ms by file size, so a keystroke never copies or
    re-renders the whole document (2026-09-26).
32. Known cosmetic issue: CodeMirror's YAML grammar mis-colours the
    first character after a whole-line `{{- … }}`; the engine overlays
    are correct. A template-aware highlighting mode belongs to phase 05
    (2026-09-26).
33. Helm values: the SDK computes the merged values (they are what Helm
    renders with, by construction). Provenance is a lookup, not a second
    merge: for each final path, every layer that defines it, highest
    precedence first (--set, -f files, chart defaults; parent globals
    before a subchart's own). Tests pin chains for lists, null deletion,
    subcharts and globals (2026-09-26).
34. Rendering runs `action.Install` in client-only dry-run mode, exactly
    as `helm template` does, so output is byte-identical (verified with
    the v4.3.0 CLI built from the same module; opt-in test via
    HELM_BIN). Cost: binary 12.2 MB → 45.7 MB stripped; the engine alone
    already needs client-go (≈36 MB), so re-implementing Install to save
    ~10 MB isn't worth the drift risk (2026-09-26).
35. Helm templates use CodeMirror's line-based legacy YAML mode
    (`@codemirror/legacy-modes`) plus the engine's expression overlays;
    this resolves #32 (2026-09-26).
36. Provenance hovers live where they are exact: the merged-values view
    and `.Values.*` expressions in templates (from the chart's last
    render). A rendered field has no reliable link back to a value, so
    it gets none (2026-09-26).
37. The `.Values` reference check warns only for unguarded uses (not in
    an if/with/range on the same path, nor with default/required/hasKey
    and friends); unused chart defaults are infos (2026-09-26).
38. Docker image: GoReleaser `dockers_v2` on distroless `static:nonroot`,
    copying the already-built binaries (no build stage, no QEMU). Docs
    always publish the port on `127.0.0.1`: the server binds 0.0.0.0
    inside the container, and anyone who can load the page gets its
    token (2026-09-26).
39. Saved tabs are tied to the folder they were opened in; on a first
    visit (no folder recorded yet) a file tab opened by the URL is kept,
    so links to `#/file/…` work (2026-09-26).
40. Schemas: santhosh-tekuri/jsonschema v6 (pure Go, all drafts, error
    tree with instance locations; brings golang.org/x/text). Sources:
    yannh "standalone-strict" Kubernetes schemas per version (unknown
    fields are errors, catching typos), datreeio CRD catalog, SchemaStore
    entries for GitHub Actions, GitLab CI, Azure Pipelines, Compose,
    Kustomization. Downloaded on first use into `os.UserCacheDir()/codec/
    schemas` (versioned Kubernetes files never refetched, others weekly;
    404s remembered); proxy via HTTPS_PROXY. No network proxy setting:
    the user confirmed none is needed (2026-09-26).
41. Default target Kubernetes version 1.34 (selectable 1.30–1.37); it
    drives both schemas and the embedded deprecation table (2026-09-26).
42. Schema quirks handled in codec, not by editing schemas: oneOf/anyOf
    errors show only the branch that got furthest (types merged);
    templated/runtime-expression values are never type-checked; Azure
    Pipelines ignores scalar-vs-scalar type mismatches (Azure converts
    them); enums Kubernetes only states in prose ("One of …") feed
    completion and hover, never validation (2026-09-26).
43. Lint levels (error/warning/info/off per rule), target version and
    schema options live in user settings and apply everywhere: editor,
    workspace lint, Helm output, `codec yaml lint` (exit 2 on warnings or
    errors; infos don't fail). Raw Helm templates get no Kubernetes
    checks (their output does); Helm test hooks are skipped (2026-09-26).
44. Semantic diff identity: Kubernetes documents pair by kind +
    namespace/name (two single-document files always pair); list items
    by the first of name/key/mountPath/containerPort/port/id that is
    present and unique on both sides; leftovers pair when ≥ 50% of their
    leaves match (a renamed container is one change). Scalar lists are
    sets, except command/args/entrypoint/script, which are compared as a
    whole ("reordered" when only the order changed) (2026-09-26).
45. Query uses gojq; positions come from running path(expr) first, and
    only non-path expressions fall back to plain values (pointing at
    their document). $ENV/env are empty: queries see files, not the
    machine. A document an expression errors on is skipped (2026-09-26).
46. Diff ignore patterns are globs over the displayed path ("*" = any
    run of characters), stored in user settings (`diffIgnore`) and
    shared by the Compare view and `codec yaml diff` (2026-09-26).
47. Graph layout: @dagrejs/dagre (maintained dagre fork, ~48 KB ESM,
    layered layout that suits dependency graphs), drawn by our own SVG
    component so styling stays on tokens; shared by later lenses
    (2026-09-26).
48. Kustomize runs in-process (sigs.k8s.io/kustomize/api, already in the
    module graph via Helm) on an in-memory filesystem holding only the
    files the kustomization reaches (resources, bases, components,
    patches, generator inputs), so what-if edits apply and nothing is
    written. Reorder "unspecified" like kubectl. Remote bases and
    helmCharts are refused (no downloads, no helm binary) (2026-09-26).
49. Relationships are resolved within a scope (folder, render, build);
    a target outside it is a warning, never an error, and well-known
    objects (cluster-admin/view/edit/admin, system:*, default service
    account, kube-root-ca.crt) and optional refs aren't reported. An
    unset namespace matches any, since renders often omit it
    (2026-09-26).
50. Lenses register themselves in `provider.Default` from their
    package's init, wrapping the built-in detector they replace
    (`Registry.Lookup`); web and cli import them (2026-09-26).
51. Argo values are resolved, never evaluated: a value is split into
    parts (literal, resolved, run time, expression, Helm template,
    missing). Step outputs, workflow.uid/status, event data and
    `{{=expr}}` stay as written. `when` is evaluated only when every
    part is known, with a small evaluator (comparisons, =~, && || !);
    otherwise it says "known at run time". Loops expand up to 20
    iterations, named like Argo (`build(0:linux)`) (2026-09-26).
52. `workflow.parameters` always means the submitted workflow's, also
    inside templateRef'd templates; a WorkflowTemplate's own
    spec.arguments apply only when it is submitted. The entrypoint's
    inputs bind from workflow arguments by name. A library's
    entrypoint input with no argument is a warning, not an error
    (callers pass it) (2026-09-26).
53. Workflows a Sensor submits are first-class: arguments filled from
    event data are run-time values, with the dependency filter's
    accepted values as a hint. Filters are regexes, so they are never
    treated as the value (2026-09-26).
54. The Argo index spans the folder's Argo files, raw Helm templates
    included (names are usually literal; Helm escapes like
    ``{{ `{{…}}` }}`` are undone, Go-template values show "render the
    chart"). It is built on demand, caching which files are Argo files
    by mtime+size; the Helm view gets an Argo tab for exact values
    (2026-09-26).
55. "Render this app" links an Argo CD source to a chart in the folder
    by path (suffix match) or by Chart.yaml name; value files resolve
    relative to the chart, inline values become a virtual layer that
    is never saved in a profile (2026-09-26).
56. CI effective configurations are merged with an origin per entry
    (key nodes are copied per tree and tagged). GitLab order: anchors
    (per file) → include (deep merge, the including file wins) →
    `!reference` (against the merged raw config, before extends) →
    extends (deep merge in order, the job wins, lists replace) →
    default:/old top-level defaults for unset keys and global
    variables, as `inherit:` allows; script lists are flattened
    (2026-09-27).
57. Graph edges are the real execution order. GitLab: `needs` where
    given (`needs: []` starts at once), otherwise every earlier-stage
    job not already waited for through another. GitHub: a local
    reusable workflow's jobs replace the caller (`caller/job`); its
    first jobs wait for the caller's needs, jobs needing the caller
    wait for its last jobs. Azure: stages in order unless dependsOn,
    jobs in parallel unless dependsOn (2026-09-27).
58. Remote, project, template and component includes, remote reusable
    workflows and templates in other repositories are listed, never
    fetched. Names they might define are info ("may come from …")
    unless a local near miss (≤ 3 edits, swaps count as one) suggests
    a typo (2026-09-27).
59. Azure `${{ }}` template expressions are evaluated from parameters
    (defaults or what-if) and statically set variables; `$(macro)` and
    `$[ ]` are run time and never evaluated. An undecidable `if` keeps
    its branch, marked "only if …"; `each` over an unknown collection
    shows nothing (info) (2026-09-27).
60. The repository root of a pipeline file is the folder above
    `.github/`, else the nearest folder holding `.git` (an opened
    folder may hold several repositories); the CLI defaults to the
    folder holding `.git` (2026-09-27).
61. The layered merge with origins lives in `yamlkit` (`Origins`,
    `MergeRules`, `Emit`) and serves CI and Compose; a merged key takes
    the overriding side's key node, so file and line agree. Compose
    rules: maps merge, `command`/`entrypoint`/`healthcheck` replace,
    ports/volumes/secrets/configs/devices merge by what they are, other
    lists add missing items, list-form environment/labels merge by
    name, `!reset` removes, `!override` replaces (2026-09-27).
62. Compose `${VAR}` comes from what-if values, then the env file
    (`.env` next to the first file, or one chosen); the process
    environment is never read. Default layers are what `docker
    compose` reads: `compose.yaml` and its override; other Compose
    files in the folder can be added as layers (2026-09-27).
63. Ansible roles are searched in roles/ next to the playbook,
    `roles_path` from the nearest ansible.cfg, roles/ in folders up to
    the repository root, then a folder of that name. A role not there
    but in another repository of the open folder is read from there
    (info); otherwise it is info (installed at run time; a
    requirements file listing it is named) unless a near miss makes it
    a warning — as #58 (2026-09-27).
64. Ansible variable definitions are ranked by Ansible's precedence
    list (role defaults 2 … role params 20); group_vars/host_vars are
    collected from the repository without knowing which hosts run, and
    say so. A logged task is found by its task path (longest shared
    tail of path segments), else by name, `{{ }}` matching any value
    when the name has at least 4 literal characters (2026-09-27).
65. Starters are `text/template` sets with `<% %>` delimiters (`{{ }}`
    and `[[ ]]` belong to the files themselves). Built-ins are embedded;
    personal ones are read from `<settings>/templates` (a folder per
    starter with an optional starter.yaml, or single files whose fields
    are the placeholders they use). Output is only text to copy, a
    what-if preview, or a zip the browser saves (2026-09-27).
66. Clone renames whole-word uses of the old name (letters and digits
    bound a word) in values and keys, except label/annotation keys.
    A field naming another object is renamed only when that object
    (kind and name) is part of the copy; Secrets, images, hosts and
    addresses are suggestions; images, hosts and Secret references are
    listed to check (2026-09-27).
67. Snippets come from the backend per file type (embedded YAML in
    CodeMirror placeholder syntax), offered where a key or list item
    starts. The Argo DAG task snippet depends on the last task;
    depends/dependencies/template complete from the DAG (2026-09-27).
68. The sample repository is embedded in the binary and written to
    `<user cache>/codec/sample` (files put back if changed), then opened
    like any folder: no second file system in the workspace. It isn't a
    recent folder; the UI reopens it after a restart (2026-09-27).
69. One Settings view holds every setting. Settings saved on the server
    (profile folder) are shared with the CLI; display choices (editor
    link, panels) stay in browser storage (2026-09-27).
70. Performance budgets (phase 14): files up to 8 MB are listed and
    read; documents parse in parallel from 16 up; an outline keeps at
    most 20,000 symbols (fewer levels, then one per document); trees
    over 300 rows draw only what is in view (2026-09-27).
71. Files a kustomization lists as patches (same folder or up to two
    above) skip "required field" schema errors. The Helm view waits at
    most 1 s for schemas and re-renders while `schemasPending`
    (2026-09-27).
72. After v2 merges, `main` is the base. Work happens on short-lived
    branches with pull requests; the phase files were compacted into
    history.md and new work is planned in roadmap.md (2026-09-27).
73. One branch per release: `feature/mab/vX.Y.Z` from `main`, holding
    that version's tasks in order (one or more commits each), merged by
    PR and tagged on `main`; then the next version branches from `main`.
    Fixes for a released version go on `fix/<topic>` as a patch release.
    New features are minor releases. Refines 72 (2026-10-03).
74. Compare takes pasted text as a third source per side (File · Helm
    render · Paste), not a separate screen. Auto mode compares by
    structure only when both sides are mappings or lists; anything else
    gets a text diff. Pasted text is never stored (19) (2026-10-03).
75. Find in Files sits beside Query — text vs structure — instead of
    replacing it. It searches exactly the files the Explorer lists,
    through the root guard, with no index and no replace (3)
    (2026-10-03).
76. The log analyzer takes big logs as a dropped or picked file, sent
    to the server as is (not through an editor box), with a 64 MB
    limit on that endpoint only; pasting stays for smaller logs.
    Nothing is stored (19) (2026-10-03).
77. The playbook map is coloured by a loaded log run with generic
    matching only: a task's task path (file, or a role's folder), else
    its role prefix, else its play; handlers by name. No playbook
    evaluation; nodes no task reached show "never ran" (2026-10-03).
