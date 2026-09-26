# UI design language

Dark only. Calm, dense, precise — a professional dev tool, closer to
Linear/VS Code than to a marketing site. Tokens live in
`frontend/src/lib/styles/tokens.css`; components use tokens only.

## Tokens

| Group | Tokens |
|---|---|
| Surfaces | `--bg-0 #0b0d12` app/rail · `--bg-1 #10131a` panels · `--bg-2 #161a23` editor/cards · `--bg-3 #1d2230` hover/inputs |
| Borders | `--border #252b38` · `--border-strong #323a4b` |
| Text | `--fg-0 #e6e8ee` primary · `--fg-1 #a9b0bf` secondary · `--fg-2 #6e7686` muted (large text/icons only) |
| Accent | `--accent #7aa2ff` · `--accent-soft rgb(122 162 255 / .14)` |
| Status | `--ok #3fb971` · `--warn #e5b449` · `--err #f06b75` · `--info #5ab0ff` |
| Space | 4px scale: `--s-1 4` `--s-2 8` `--s-3 12` `--s-4 16` `--s-6 24` `--s-8 32` |
| Radius | `--r-sm 4` inputs · `--r-md 6` buttons · `--r-lg 10` panels/dialogs |
| Type | UI: Inter 13px/1.5 (12 small, 14/16/20 headings). Code: JetBrains Mono 13px/1.55. Both bundled as woff2 (OFL). |
| Motion | 120ms ease-out; none under `prefers-reduced-motion` |

Depth comes from surface steps, not shadows (one soft shadow for
popovers/dialogs only). Focus ring: 2px `--accent`, always visible on
keyboard focus. Text contrast ≥ 4.5:1 on its surface.

## Syntax colors (CodeMirror theme)

key `#8fb4ff` · string `#a8d68a` · number `#f2a66b` · bool/null
`#d59bf6` · comment `#5c6576` italic · anchor/alias `#4fd1c5` · tag
`#e5b449` · punctuation `--fg-2` · `---` separator `--accent`.
Expressions get a tinted background so they pop inside YAML:
- template-time (Helm/Go template, Jinja): `#f7c873` on `rgb(247 200 115 / .08)`
- runtime (Argo `{{…}}`, GitHub `${{…}}`): `#ff9ecf` on `rgb(255 158 207 / .08)`
- unresolvable/runtime-only values in rendered output: dashed underline.

## Layout

```
┌rail┬ tabs ─────────────────────────────── ⌘K ┐
│    ├ files ┬ editor / main ──┬ lens panel ────┤
│    │ tree  │                 │ Outline·Render │
│    │       │                 │ Values·Problems│
├────┴───────┴─────────────────┴────────────────┤
│ status: type logo · path · ✓/⚠ counts · ver   │
└───────────────────────────────────────────────┘
```

- Rail: 48px icons / 220px labels, toggle `Ctrl+B`, state remembered.
  Groups: Workspace · YAML tools · Encode/Decode · Logs · Settings.
- All panes resizable and collapsible; sizes persist (localStorage).
- Command palette `Ctrl+K` / `⌘K` for every action and file.

## Components (build once in `lib/components`)

Button (primary/ghost/icon), IconButton, Tabs, SplitPane, TreeView,
Badge (file type + logo), Tooltip, Popover, Dialog, Toast, Kbd,
EmptyState (with "try a sample"), CodeView (CodeMirror wrapper),
StatusBar, CommandPalette, Select, Toggle, SearchInput.

## Icons and logos

- UI icons: `@lucide/svelte` (per-icon imports), 16px, stroke 1.75, `currentColor`.
- Brand logos (Kubernetes, Helm, Argo, GitHub Actions, GitLab, Azure
  Pipelines, Ansible, Docker/Compose, YAML): local SVGs in
  `frontend/src/assets/logos/`, used only to label file types. Record
  source + license per file in `SOURCES.md` (CNCF artwork repo, Simple
  Icons, vendors' brand pages).

## UX principles

- Instant: local operations respond < 100ms; show skeletons, not spinners.
- Keyboard-first; every action has a shortcut or palette entry.
- Never lose state: what-if edits persist per tab until reset.
- Explain, don't just flag: every error says why and links to the spot.
- Copy everywhere it helps (paths, values, rendered docs, diffs).
