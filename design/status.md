# Status

**Current phase:** 06 — MVP release ([phases/06-mvp-release.md](phases/06-mvp-release.md))
**State:** not started. Phases 00–05 are done and committed on `feature/mab/yaml-tools`.
**Next step:** start phase 06 on `feature/mab/yaml-tools`.
**Blockers:** none. Open checks: phase 03's Docker run (phase 06 acceptance); phase 04 typing feel on a 5k-line file (user).

Released: `v1.0.0` (2026-09-26). All v2 work stays on `feature/mab/yaml-tools` until the user raises the PR.

## Log

One line per session, newest first: date · phase · what changed.

- 2026-09-26 · 05 · `internal/helm` (Helm 4.3 SDK, byte-identical to helm template, provenance by layer lookup, reference/lookup checks), `/api/v2/helm/*`, `codec helm render|values --provenance`, Helm view (layers, --set, profiles, rendered/values/notes/problems, what-if), template-aware highlighting.
- 2026-09-26 · 04 · Provider hover/definition/completion + `Analyze`, `internal/textdiff` (Myers, git-applicable patches), `/api/v2/yaml/*` + `/files/diff`; editor: overlays, lint, hover, F12/Ctrl+click, alias completion, outline/problems panel, path breadcrumb, doc switcher, what-if edits with diff, open in VS Code/Cursor.
- 2026-09-26 · 03 · `internal/config`, `internal/workspace` (gitignore, tree, watcher, os.Root reads), secured `/api/v2` + SSE, `serve --root/--host/--poll/--open`; explorer, open-folder dialog, Ctrl+P, file tabs with live refresh, logos.
- 2026-09-26 · 02 · `internal/yamlkit` (template-aware parse, positions, friendly errors, paths, fmt/JSON/flatten/resolve), `internal/provider` (13 detectors + outline), `codec yaml` CLI; verified on 2,290 real files.
- 2026-09-26 · 01 · New dark Svelte UI at `/`: design tokens, shell (rail, tabs, palette, status bar), v1 tools ported with rich JWT/Ansible views; v1 static UI removed.
- 2026-09-26 · 00 · `/v2` module, Go 1.26, Svelte 5 + Vite 8 frontend, CI Linux+Windows, GoReleaser builds frontend.
- 2026-09-26 · planning · v2 plan, phases and agent docs written.
