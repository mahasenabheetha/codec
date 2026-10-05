# AGENTS.md

Entry point for any AI agent or tool working on this repository. Keep
reads minimal: this file → [design/workflow.md](design/workflow.md) →
only the design files the task needs ([design/README.md](design/README.md)
lists them).

## What codec is

A local, single-binary developer tool: a Go backend with an embedded
Svelte web UI and a CLI. It is a **local workbench for DevOps
engineers**: read-only on the YAML of
Kubernetes, Helm, Kustomize, Argo, GitHub Actions, GitLab CI, Azure
Pipelines, Compose and Ansible (render, lint, schemas, compare, query,
lens views, starters), plus everyday tools (base64, JSON, JWT, Ansible
logs, and Utilities: URL, hex, hashes, secrets, htpasswd, timestamps,
cron, regex). It is a companion to VS Code/Cursor, not a replacement.
Releases are 2.x minors; the user uses it daily, so fixes and small
improvements are the common work. User docs: [docs/](docs/index.html).

## How to work

1. Read [design/workflow.md](design/workflow.md): where things live,
   how to fix a bug, how to add a feature, what to update.
2. New work is planned in [design/roadmap.md](design/roadmap.md);
   choices are recorded in [design/decisions.md](design/decisions.md).
3. Branch from `main` (`fix/<topic>`, `feature/mab/<topic>`); never
   commit to `main`. Don't push or open pull requests unless asked;
   the user does.
4. Before finishing: tests for what you changed pass, docs and
   CHANGELOG updated per the workflow matrix, and a short summary to
   the user of what changed and what they should check.

## Hard rules

- codec **never writes to the folders it opens**. UI edits are
  in-memory "what-if" changes only.
- No config files in user repositories; settings live in the user
  profile (`internal/config`).
- Engine packages are pure (no files, network, clock, environment);
  I/O lives in adapter packages. See design/architecture.md.
- Nothing pasted into tools is persisted; no third-party calls except
  schema downloads.
- UI uses design tokens and shared components only
  (design/design-language.md).
- Don't re-litigate settled decisions; propose a change to the user.
- Tests: table-driven, a named regression case for every fixed bug.
  Run only the packages you touched (`go test ./internal/<pkg>/...`);
  the full suite once before a pull request. The user does UI testing.
- The user is learning Go: write idiomatic, commented code (comments
  explain *why*), and name new Go concepts in your chat summary, not in
  files.
- Keep docs short and factual. Update facts; don't append essays.

## Commands

```bash
go vet ./... && go test ./internal/<pkg>/...   # backend
npm --prefix frontend ci                        # once, installs UI deps
npm --prefix frontend run check                 # UI type-check
npm --prefix frontend run build                 # builds into internal/web/dist/app
go build -o codec ./cmd/codec                   # binary (needs the UI built)
scripts/dev.sh build|run|ui [port]              # all of the above in one step
scripts/desktop.sh [exe]                        # Windows desktop exe + installer in dist/desktop (NSIS: MAKENSIS=…)
scripts/perf.sh [dir]                           # time big inputs against the budgets
codec serve --sample                            # try changes on the sample repository
```
