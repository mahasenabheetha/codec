# 01 — UI shell + v1 port

**Goal:** the new dark app shell with a reusable design system, and the
v1 tools rebuilt in it at full parity.
**Depends on:** 00 · **Branch:** `feature/mab/yaml-tools`
**Read:** ui.md (all), conventions.md (Frontend)

## Scope

- `lib/styles/tokens.css` + global styles (fonts bundled, scrollbars,
  selection, focus ring) exactly per ui.md.
- Components from ui.md needed now: Button, IconButton, Tabs,
  SplitPane, Tooltip, Popover, Dialog, Toast, Kbd, Badge, EmptyState,
  Select, Toggle, SearchInput, StatusBar, CommandPalette, and a basic
  CodeView (CodeMirror 6 with JSON/YAML highlight, dark theme).
- App layout: collapsible rail (`Ctrl+B`, persisted), tab bar, main
  area, status bar. Hash routing (`#/tools/base64`, …). Lazy-load routes.
- `lib/api/` client (all backend calls go through it; token header
  support ready for phase 03). Shortcut registry + command palette.
- Port v1 tools, same `/api/transform` API:
  Smart paste (auto) · Base64 (encode/decode, URL-safe) · JSON
  (pretty/min/validate, click error → jump) · JWT (header/payload
  cards, `exp`/`iat` shown as dates, "signature not verified" note) ·
  Ansible log (status, cause banner, chips, severity-colored output,
  errors-only filter).
- Swap: new app at `/`, delete `internal/web/static/`, keep PWA manifest
  and icons (update theme color to `--bg-0`).

## Out of scope

Workspace, YAML features, backend changes beyond serving the new app.

## Acceptance

- [x] Every v1 web feature works, incl. shortcuts Ctrl+Enter, Alt+C, Esc,
      transform-on-paste, swap, copy.
- [x] Rail collapse, pane sizes, open tabs and last route persist across reloads.
- [x] Command palette reaches every tool/action.
- [x] No hard-coded colors in components; spacing, radii and type via tokens
      (fixed element dimensions such as row heights stay plain px).
- [x] Initial load ≤ 300 KB gzipped: ~75 KB JS+CSS + ~88 KB fonts; the
      editor chunk (~127 KB) loads on first tool open.
- [x] PWA manifest + icons served (install itself: verify in Edge/Chrome).

## Notes

- Bits UI provides dialog/popover/select/tooltip/command behaviour; we style it.
- Tools stay mounted while their tab is open (state kept); closing a tab resets it.
- Escape in an editor with a selection first collapses the selection
  (CodeMirror), a second Escape clears the tool.
- Use `setTimeout`, not `requestAnimationFrame`, for post-open focus and
  deferred actions: rAF never fires while the window isn't painting.
- API gained optional `indent` (request) and `jwt` (response); `/app/`
  redirects to `/`. Logos folder + `SOURCES.md` exist; badges come in 03.

**Go concepts:** wire types separate from engine types (JSON tags at the HTTP edge), `http.RedirectHandler`. Svelte: runes, snippets, `$state` in classes, `$effect` cleanup.
