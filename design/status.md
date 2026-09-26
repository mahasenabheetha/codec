# Status

**Current phase:** 06 — MVP release ([phases/06-mvp-release.md](phases/06-mvp-release.md))
**State:** done and committed on `feature/mab/yaml-tools`, except the user checks: native Windows/macOS run and the manual checklist in the phase file.
**Next step:** user raises the PR `feature/mab/yaml-tools` → `main`, merges, tags `v2.0.0-alpha.1`, makes the GHCR package public. Then phase 07.
**Blockers:** none. Open check: phase 04 typing feel on a 5k-line file (in the checklist).

Released: `v1.0.0` (2026-09-26). All v2 work stays on `feature/mab/yaml-tools` until the user raises the PR.

## Log

One line per session, newest first: date · phase · what changed.

- 2026-09-26 · 06 · Docker image (GoReleaser dockers_v2, distroless non-root, GHCR in the release workflow), snapshot + container verified, README rewritten with screenshots, CHANGELOG 2.0.0-alpha.1, releases.md Docker section; fixes: first-visit file links, not-found messages, template hover wording, container port hint.

- 2026-09-26 · 05 · `internal/helm` (Helm 4.3 SDK, byte-identical to helm template, provenance by layer lookup, reference/lookup checks), `/api/v2/helm/*`, `codec helm render|values --provenance`, Helm view (layers, --set, profiles, rendered/values/notes/problems, what-if), template-aware highlighting.
- 2026-09-26 · 04 · Provider hover/definition/completion + `Analyze`, `internal/textdiff` (Myers, git-applicable patches), `/api/v2/yaml/*` + `/files/diff`; editor: overlays, lint, hover, F12/Ctrl+click, alias completion, outline/problems panel, path breadcrumb, doc switcher, what-if edits with diff, open in VS Code/Cursor.
- 2026-09-26 · 03 · `internal/config`, `internal/workspace` (gitignore, tree, watcher, os.Root reads), secured `/api/v2` + SSE, `serve --root/--host/--poll/--open`; explorer, open-folder dialog, Ctrl+P, file tabs with live refresh, logos.
- 2026-09-26 · 02 · `internal/yamlkit` (template-aware parse, positions, friendly errors, paths, fmt/JSON/flatten/resolve), `internal/provider` (13 detectors + outline), `codec yaml` CLI; verified on 2,290 real files.
- 2026-09-26 · 01 · New dark Svelte UI at `/`: design tokens, shell (rail, tabs, palette, status bar), v1 tools ported with rich JWT/Ansible views; v1 static UI removed.
- 2026-09-26 · 00 · `/v2` module, Go 1.26, Svelte 5 + Vite 8 frontend, CI Linux+Windows, GoReleaser builds frontend.
- 2026-09-26 · planning · v2 plan, phases and agent docs written.
