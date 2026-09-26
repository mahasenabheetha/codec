# Status

**Current phase:** 10 — Argo lens ([phases/10-argo.md](phases/10-argo.md))
**State:** not started. Phases 00–09 are done and committed on `feature/mab/yaml-tools`; 06 still awaits the user's native checks and the `v2.0.0-alpha.1` tag.
**Next step:** start phase 10 on `feature/mab/yaml-tools`.
**Blockers:** none. Open checks: phase 06 manual checklist (native Windows/macOS, typing on a 5k-line file).

Released: `v1.0.0` (2026-09-26). All v2 work stays on `feature/mab/yaml-tools` until the user raises the PR.

## Log

One line per session, newest first: date · phase · what changed.

- 2026-09-26 · 09 · `internal/kube` (objects, label selectors, relationship graph + soft broken-ref findings, inventory, cards, neat, Secret decode, in-process Kustomize = kubectl output), kube provider (meaningful outline, Secret hover), `/api/v2/k8s/*`, `codec k8s images|refs|neat`, `codec kustomize build`; UI: shared Graph (dagre), Resources view (objects/graph/inventory/references) for folders, Helm renders and Kustomize builds.

- 2026-09-26 · 08 · `yamlkit.Diff` (semantic: key/list order ignored, docs by identity, list items by name, similar leftovers paired), `internal/query` (gojq, path() for positions, no env), `/api/v2/compare|query`, `codec yaml diff` (exit 2) and `codec yaml query`; Compare view (files, what-if vs disk, Helm profiles, single docs, ignore paths) and Query view (workspace/file/Helm, jump to line).

- 2026-09-26 · 07 · `internal/lint` (8 style rules, 7 Kubernetes checks, deprecation table), `internal/schema` (schema per doc, validation mapped to lines, completion/hover), `internal/schemacache` (on-demand fetch, cache, offline, custom folder), `internal/check`; `/api/v2/lint/*`, `codec yaml lint` (exit 2); editor schema badge, schema completion, Why/Turn off on problems, Workspace problems and Lint settings views, lint on Helm output.

- 2026-09-26 · 06 · Docker image (GoReleaser dockers_v2, distroless non-root, GHCR in the release workflow), snapshot + container verified, README rewritten with screenshots, CHANGELOG 2.0.0-alpha.1, releases.md Docker section; fixes: first-visit file links, not-found messages, template hover wording, container port hint.

- 2026-09-26 · 05 · `internal/helm` (Helm 4.3 SDK, byte-identical to helm template, provenance by layer lookup, reference/lookup checks), `/api/v2/helm/*`, `codec helm render|values --provenance`, Helm view (layers, --set, profiles, rendered/values/notes/problems, what-if), template-aware highlighting.
- 2026-09-26 · 04 · Provider hover/definition/completion + `Analyze`, `internal/textdiff` (Myers, git-applicable patches), `/api/v2/yaml/*` + `/files/diff`; editor: overlays, lint, hover, F12/Ctrl+click, alias completion, outline/problems panel, path breadcrumb, doc switcher, what-if edits with diff, open in VS Code/Cursor.
- 2026-09-26 · 03 · `internal/config`, `internal/workspace` (gitignore, tree, watcher, os.Root reads), secured `/api/v2` + SSE, `serve --root/--host/--poll/--open`; explorer, open-folder dialog, Ctrl+P, file tabs with live refresh, logos.
- 2026-09-26 · 02 · `internal/yamlkit` (template-aware parse, positions, friendly errors, paths, fmt/JSON/flatten/resolve), `internal/provider` (13 detectors + outline), `codec yaml` CLI; verified on 2,290 real files.
- 2026-09-26 · 01 · New dark Svelte UI at `/`: design tokens, shell (rail, tabs, palette, status bar), v1 tools ported with rich JWT/Ansible views; v1 static UI removed.
- 2026-09-26 · 00 · `/v2` module, Go 1.26, Svelte 5 + Vite 8 frontend, CI Linux+Windows, GoReleaser builds frontend.
- 2026-09-26 · planning · v2 plan, phases and agent docs written.
