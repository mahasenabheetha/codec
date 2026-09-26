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
    used (2026-09-26).
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
    whole-line control flow becomes spaces, inline expressions become
    `x…x`; values keep the original expression text and are marked
    `Templated`. Duplicate-key checks are skipped in documents with
    template control flow (branches repeat keys legitimately) (2026-09-26).
