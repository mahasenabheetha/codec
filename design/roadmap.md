# Roadmap

Each phase is one PR-sized chunk (1–3 sessions). Do them in order;
dependencies are listed in each phase file.

| # | Phase | Output | Status |
|---|---|---|---|
| 00 | [Foundation](phases/00-foundation.md) | `/v2` module, Svelte toolchain, CI on Linux+Windows | done |
| 01 | [UI shell + v1 port](phases/01-ui-shell.md) | Dark design system, app layout, v1 tools in Svelte | done |
| 02 | [YAML engine core](phases/02-yaml-engine.md) | `internal/yamlkit` + `codec yaml` CLI | done |
| 03 | [Workspace](phases/03-workspace.md) | Open folder read-only, index, file tree, viewer | done |
| 04 | [Editor intelligence](phases/04-editor.md) | Highlighting overlays, diagnostics, outline, hover, what-if edits | done |
| 05 | [Helm](phases/05-helm.md) | Embedded render, values layers + provenance, live preview | done |
| 06 | [MVP release](phases/06-mvp-release.md) | `v2.0.0-alpha.1`, Docker image, docs | done (user checks + tag pending) |
| 07 | [Lint + schemas](phases/07-lint.md) | Rules, JSON Schema, K8s checks, deprecations | done |
| 08 | [Compare + query](phases/08-compare.md) | Semantic diff, env diff, jq query | done |
| 09 | [Kubernetes lens](phases/09-kubernetes.md) | Cards, inventory, relations, neat, kustomize | done |
| 10 | [Argo lens](phases/10-argo.md) | Params, templateRef, DAG graph, ArgoCD apps | done |
| 11 | [CI pipelines lens](phases/11-ci.md) | GitHub Actions, GitLab CI, Azure Pipelines | done |
| 12 | [Ansible + Compose lens](phases/12-ansible-compose.md) | Playbook outline, Compose graph | done |
| 13 | [Scaffolding](phases/13-scaffolding.md) | Starters, clone-with-rename, snippets | done |
| 14 | [v2.0.0 release](phases/14-release.md) | Polish, performance, docs, stable release | done (user checks + tag pending) |

Pre-releases: `v2.0.0-alpha.N` after phase 06 and as later phases land.

**After v2 (not planned in detail):** v3 Wails desktop app; `codec lsp`
for VS Code/Cursor; user-defined lint rules (CEL); optional AI assist.
