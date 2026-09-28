# Design language

How codec looks and behaves, for anyone (person or agent) changing the
UI. The user-facing version is `docs/design.html`. Tokens live in
`frontend/src/lib/styles/tokens.css`; components use tokens only.

Dark only [8]. Calm, dense, precise — a professional dev tool, closer to
Linear or VS Code than to a marketing site.

## Principles

| Principle | Rule |
|---|---|
| Instant | Local operations answer in < 100 ms. Loading keeps the layout and shows `Skeleton`, never a spinner. |
| Keyboard-first | Every action has a shortcut or a palette entry (`commands.register`). Focus is always visible. |
| Never lose state | Tabs keep state while open; what-if edits last until reset; layout persists. |
| Explain, don't just flag | Every problem says why, how to fix, and links to the spot. |
| Copy everywhere it helps | Paths, values, rendered docs, diffs, effective configs. |
| Honest | Run-time or remote values are shown as unknown, never guessed. |

## Tokens

| Group | Tokens |
|---|---|
| Surfaces | `--bg-0 #0b0d12` app/rail · `--bg-1 #10131a` panels · `--bg-2 #161a23` editor/cards · `--bg-3 #1d2230` hover/inputs · `--bg-4 #252b3a` pressed |
| Borders | `--border #252b38` · `--border-strong #323a4b` |
| Text | `--fg-0 #e6e8ee` primary · `--fg-1 #a9b0bf` secondary · `--fg-2 #878fa0` muted (≥ 4.5:1 on bg-0…bg-3) |
| Accent | `--accent #7aa2ff` · `--accent-hover` · `--accent-soft` (14%) · `--accent-fg` text on solid accent |
| Status | `--ok #3fb971` · `--warn #e5b449` · `--err #f06b75` · `--info #5ab0ff`, each with `-soft` |
| Space | 4px scale: `--s-1 4` `--s-2 8` `--s-3 12` `--s-4 16` `--s-5 20` `--s-6 24` `--s-8 32` `--s-12 48` |
| Radius | `--r-sm 4` inputs · `--r-md 6` buttons · `--r-lg 10` panels/dialogs · `--r-full` pills |
| Type | `--font-ui` Inter, `--font-mono` JetBrains Mono (bundled, OFL). Sizes `--fs-xs 11` `-sm 12` `-md 13` (base) `-lg 14` `-xl 16` `-2xl 20` `-3xl 28`; line height 1.5 (code 1.55) |
| Controls | `--control-h 28` · `--control-h-sm 24` · `--tabbar-h 38` · `--statusbar-h 26` · rail 48/220 |
| Motion | `--dur 120ms`, `--ease`; none under `prefers-reduced-motion` |
| Elevation | `--shadow-pop` for popovers and dialogs only; depth otherwise comes from surface steps |
| Layers | `--z-sticky 10` `--z-popover 50` `--z-dialog 60` `--z-toast 70` |

Syntax (CodeMirror): key `#8fb4ff` · string `#a8d68a` · number
`#f2a66b` · bool/null `#d59bf6` · comment `#7a8497` italic ·
anchor/alias `#4fd1c5` · tag `#e5b449` · punctuation `--fg-2` · `---`
`--accent`. Expressions get a tinted background: template-time (Helm,
Jinja) `#f7c873` on 8%; run time (Argo, GitHub, Azure) `#ff9ecf` on 8%;
run-time-only values in rendered output: dashed underline. Diff lines:
`--diff-add-bg`, `--diff-del-bg`, `--diff-chg-bg`. File-type logos use
`--brand-*` (lightened for contrast).

Never hard-code a colour, size or font in a component. A new token is a
change to this file and `tokens.css` together.

## Layout

```
┌rail┬ tabs ─────────────────────────────── ⌘K ┐
│    ├ files ┬ main ───────────┬ lens panel ────┤
│    │ tree  │ editor, lens    │ Outline        │
│    │       │ view or tool    │ Problems       │
├────┴───────┴─────────────────┴────────────────┤
│ status: type · path · counts · folder · ver   │
└───────────────────────────────────────────────┘
```

- Rail 48/220 px (`Ctrl+B`); groups Workspace · Encode & decode · Logs ·
  Settings. Explorer and lens panel resize and collapse; sizes persist.
- Every view is a tab keyed by its route (`lib/stores/router.svelte.ts`);
  views stay mounted while their tab is open (`active` prop hides them).
- Lens views (Argo, Pipeline, Compose, Ansible) use `LensView` (header,
  what-if overrides, reload counter) and `LensGrid` (side · main ·
  detail, stacked when narrow). New lens views follow the same frame.

## Components

Build once in `frontend/src/lib/components` and reuse:

| Component | Use for |
|---|---|
| `Button` | Actions with text (primary, secondary, ghost; `sm`) |
| `IconButton` | Icon-only actions; always a `label` (also its tooltip) |
| `Tabs`, `SegmentedControl` | Switching views of the same thing; exclusive choices |
| `Select`, `Toggle`, `SearchInput` | Form controls (Bits UI underneath) |
| `SplitPane` | Resizable panes (sizes persisted by key) |
| `TreeView` | File trees and outlines; keyboard pattern, windowed over 300 rows |
| `CodeView` | Any code or YAML (CodeMirror 6), read-only or editable |
| `Graph` | Node graphs (dagre layout, pan/zoom, colour groups `g-<group>`) |
| `EffectiveLines` | Config with an origin per line (CI, Compose) |
| `Popover`, `Dialog`, `Tooltip` | Overlays; dialogs trap focus and close on Esc |
| `Badge`, `Kbd` | File type with logo; key combos |
| `EmptyState` | Nothing to show yet: icon, one sentence, the action that fills it |
| `Skeleton` | Loading placeholders in the content's shape |
| `Toaster` / `toast()` | Brief confirmations and non-blocking errors |

Shell pieces live in `lib/shell` (Rail, TabBar, StatusBar,
CommandPalette, ShortcutsDialog). New tools register in `lib/tools.ts`
(rail, home, tabs and palette are generated from it).

Icons: `@lucide/svelte`, imported per icon
(`@lucide/svelte/icons/<name>`), 16 px, stroke 1.75, `currentColor`.
Logos: local SVGs in `frontend/src/assets/logos/`, source and licence
per file in its `SOURCES.md`; only to label file types.
App icon: `frontend/public/icon.svg` (also `docs/assets/logo.svg`), a YAML
key over nested key: value rows in `--accent` to `--syn-anchor`. The PNGs
beside it (192, 512, maskable 512 with the glyph in the 80% safe zone,
180 touch icon) and `favicon.ico` (16 + 32) are renders of it; re-render
them when it changes, and rename the SVG if browser tabs keep the old one.

## Patterns

- **States.** Empty → `EmptyState`. First load → `Skeleton` inside a
  container with `aria-busy="true"` and an `aria-label`. Refresh of
  content already shown → small muted text in the header ("Reading…").
  Error → plain sentence, the reason, a way forward; never a raw stack.
- **Commands and shortcuts.** App-wide commands in `App.svelte`; a view's
  own commands registered while it is active
  (`$effect(() => active && commands.register([...]))`, or
  `toolCommands` for tools). Shortcuts via `shortcut()`; add them to
  `ShortcutsDialog` and `docs/shortcuts.html`.
- **Persistence.** `persisted()` is for display preferences only. Never
  persist tool input, what-if edits or variables [19].
- **API.** Components call the backend only through `lib/api/*`.
  Requests that follow typing are debounced and abortable.
- **Feedback.** Copy actions confirm with a toast; destructive actions
  don't exist (read-only), "forget" actions confirm with a toast.

## Accessibility checklist

- Text contrast ≥ 4.5:1 on its surface; don't use `--fg-2` on `--bg-4`.
- Keyboard focus visible (global 2 px accent ring); if a component
  removes the outline, it shows an equivalent (border colour plus ring).
- Every interactive element has an accessible name (text, `aria-label`
  or `label`). Icon-only buttons use `IconButton`.
- Trees, lists, tabs, radio groups follow WAI-ARIA keyboard patterns.
- Severity is never colour alone: pair it with an icon or a word.
- Respect `prefers-reduced-motion`.

## Writing

- Plain, short sentences; sentence case for titles, buttons and menus.
- Say what happened and what to do next; name the file and line.
- Use each tool's own terms (`values.yaml`, `needs`, `templateRef`).
- No exclamation marks, no blame, no "please", no jargon a common word
  can replace. Numbers as digits.

## Review checklist for UI changes

1. Tokens only; shared components reused or extended, not copied.
2. Empty, loading and error states present.
3. Reachable by keyboard; has a palette entry if it is an action.
4. Accessible names on controls; contrast checked for new colour pairs.
5. Works at narrow widths (the app is used side by side with an editor).
6. `npm --prefix frontend run check` passes; screenshots in `docs/`
   updated if the view is shown there.
