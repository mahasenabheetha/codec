# AGENTS.md

Entry point for any AI agent or tool working on this repo. Keep reads
minimal: this file → `design/status.md` → the current phase file. Open
other design docs only when the phase file points to them.

## What codec is

A local, single-binary developer tool (Go backend + embedded web UI).
v1: base64 / JSON / JWT / Ansible-log tools. v2 (in progress): a
read-only YAML workbench — open a repo folder, understand, lint and
render YAML (Kubernetes, Helm, Argo, CI, Ansible, Compose). It is a
companion to VS Code/Cursor, not a replacement.

## Session workflow

1. Read `design/status.md` for the current phase and next step.
2. Read that phase file in `design/phases/`. Follow its scope exactly;
   do not pull in work from later phases.
3. Work on a feature branch (`feature/mab/<topic>`). **Never commit to
   `main`.** Don't push or merge unless the user asks.
4. Before finishing: tick completed acceptance items in the phase file,
   update `design/status.md` (state, next step, one log line), and add
   any new decision to `design/decisions.md`.

## Hard rules

- codec **never writes to the user's workspace files**. Edits in the UI
  are in-memory "what-if" changes only.
- Engine packages are pure (no I/O); I/O lives in adapter packages.
  See `design/conventions.md`.
- Don't re-litigate settled decisions in `design/decisions.md`; propose
  a change to the user instead.
- Tests: write table-driven tests for engine logic, but run only the
  packages you touched (`go test ./internal/<pkg>/...`). Full suite only
  right before a PR. The user does manual/UI testing.
- The user is learning Go: write idiomatic, commented code (comments
  explain *why*), and briefly name any new Go concepts in your final
  chat summary — not in files.
- Keep design docs short. Update facts; don't append essays.

## Commands

```bash
go vet ./... && go test ./internal/<pkg>/...   # backend
npm --prefix frontend ci                        # once, installs UI deps
npm --prefix frontend run dev                   # UI dev server; CODEC_API=http://127.0.0.1:<port> if not 8765
npm --prefix frontend run build                 # builds into internal/web/dist/app
go build -o codec ./cmd/codec                   # binary (needs frontend built)
scripts/dev.sh build|run|ui [port]              # all of the above in one step
```
