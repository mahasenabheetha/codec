# Design language

How codec looks and behaves, for anyone (person or agent) changing the
UI. The user-facing version is `docs/design.html`. Tokens live in
`frontend/src/lib/styles/tokens.css`; components use tokens only.

Dark by default, ten themes in all [81, 94, 95]. Calm, dense, precise —
a professional dev tool, closer to Linear or VS Code than to a
marketing site. Refreshed in 3.10.1 and 3.10.2 (decision 105): colour
in every theme, shared parts in place of copies, panes you can size,
and a shell built around what you use most.

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
| Radius | `--r-sm 4` inputs, small marks · `--r-md 8` buttons, rows · `--r-lg 12` panels, cards · `--r-xl 16` dialogs · `--r-full` pills |
| Type | `--font-ui` Inter, `--font-mono` JetBrains Mono (bundled, OFL). Sizes `--fs-xs 11` `-sm 12` `-md 13` (base) `-lg 14` `-xl 16` `-2xl 20` `-3xl 28`, each a step bigger at Interface size Large (12 … 30); line height 1.5 (code 1.55). Figures are tabular everywhere (`tnum`), so counts and times don't shift. The code font is for code, paths and values, never for sentences |
| Controls | Interface size (`data-size` on `<html>`): `--control-h` 28 / 32 / 36 and `--control-h-sm` 24 / 28 / 32 for Compact / Comfortable (default) / Large; Lucide icons keep the size written in code (the Compact one) and grow by `--icon-scale` 1 / 1.125 / 1.25, rounded to whole pixels (global.css), so write icon sizes for Compact · `--tabbar-h 38` · `--statusbar-h 26` · rail 48/220 |
| Focus | `--focus-ring`: the ring a field shows around its border (in the tool's colour inside a tool). The global outline stays for everything else |
| Match | `--match-bg`, `--match-current-bg`: a search hit, wherever it is drawn (editor, log rows, Find in Files, the source peek) |
| Motion | `--dur-fast 80ms` hovers and presses · `--dur 120ms` most changes · `--dur-slow 240ms` things that move across the screen · `--ease`; none under `prefers-reduced-motion` |
| Elevation | `--shadow-pop` for popovers and dialogs; `--shadow-raise`, barely there, for a card that lifts under the pointer; depth otherwise comes from surface steps |
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

Midnight (decision 95) is Dusk on deep navy: it shares Dusk's block and
overrides only the surfaces (`--bg-0 #0a1628` · `--bg-1 #0d1b2e` ·
`--bg-2 #111f35` · `--bg-3 #192941` · `--bg-4 #21334e`, hue 258 with more
chroma), the borders (`#1e2e46`, `#2d405d`), `--fg-0 #cbcfdc` (a touch
dimmer, to stay under 12:1 on the darker ground), `--accent-fg`, the
shadow and the overlay.

Forest, Rosé and Ocean (3.10.0) are Dusk on other grounds in the same
way: green-grey (`--bg-2 #1c2520`, accent a misty blue `#a6c2e6` so it
never reads as ok), pink-violet (`--bg-2 #2b2330`, accent rose `#e8a8d0`)
and deep blue (`--bg-2 #122532`, accent aqua `#76cfd6`, rest a soft green
`#9fd3a6` so a break doesn't look like the accent). Paper is a warm,
low-glare light theme: cream surfaces (`--bg-2 #f9f5ee`), text at most
14:1, and its own dark hues (`#4c5cb4` … `#1f6a8a`) with the tints at
100%, so tools show their colours on it. Mist is Light dimmed: grey
surfaces well below white (`--bg-2 #c2c5cc`), near-black text at most 14:1, deep
hues (`#36438f` … `#15516b`) and the tints at 100%; it shares Light's
block and overrides its colours. High contrast is black, white and
bright colours, every text, accent, status and syntax colour at least
7:1 (WCAG AAA), with firm borders; its tool hues are bright too
(`#aab6ff` … `#92ddff`, at least 7:1 on black, black text on them) with
the tints at 100%, so tools keep their colours.

Dusk's, Midnight's, Forest's, Rosé's and Ocean's text stays between 7:1
and 12:1 on their surfaces: readable, never glaring.
`npm --prefix frontend run contrast` (`frontend/scripts/contrast.mjs`)
checks every rule here for all ten themes.

All themes meet the same rules: text ≥ 4.5:1 on `--bg-0`…`--bg-3` and
on its own `-soft` background, logos ≥ 3:1. A new colour token gets a
value in each theme block (Dusk may inherit Dark's).

**Colour roles** (decision 94). Colour has three jobs, kept apart:

- **Tool hue.** Each tool and workspace view has one (`hue` in
  `lib/tools.ts`, `viewHue` for the rail's views), from `--hue-indigo`,
  `-violet`, `-orchid`, `-pink`, `-peach`, `-lime`, `-teal`, `-cyan`
  (eight; no two tools in one group of the rail share one, and none
  sits on a status colour: a blue or a green was left out for that). What belongs to
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
`--tint-action`, and `--tint-fill` for fills drawn in the tool's colour
(the selected sub-tab as a soft pill, the selected rail entry's glow, a
line along the top of a `Pane`). Since 3.10.1 every theme shows them:
tool and action `100%` everywhere, fills `60%` in Dark and Light (the
quieter two) and `100%` in the rest; Light has its own hues, dark
enough for white. Settings → Tool colours off sets
`data-colour='plain'` on `<html>`, which zeroes all three in any theme
(Dark and Light as they were before 3.10.1). Workspace views (Compare,
Query, New, Problems) are `.hued` too, with their `viewHue`. Never use a `--hue-*` or `--act-*` token directly; go through
the roles, so a theme or the setting can turn them up or down in one
place. A shared component reads `var(--tool-accent, var(--accent))`.

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

- **Rail** 48/220 px (`Ctrl+B`): Home, then Pinned (the tools and
  sub-tabs the user pinned, `lib/stores/usage.svelte.ts`), Workspace,
  and the tool groups; Settings, Keyboard shortcuts and Collapse are
  three icons in a row at the bottom. The list scrolls, and fades where
  it goes on. A tool's menu (right-click, Shift+F10) pins it.
- **Tabs.** Every view is a tab keyed by its route
  (`lib/stores/router.svelte.ts`); views stay mounted while their tab
  is open (`active` prop hides them). Tabs that don't fit scroll (the
  wheel too), the current one stays in view and a count at the end
  lists them all; a tab drags to a new place and has a menu (close,
  the others, to the right, all). `Alt+]`, `Alt+[`, `Alt+Shift+W`.
- **Home** is where a tool is found: a search box, pinned tools, the
  ones opened lately, the workspace, then every tool by group; a star
  on a card pins it.
- **Panes resize.** Anything with two or three parts is a `SplitPane`
  or `ThreePanes`, never a fixed grid: sizes persist per `id`, a side
  panel keeps its width in px when the window changes (`px`), and the
  parts stack when narrow. Explorer and lens panel resize and collapse.
- **Lens views** (Argo, Pipeline, Compose, Ansible) use `LensView`
  (`PanelHeader`, what-if overrides, reload counter) and `LensGrid` or
  `ThreePanes` (side · main · detail, one remembered size for all of
  them). New lens views follow the same frame.
- **A view's header** is `PanelHeader`: icon, name, what is known
  (counts, a **What-if · N** badge with a reset beside it in the lens
  views), actions on the right.
- **A view that is drawn again and fails** keeps what it showed and
  says so (`ErrorNote` at the top of its side panel, with Try again);
  a first load that fails is an `EmptyState` with Try again.

## Components

Build once in `frontend/src/lib/components` and reuse:

| Component | Use for |
|---|---|
| `Button` | Actions with text (primary, secondary, ghost; `sm`) |
| `IconButton` | Icon-only actions; always a `label` (also its tooltip) |
| `Tabs`, `SegmentedControl` | Switching views of the same thing; exclusive choices |
| `WorkTabStrip` + `WorkTabs` (features/shared) | Several of the same work open at once in one tool (pastes, comparisons, logs; decision 97): one store and strip, each tab's component kept mounted and hidden when not current |
| `Select`, `Toggle`, `SearchInput` | Form controls (Bits UI underneath) |
| a plain `<input>`, `<textarea>`, `<select>` | Text fields and native menus: global.css gives every one the same frame and focus (border colour plus `--focus-ring`), with no weight, so a view writes only what differs. A field inside its own framed box sets `box-shadow: none` and the box shows the focus (`:focus-within`) |
| `Chip` | A small choice in a row: a suggestion (a preset; `selected` marks the one in use) or a toggle (`pressed`: a flag, a weekday), `square` for one letter |
| `SplitPane` | Two resizable panes: drag, arrow keys (Shift, Home, End), double click or Enter to reset; `direction`, `stackBelow`, `flush` (a line, for panes that touch), `px` (a side panel's width); sizes persisted by `id`, side by side and stacked apart |
| `ThreePanes` | Side · main · detail, each resizable: the frame of the lens views |
| `PanelHeader` | The strip at the top of a view: icon (or `lead`, a file-type logo), title, `meta`, `actions`. Every view but the file view uses it; the file view's breadcrumb strip has the same height |
| `Menu` | A button that opens a menu of named actions: the "…" at the end of a toolbar, for what doesn't earn a button. One main action per view as a button, the rest here |
| `foldSections` (lib/actions) | On the side panel of a lens view: each `section > h3` folds (remembered per view), and Problems comes first when it has any |
| `TreeView` | File trees and outlines; keyboard pattern, windowed over 300 rows |
| `VirtualList` | Long lists of rows of any height (Problems, Resources cards, Log viewer): only rows in view are drawn, each measured once shown; `scrollToIndex`, `onrange` for page-by-page loading |
| `CodeView` | Any code or YAML (CodeMirror 6), read-only or editable |
| `Graph` | Node graphs (dagre layout, colour groups `g-<group>`, dashed edges for run-time links, run status `s-failed/changed/ok/rescued/never` as a glyph as well as a colour). `selected` outlines the node whose details show; `lane` on a node puts it in a named band (a pipeline's stage). Pan by dragging, zoom with the wheel; keyboard: arrows move between nodes (or pan), `+` `-` zoom, `0` fits, Enter selects. A minimap appears when the graph is bigger than its box; the view is kept when the same graph changes (a what-if value) and fitted afresh for a different one. The legend adds what the drawing uses: dashed nodes (`missingNode`), dashed edges (`dashedEdge`), statuses. A cut label's whole text is the node's tooltip |
| `EffectiveLines` | Config with an origin per line (CI, Compose) |
| `Popover`, `Dialog`, `Tooltip` | Overlays; dialogs trap focus and close on Esc |
| `ContextMenu` | Right-click menus (also Shift+F10); entries built per open from the event; sentence case, an icon per entry |
| `Badge`, `Kbd` | File type with logo; key combos |
| `EmptyState` | Nothing to show yet: icon, one sentence, the action that fills it |
| `ErrorNote` | Something failed: icon, the reason, and `retry` (Try again) or other ways forward; `inline` beside a field or a row. Never a bare red paragraph |
| `Skeleton` | Loading placeholders in the content's shape |
| `Working` | "Working…" in a pane's header while content already shown is worked out again; appears after 250 ms. Never a spinner |
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
  content already shown → `Working` in the header ("Reading…").
  Error → `ErrorNote`: a plain sentence, the reason, a way forward
  (`retry` wherever the thing can be asked for again); never a raw
  stack, never a spinner.
- **Commands and shortcuts.** App-wide commands in `App.svelte`; a view's
  own commands registered while it is active
  (`$effect(() => active && commands.register([...]))`, or
  `toolCommands` for tools). Shortcuts via `shortcut()`; add them to
  `ShortcutsDialog` and `docs/shortcuts.html`.
- **Tools.** One page: title and toolbar, input left, output right
  (`ToolLayout`, `InputPane`, `Pane`) when the input can be a document;
  the two resize, and a button at the end of the toolbar puts the
  input over the output instead (remembered per tool,
  `features/shared/panes.svelte.ts`). The output pane is `TextOutput`
  (`OutputPane` is the same, fed from a transform session).
  When it is a value or a small form (a timestamp, a cron expression,
  generator options), one column instead (`ToolLayout` `body`,
  `LineInput`, `Pane grow={false}`): the input on top, answers below,
  each as tall as its content. A tool with sub-tabs lists them in
  `lib/tools.ts` (`tabs`), shows them as a `SegmentedControl` first in its
  toolbar (`ToolTabs`) and renders through `TabbedTool`, which mounts a
  sub-tab when first shown and keeps it; the last sub-tab is remembered,
  and Home and the palette list each one by name. Requests go through
  `Job` / `TextJob` (newest answer wins, debounced while typing). One
  tool hands input to another with `handoff.send(tool, tab, input)`;
  every output pane offers the tools that take text (**Send to**,
  `features/shared/sendto.ts`): a tool that can take a text handed to
  it calls `handoff.take` and is listed there.
  One wording per action: **Try a sample**, **Clear** (the eraser),
  **Copy**. An option that is how a result is written (hex or base64,
  a diff's layout) is a display choice and is remembered; what was
  typed is not.
  Results with several values give each its own copy button.
- **Persistence.** `persisted()` is for display preferences only: the
  theme, sizes of panes, which tools are pinned or were opened last,
  the palette's last commands. Never persist tool input, what-if edits
  or variables [19].
- **Palette.** One list for everything that can be done: commands,
  tools and their sub-tabs, files of the open folder's lenses, and one
  entry per setting (`features/settings/goto.svelte.ts`: "Settings:
  Theme" opens the page on its row). The commands run last come first
  while nothing is typed.
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
- Keyboard focus visible (global 2 px accent ring); a field shows its
  border colour plus `--focus-ring` (global.css does this for every
  input); a component that removes either shows an equivalent.
- Every interactive element has an accessible name (text, `aria-label`
  or `label`). Icon-only buttons use `IconButton`.
- An id in a component that can be mounted more than once (every view:
  tabs stay mounted) starts with `$props.id()`, so a label points at
  its own field.
- A full-window overlay is a dialog: focus goes in, Tab stays in, and
  it goes back where it was. A value that changes every second is a
  `role="timer"`, never a live region.
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

1. Tokens only; shared components reused or extended, not copied. No
   hand-rolled field, chip, header, error line, spinner or fixed
   two-pane grid: `Chip`, `PanelHeader`, `ErrorNote`, `Working`,
   `SplitPane` exist for them.
2. Empty, loading and error states present.
3. Reachable by keyboard; has a palette entry if it is an action.
4. Accessible names on controls; contrast checked for new colour pairs.
5. Works at narrow widths (the app is used side by side with an editor).
6. `npm --prefix frontend run check` passes; screenshots in `docs/`
   updated if the view is shown there.
