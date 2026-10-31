# Design language

How codec looks and behaves, for anyone (person or agent) changing the
UI. The user-facing version is `docs/design.html`. Tokens live in
`frontend/src/lib/styles/tokens.css`; components use tokens only.

Dark by default, with a light theme [81]. Calm, dense, precise — a professional dev tool, closer to
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
| Controls | Interface size (`data-size` on `<html>`): `--control-h` 28 / 32 / 36 and `--control-h-sm` 24 / 28 / 32 for Compact / Comfortable (default) / Large; Lucide icons keep the size written in code (the Compact one) and grow by `--icon-scale` 1 / 1.125 / 1.25, rounded to whole pixels (global.css), so write icon sizes for Compact · `--tabbar-h 38` · `--statusbar-h 26` · rail 48/220 |
| Motion | `--dur 120ms`, `--ease`; none under `prefers-reduced-motion` |
| Elevation | `--shadow-pop` for popovers and dialogs only; depth otherwise comes from surface steps |
| Layers | `--z-sticky 10` `--z-popover 50` `--z-dialog 60` `--z-toast 70` |

Syntax (CodeMirror): key `#8fb4ff` · string `#a8d68a` · number
`#f2a66b` · bool/null `#d59bf6` · comment `#7a8497` italic ·
anchor/alias `#4fd1c5` · tag `#e5b449` · punctuation `--fg-2` · `---`
`--accent`. Expressions get a tinted background: template-time (Helm,
Jinja) `#f7c873` on 8%; run time (Argo, GitHub, Azure) `#ff9ecf` on 8%;
run-time-only values in rendered output: dashed underline. Diff lines:
`--diff-add-bg`, `--diff-del-bg`, `--diff-chg-bg`; changed characters inside a line `--diff-add-strong`, `--diff-del-strong`. File-type logos use
`--brand-*` (lightened for contrast).

Light theme (`:root[data-theme='light']` in `tokens.css`) redefines the
colours only; space, type, layout and motion are shared:

| Group | Light values |
|---|---|
| Surfaces | `--bg-0 #f2f3f6` · `--bg-1 #f8f9fb` · `--bg-2 #ffffff` · `--bg-3 #eaecf1` · `--bg-4 #dfe2e9` |
| Borders | `--border #d9dce3` · `--border-strong #c3c8d2` |
| Text | `--fg-0 #1b1f27` · `--fg-1 #485062` · `--fg-2 #5f6778` |
| Accent | `--accent #2b5cd6` (hover darker) · `--accent-fg #ffffff` |
| Status | `--ok #16702f` · `--warn #8a5d00` · `--err #c62a35` · `--info #0a68cf`; `-soft` at 12% |
| Syntax | key `#0550ae` · string `#1f6f2b` · number `#a64d00` · bool/null `#7c3fd1` · comment `#626b7a` · anchor `#0b7268` · tag `#8a5d00` · template `#875400` · run time `#b0246f` |
| Logos | official colours where they reach 3:1 on `--bg-1`; Helm `#3346c9` and Argo `#d9592b` adjusted |

Dusk (`:root[data-theme='dusk']`, decision 94) is the soft, colourful
dark theme, made in OKLCH so its colours share one lightness:

| Group | Dusk values |
|---|---|
| Surfaces | `--bg-0 #1c1d2a` · `--bg-1 #202130` · `--bg-2 #252636` · `--bg-3 #2d2f41` · `--bg-4 #36384e` (blue-violet, hue 280) |
| Borders | `--border #323446` · `--border-strong #43465d` |
| Text | `--fg-0 #d5d6e4` (10.3:1 on bg-2, Dark is 16:1) · `--fg-1 #adb0c3` · `--fg-2 #9da0b6` |
| Accent | `--accent #acb1f0` periwinkle · `--accent-fg` is `--bg-0` |
| Status | `--ok #7cc98d` · `--warn #d4af5d` · `--err #f59693` · `--info #72bcf9` (L 0.77, more chroma than the hues) |
| Hues | indigo `#a8bcf5` · violet `#c6b2ed` · pink `#e1a9d4` · peach `#ebae8d` · lime `#b1c88b` · teal `#7ad0c5` · cyan `#7ccbe4` (L 0.80, C 0.085); `--tint-tool` and `--tint-action` 100% |
| Syntax | key `#a5bdf2` · string `#aed09c` · number `#e7b18c` · bool `#caade1` · comment `#9497ab` · anchor `#89cdc3` · tag `#dcc188` |

Dusk's text stays between 7:1 and 12:1 on its surfaces: readable, never
glaring. `npm --prefix frontend run contrast` (`frontend/scripts/contrast.mjs`)
checks every rule here for all three themes.

All themes meet the same rules: text ≥ 4.5:1 on `--bg-0`…`--bg-3` and
on its own `-soft` background, logos ≥ 3:1. A new colour token gets a
value in each theme block (Dusk may inherit Dark's).

**Colour roles** (decision 94). Colour has three jobs, kept apart:

- **Tool hue.** Each tool and workspace view has one (`hue` in
  `lib/tools.ts`, `viewHue` for the rail's views), from `--hue-indigo`,
  `-violet`, `-pink`, `-peach`, `-lime`, `-teal`, `-cyan`. What belongs to
  the tool (its page, rail entry, Home card) sets `--tool` to
  `hueVar(hue)` and takes the `.hued` class, which works out
  `--tool-accent`, `--tool-accent-hover`, `--tool-accent-soft` and
  `--tool-icon` for everything inside. Shared components read
  `var(--tool-accent, var(--accent))`, so they work outside a tool too.
- **Action tone.** `tone` on `IconButton` and `Button`: `copy`, `clear`,
  `save`, `swap`, `open`, `add`, `run` (`--act-*`). Only the icon takes
  it, never on a primary button or a disabled one. A close or remove ×
  has no tone.
- **Status.** `--ok`, `--warn`, `--err`, `--info` keep hues no tool or
  action uses, so colour never suggests a problem that isn't there.

How much of the first two a theme shows is `--tint-tool` and
`--tint-action`: `0%` in Dark and Light, which keeps them exactly as
they were. Never use a `--hue-*` or `--act-*` token directly; go through
the roles, so a theme can turn them up or down in one place.

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
  Utilities · Settings. Explorer and lens panel resize and collapse; sizes persist.
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
| `VirtualList` | Long lists of rows of any height (Problems, Resources cards, Log viewer): only rows in view are drawn, each measured once shown; `scrollToIndex`, `onrange` for page-by-page loading |
| `CodeView` | Any code or YAML (CodeMirror 6), read-only or editable |
| `Graph` | Node graphs (dagre layout, pan/zoom, colour groups `g-<group>`, dashed edges for run-time links, run status `s-failed/changed/ok/rescued/never`) |
| `EffectiveLines` | Config with an origin per line (CI, Compose) |
| `Popover`, `Dialog`, `Tooltip` | Overlays; dialogs trap focus and close on Esc |
| `ContextMenu` | Right-click menus (also Shift+F10); entries built per open from the event; sentence case, an icon per entry |
| `Badge`, `Kbd` | File type with logo; key combos |
| `EmptyState` | Nothing to show yet: icon, one sentence, the action that fills it |
| `Skeleton` | Loading placeholders in the content's shape |
| `Toaster` / `toast()` | Brief confirmations and non-blocking errors; an optional action ("Cleared · Undo") for undoable clears |

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
- **Tools.** One page: title and toolbar, input left, output right
  (`ToolLayout`, `InputPane`, `Pane`) when the input can be a document.
  When it is a value or a small form (a timestamp, a cron expression,
  generator options), one column instead (`ToolLayout` `body`,
  `LineInput`, `Pane grow={false}`): the input on top, answers below,
  each as tall as its content. A tool with sub-tabs lists them in
  `lib/tools.ts` (`tabs`), shows them as a `SegmentedControl` first in its
  toolbar (`ToolTabs`) and renders through `TabbedTool`, which mounts a
  sub-tab when first shown and keeps it; the last sub-tab is remembered,
  and Home and the palette list each one by name. Requests go through
  `Job` / `TextJob` (newest answer wins, debounced while typing). One
  tool hands input to another with `handoff.send(tool, tab, input)`.
  Results with several values give each its own copy button.
- **Persistence.** `persisted()` is for display preferences only. Never
  persist tool input, what-if edits or variables [19].
- **API.** Components call the backend only through `lib/api/*`.
  Requests that follow typing are debounced and abortable.
- **Desktop.** The same UI runs in the desktop window. Anything only it
  can do (system folder dialog, window frame, Settings → Desktop) goes
  through `lib/platform.ts`, which checks `desktop` and leaves the
  browser with a working fallback. Never branch on the platform
  elsewhere.
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
