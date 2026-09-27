# History

How codec 2.0 was built, compacted from the phase plans (the detailed
phase files were removed once v2 was done; git history has them).
Numbers in brackets are decisions in [decisions.md](decisions.md).

v1 (`v1.0.0`, tagged 2026-09-26) was the encode/decode CLI and a small
static web page. v2 turned codec into a read-only YAML workbench in
fifteen phases, all on the integration branch `feature/mab/yaml-tools`.

| # | Phase | What it delivered | Decisions |
|---|---|---|---|
| 00 | Foundation | `/v2` Go module (Go 1.26), Svelte 5 + Vite frontend embedded in the binary, CI on Linux and Windows, GoReleaser builds the frontend. | 1–12 |
| 01 | UI shell | Dark design system (tokens, fonts, icons), shell (rail, tabs, palette, status bar), v1 tools ported with rich JWT and Ansible-log views; the v1 page removed. | 8, 15–19 |
| 02 | YAML engine | `yamlkit`: template-aware parse with exact positions, plain-English errors, paths in five notations, fmt/convert/flatten/resolve; `provider` with 13 detectors; `codec yaml`. Verified on 2,290 real files. | 20–22 |
| 03 | Workspace | Read-only folder access through `os.Root`, `.gitignore`, watcher with polling fallback, secured `/api/v2` (host check, token, origin) and SSE; explorer, open-folder dialog, Go to file, live tabs. | 23–28 |
| 04 | Editor | Provider hover/definition/completion, analysis loop, expression tints, outline and problems, path breadcrumb, what-if edits with git-applicable diffs (`textdiff`), open in VS Code/Cursor. | 29–32 |
| 05 | Helm | Helm 4 SDK render from memory, byte-identical to `helm template`; values provenance; values checks; Helm view with layers, `--set` and profiles. | 33–37 |
| 06 | MVP release | Docker image (distroless, non-root, GHCR), README with screenshots, CHANGELOG `2.0.0-alpha.1`. | 38–39 |
| 07 | Lint and schemas | 8 style rules, 7 Kubernetes checks, API deprecations for 1.30–1.37; JSON Schema validation (Kubernetes, CRDs, CI, Compose) with a cache and offline mode; schema completion; Problems view; `codec yaml lint`. | 40–43 |
| 08 | Compare and query | Semantic diff (order-insensitive, identity pairing) and jq query (gojq) with positions; Compare and Query views; `codec yaml diff/query`. | 44–46 |
| 09 | Kubernetes | Objects, relationships, inventory, neat, Secret decode, in-process Kustomize; shared Graph component; Resources view. | 47–50 |
| 10 | Argo | Workflow resolution across files (templateRef, Sensors, CronWorkflows), depends parser, `when` evaluator, Argo CD apps with "Render this app"; Argo view. | 51–55 |
| 11 | CI pipelines | GitHub (matrix, reusable workflows, composite actions), GitLab (includes, `!reference`, extends, default), Azure (templates, `${{ }}`); effective config with origins; Pipeline view. | 56–60 |
| 12 | Compose and Ansible | Compose merge with origins, variables, graph, checks; Ansible execution order, role search, precedence, inventories, log task → definition; shared layered merge in `yamlkit`. | 61–64 |
| 13 | Scaffolding | 13 starters (`<% %>` templates) with forms and checks, personal starters, clone-with-rename, snippets. | 65–67 |
| 14 | Release | Settings view, embedded sample repository and first-run steps, performance work and budgets (`scripts/perf.sh`), Kustomize patch awareness, contrast and loading-state fixes, palette coverage; then this documentation site. | 68–71 |

## Results worth remembering

- Performance budgets and measurements: [architecture.md](architecture.md#performance).
- Deliberately not done: a schema-driven form for any kind (phase 13
  stretch); remote includes, remote Kustomize bases and chart downloads
  (codec never downloads).
- Checks the user still owed at the end of v2: a native macOS run, the
  phase 06 manual checklist, confirming no open 2.0 bugs, tagging
  `v2.0.0` and making the GHCR package public. They are tracked in
  [roadmap.md](roadmap.md#open-checks).
