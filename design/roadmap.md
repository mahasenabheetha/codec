# Roadmap and backlog

codec 2.0 is complete ([history.md](history.md)). New work — features,
improvements, known gaps — is listed here first, so everyone (people and
agents) picks it up the same way. Bugs go to GitHub issues; a bug that
needs a design change gets an item here too.

## How to add an item

Add a row to the right table, then a short section under
[Items](#items) using this template:

```markdown
### <short name>
- **Problem:** what is hard or impossible today, for whom.
- **Result:** what the user sees when it is done (UI and CLI).
- **Scope:** packages likely touched; what is explicitly out of scope.
- **Done when:** checks that prove it (tests, a screenshot on the sample, docs updated).
- **Decisions:** existing ones it relies on; new ones it needs (add to decisions.md when agreed).
```

Pick up an item by following [workflow.md](workflow.md#adding-a-feature).
When it ships, delete its section, move the row to "Done" with the
version, and add a CHANGELOG entry.

## Open checks

| Check | Owner |
|---|---|
| Run codec natively on macOS (arm64) before tagging | user |
| Confirm no open issues labelled for 2.0 | user |
| Merge `feature/mab/yaml-tools` and tag `v2.0.0` | user |
| Make the GHCR package public (first release with an image) | user |
| Turn on GitHub Pages: Settings → Pages → `main`, `/docs` | user |

## Next

| Item | Size | Notes |
|---|---|---|
| [Expand matrix jobs in the pipeline graph](#expand-matrix-jobs-in-the-pipeline-graph) | S | A matrix job calling a reusable workflow shows as one node |
| [Reload prompt after a server restart](#reload-prompt-after-a-server-restart) | S | Today requests fail until the page is reloaded |
| [Kustomize helmCharts](#kustomize-helmcharts) | M | Helm SDK is already embedded |
| [Window long lists](#window-long-lists) | S | Problems and Resources cards, like TreeView |
| [Compare: paste as a source](#compare-paste-as-a-source) | M | Third source per side; works with no folder open |
| [Compare: pick files from the Explorer](#compare-pick-files-from-the-explorer) | M | Drag and drop, plus a new Explorer context menu |
| [Compare: clear](#compare-clear) | S | Per-side × and a Clear button |

## Later

| Item | Notes |
|---|---|
| Schema-driven form for any kind | Phase 13 stretch; uses the schema cache |
| `codec lsp` | Provider concepts already mirror LSP |
| User-defined lint rules (CEL) | Per-user rules in settings, never in repos |
| v3 desktop app (Wails) | The frontend talks to the backend only through `lib/api`; swap the transport |
| Optional AI assist | Must stay opt-in and local-first |

## Done

| Version | Items |
|---|---|
| 2.0.0 | Everything in [history.md](history.md) |

## Items

### Expand matrix jobs in the pipeline graph
- **Problem:** a GitHub job with a `matrix` that calls a local reusable
  workflow is drawn as one node; the CLI already knows it runs × N.
- **Result:** one node per combination (or a badge "× 3" with the list
  on click), consistent with plain matrix jobs.
- **Scope:** `internal/ci` graph building, the Pipeline view.
- **Done when:** the sample's `ci.yml` shows three `test` runs; ci tests cover it.

### Reload prompt after a server restart
- **Problem:** each `codec serve` run has a new token, so a page left
  open fails with "can't reach the server" until reloaded.
- **Result:** the UI detects the token mismatch (401 with a known body)
  and shows "codec restarted — Reload".
- **Scope:** `internal/web` (distinguish token mismatch), `frontend/src/lib/api/client.ts`, shell.
- **Done when:** restarting codec shows the prompt; reload restores tabs.

### Kustomize helmCharts
- **Problem:** kustomizations using `helmCharts` can't be built; codec
  says so and points to the Helm view.
- **Result:** charts vendored in the repository are inflated with the
  embedded Helm SDK; remote charts stay unsupported (decision 3's spirit:
  never download).
- **Scope:** `internal/kube` (kustomize plugin hook), `internal/helm`.
- **Done when:** a kustomization with a local chart builds like `kubectl kustomize --enable-helm`.

### Window long lists
- **Problem:** the Problems view and Resources cards render every row;
  a repository with thousands of findings or objects gets slow.
- **Result:** only visible rows are in the DOM, like TreeView (decision 70).
- **Scope:** `frontend/src/features/lint`, `features/kube`, possibly a shared list component.
- **Done when:** 5,000 findings scroll smoothly; `scripts/perf.sh` notes it.

### Compare: paste as a source
- **Problem:** Compare only takes workspace files and Helm renders, and
  needs a folder open. Comparing a live object (`kubectl get -o yaml`),
  two API responses or two snippets means saving them as files first.
- **Result:** each side's source is File · Helm render · **Paste**; a
  Paste side is an editor box. Both sides pasted, or one pasted against
  a file or render. Comparing two pasted sides works with no folder open,
  reachable from Home and Tools ("Compare text") as well as the Compare tab.
  - Compared by structure (as today) only when both sides parse to a
    mapping or list; YAML and JSON mix freely (JSON is YAML). Anything
    else — plain text, logs, XML, ini, a lone scalar — gets the text diff,
    so two paragraphs don't show as one "value changed".
  - "Compare as: Auto · Structure · Text" (the API's `text` flag already
    forces a text diff); "Ignore whitespace" and "Ignore case" for text.
  - A "Live object noise" ignore preset: `metadata.managedFields`,
    `resourceVersion`, `uid`, `creationTimestamp`, `generation`, `status`.
- **Scope:** `internal/web/compare_api.go` (a `paste` side kind; no
  workspace needed when neither side reads one), `internal/textdiff`
  (whitespace/case options), `features/compare` (SidePicker, paste editor),
  Home and Tools entries, docs `compare.html`. Out of scope: per-format
  parsers (XML, ini); saving pasted text.
- **Done when:** compare_api tests cover paste × paste with no folder,
  paste × file, the scalar-to-text rule and the forced modes; the sample's
  Deployment against a pasted live copy with the noise preset shows only
  real changes; docs updated.
- **Decisions:** 19 (pasted text is never persisted, so it is gone after
  a reload — agreed), 44 (semantic diff identity). New: Paste is a third
  source per side, not a separate screen (agreed 2026-10-02).

### Compare: pick files from the Explorer
- **Problem:** a side's file can only be chosen through the quick-open
  search; the Explorer tree can't feed Compare.
- **Result:** drag a file from the Explorer onto the Left or Right side;
  the side shows "Choose a file… or drop one here". A new Explorer
  context menu (right-click) has "Select for compare", "Compare with
  selected", and with two files Ctrl+clicked, "Compare selected". The
  search stays. No second tree inside Compare, and a plain click in the
  Explorer still opens the file.
- **Scope:** `features/workspace` (Explorer/TreeView: context menu,
  drag source, Ctrl+click selection), `features/compare/SidePicker.svelte`
  (drop target), a shared context-menu component in `lib/components`,
  docs `compare.html` and `workspace.html`, `shortcuts.html`. Out of
  scope: other menu entries (copy path, reveal, lint this file) — the
  menu is built so they can be added later.
- **Done when:** dragging and each menu entry open the right comparison on
  the sample; the menu works with the keyboard (Shift+F10 / context-menu
  key, arrows, Esc); docs updated.
- **Decisions:** agreed 2026-10-02: drag and drop plus a context menu,
  one Explorer tree only, click keeps meaning "open".

### Compare: clear
- **Problem:** resetting Compare means re-picking both sides by hand.
- **Result:** an × on each side clears that side; "Clear" next to Swap
  empties both sides and the result. Saved ignore patterns stay. No
  confirmation; when a pasted side is cleared, a brief "Cleared · Undo"
  toast brings it back (memory only, decision 19).
- **Scope:** `features/compare` (CompareView, SidePicker, compare.svelte.ts).
- **Done when:** both buttons reset the view; Undo
  restores pasted text; docs `compare.html` updated.
- **Decisions:** none new.
