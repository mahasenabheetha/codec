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
