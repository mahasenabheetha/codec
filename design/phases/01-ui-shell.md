# 01 — UI shell + v1 port

**Goal:** the new dark app shell with a reusable design system, and the
v1 tools rebuilt in it at full parity.
**Depends on:** 00 · **Branch:** `feature/mab/p01-ui-shell`
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

- [ ] Every v1 web feature works, incl. shortcuts Ctrl+Enter, Alt+C, Esc,
      transform-on-paste, swap, copy.
- [ ] Rail collapse, pane sizes and last route persist across reloads.
- [ ] Command palette reaches every tool/action.
- [ ] No hard-coded colors/sizes in components (tokens only).
- [ ] Initial load ≤ 300 KB gzipped; size reported in the PR.
- [ ] PWA still installable.

## Notes

Use Bits UI for dialog/popover/select/tooltip behaviour and style it
ourselves. Logos folder + `SOURCES.md` created here (file-type badges
arrive in 03).

**Go concepts:** none new (frontend phase). Svelte: runes, props, snippets, stores.
