# 04 — Editor intelligence

**Goal:** YAML becomes easy to read: overlays, diagnostics, outline,
paths, hover — plus in-memory what-if edits.
**Depends on:** 03 · **Branch:** `feature/mab/p04-editor`
**Read:** architecture.md (provider model, API), ui.md (syntax colors)

## Scope

Backend (provider-backed, content optional so what-if buffers work):
- `POST /api/v2/yaml/analyze` {path, content?} → type, docs,
  diagnostics, symbols, expression spans.
- `POST /api/v2/yaml/hover`, `/definition`, `/complete` {path, content, pos}.
  Generic provider: path + value hover, in-file anchors/aliases.

Frontend (CodeView grows into the full editor):
- Expression overlays from engine spans (template-time vs runtime colors).
- Lint gutter + squiggles; problems list for the file.
- Outline panel synced with cursor (click → jump; current node highlighted).
- Status-bar breadcrumb; click → copy path in any format from phase 02.
- Doc switcher for multi-doc files; folding; search; go to line.
- **What-if edits:** editor is editable; buffer ≠ disk shows a
  "what-if" badge with Reset, Copy content, Copy diff (unified diff vs
  disk). Never saved. Disk change while dirty → ask keep/reload.
- "Open in VS Code / Cursor" (`vscode://file/<abs>:<line>`,
  `cursor://file/…`); editor choice in settings.
- Analyze debounced ~150 ms; stale requests cancelled.

## Out of scope

Schema completion/validation (07), lens-specific hovers (05, 09+).

## Acceptance

- [ ] Helm template shows YAML + template overlays with no false parse errors.
- [ ] Every friendly error from 02 appears as a squiggle with its message.
- [ ] Outline ↔ cursor sync both ways; breadcrumb copy works for all formats.
- [ ] What-if edit → Copy diff produces a patch that applies cleanly
      with `git apply`.
- [ ] Typing stays smooth on a 5k-line file.

**Go concepts:** request-scoped cancellation (`r.Context()`), diff algorithms.
