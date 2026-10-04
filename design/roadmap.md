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
| Make the GHCR package public (first release with an image) | user |
| Turn on GitHub Pages: Settings → Pages → `main`, `/docs` | user |
| Apply to SignPath Foundation for Windows code signing (before 3.0.0 T6; see [Code signing](#code-signing)) | user |

## Releases

Agreed 2026-10-03 (decision 73). Semantic Versioning: new features are
minor releases (2.x.0); patch releases (2.x.1) are bug fixes only.

### Routine for every version

1. Start from an up-to-date `main`: `git switch -c feature/mab/v2.X.0`.
2. Do the tasks in order. Each is finished before the next: code, tests
   for the packages it touches only, its docs page, its CHANGELOG line
   under "Unreleased". Commits: `2.X.0 T<n>: <what>`.
3. After each task, stop with a short summary and how to try it
   (`scripts/dev.sh run`); continue when the user says so.
4. Finish: `go vet ./...`, `go test ./...`, frontend build and
   `svelte-check`; docs pass over the changed pages; CHANGELOG
   "Unreleased" → `## [2.X.0] - <date>`; roadmap rows → "Done".
5. The user pushes, opens the PR and merges; then tags `main`
   (`git tag -a v2.X.0 -m "codec v2.X.0"`, `git push origin v2.X.0`);
   the release workflow publishes ([releases.md](releases.md)). Copy the
   new binary to the local launcher folder.
6. A bug in a released version meanwhile: `fix/<topic>` from `main`,
   released as a patch (2.X.1); then merge `main` into the open version
   branch.

Work as the code already does: follow [workflow.md](workflow.md),
[conventions.md](conventions.md) and
[design-language.md](design-language.md); reuse existing components;
keep the UI consistent and polished. Work efficiently: read only what a
task needs and run only the tests for packages touched (the full suite
runs once, at step 4).

### v2.1.0 — Compare, reload prompt, Find in Files
Branch `feature/mab/v2.1.0`.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Compare: clear | × per side, Clear next to Swap; ignore patterns kept | Done |
| T2 | Compare paste: API | `paste` side kind; no folder needed; structure only for mappings/lists; whitespace and case options in `textdiff`; tests | Done |
| T3 | Compare paste: UI | Third source with an editor box; "Compare as"; "Live object noise" preset; "Compare text" on Home and in the palette; "Cleared · Undo"; docs | Done |
| T4 | Explorer: context menu + Ctrl+click | Shared context-menu component; multi-select; keyboard | Done |
| T5 | Explorer → Compare | Drag and drop onto a side; "Select for compare", "Compare with selected", "Compare selected"; docs | Done |
| T6 | Reload prompt | Server marks a stale token; "codec restarted — Reload" banner; docs | Done |
| T7 | Find in Files: engine + API + CLI | `internal/search`, endpoint, result cap; tests; `codec search` | Done |
| T8 | Find in Files: UI | Search panel in the Explorer's slot, Ctrl+Shift+F, results open at the line; docs | Done |
| T9 | Compare: side-by-side text diff | Changed characters marked, folded context, Unified toggle; each picker heads its column (feedback on T3) | Done |
| — | Release | Full checks passed; CHANGELOG cut as 2.1.0 | Released as v2.1.0 |

### v2.2.0 — Ansible log analyzer, playbook map
Branch `feature/mab/v2.2.0`. Large logs are dropped or picked as a file and sent as is, up to 64 MB (decision 76).

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Analyzer stage 1: clean | ANSI, `\r`, CI timestamps/markers, learned wrapper prefixes; tests | Done |
| T2 | Stage 2: segment | Anchors, several runs, "other output", stray lines to their task; tests | Done |
| T3 | Stage 3: read results | Brace matching → YAML → raw; loops, retries, ignored, rescued, unreachable, `no_log`; `json` callback | Done |
| T4 | Stage 4: model + API + CLI | Run → plays → tasks → host results, recap, durations; `codec ansible log`; synthetic fixtures; fuzz test | Done |
| T5 | Analyzer UI | Summary bar, outline, filters, host picker, recap table, other-output blocks, file drop, first failure selected; existing task view as detail; docs | Done |
| T6 | Playbook map: graph | From the execution-order analysis; static/dynamic includes, "assumed", external roles; tests | Done |
| T7 | Playbook map: UI | Pipeline graph component, click to open; docs | Done |
| T8 | Run colours on the map | Failed, changed, ok, never ran | Done |
| — | Release | Full checks passed; CHANGELOG cut as 2.2.0 (the map shipped with the analyzer) | Released as v2.2.0 |

### v2.3.0 — Utilities
Branch `feature/mab/v2.3.0`. Decisions 78–80; the design notes are in this file's history.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Utilities setup | Sub-tabs in `lib/tools.ts`, "Utilities" rail heading, palette and Home entries per sub-tab; placeholders until each tool lands | Done |
| T2 | [Encode & hash](#utilities-encode--hash): engine + API + CLI | URL, hex, hash/HMAC, secrets & UUID, htpasswd (bcrypt); test vectors; `internal/encode`, `POST /api/v2/encode/{kind}`, decision 78 | Done |
| T3 | Encode & hash: UI | Five sub-tabs; docs | Done |
| T4 | [Time](#utilities-time): engine + API + CLI | Timestamps; cron parse, next runs, describe, build; dialect messages; DST tests; `internal/cron`, `internal/timeutil`, `POST /api/v2/time/{kind}`, decision 79 | Done |
| T5 | Time: UI | Timestamp with Now; cron explainer + generator; docs | Done |
| T6 | Cron hover | CronJob, GitHub and Azure schedules | Done (Argo CronWorkflow too; sample has one of each) |
| T7 | [Regex](#utilities-regex): engine + API + CLI | RE2 and regexp2, time limit, replace, explainer; tests; `internal/regex`, `POST /api/v2/regex`, decision 80 | Done |
| T8 | Regex: UI | Tester and explainer on one screen; docs | Done |
| T9 | Smart paste | Epoch numbers, URL-encoded text, cron expressions | Done (`codec.DetectUtility`; the clipboard watcher and `codec auto` are unchanged) |
| — | Release | Three parallel reviews (engines, API and CLI, frontend), fixes with regression tests and a fuzz test; full checks passed; CHANGELOG cut as 2.3.0 | Released as v2.3.0 |

### v2.4.0 — Light theme
Agreed 2026-10-03 (decision 81). Every colour is already a token in
`tokens.css` (no hard-coded colours in components), so this is a second
token set, a setting and a contrast pass.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Light palette | Light values for surfaces, borders, text, accent, status, syntax, diff, expression tints, graph groups and logos; every text pair ≥ 4.5:1; `design-language.md` gains the light column | Done |
| T2 | Theme switching | `[data-theme="light"]` in `tokens.css`, `color-scheme` per theme; a script in `index.html` sets the theme before first paint (no flash) | Done |
| T3 | Setting | System / Dark / Light in Settings and the palette, persisted as a display preference; System follows the OS live | Done |
| T4 | Pass over every view | Editor, compare, graphs, lens views, tools, dialogs and toasts in both themes; `CloneView` fallback colours to tokens; docs (`docs/design.html`, settings) | |
| — | Release | Full checks; CHANGELOG cut as 2.4.0 | |

### v3.0.0 — Desktop app
Agreed 2026-10-03 (decision 82). Wails v3 (system tray needs it), one
repo, Windows first; macOS when a Mac is available. The desktop app is a
second entry point to the same server and UI, never a second copy of a
feature: features go in `internal/` and `frontend/` as before, and
desktop-only extras sit behind `frontend/src/lib/platform.ts` with a
browser fallback, so the web UI (`codec serve`) keeps working unchanged.

```
cmd/codec/            CLI + codec serve (unchanged)
cmd/codec-desktop/    window, tray, menus; mounts the same server
frontend/             same app; lib/platform.ts for desktop extras
desktop/              Wails config, icons, installer
```

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Spike | `cmd/codec-desktop` loads the current app: the server's handler mounted in Wails (no port, no token) or, if SSE streaming fails there, a loopback server with the token; check live file events, clipboard, persisted settings, drag and drop, file drop for logs; confirm Wails v3 status | |
| T2 | Window | Single instance (second launch focuses the window and passes its file), window size and position remembered, native menus mapped to palette commands, title bar follows the theme | |
| T3 | Native dialogs | Open folder / file through `lib/platform.ts`; the web keeps its folder browser | |
| T4 | Tray and auto-start | Tray menu (open, recent folders, quit); closing the window keeps codec in the tray; "Start at login" setting (per-user Run key) | |
| T5 | Installer | Per-user NSIS (`RequestExecutionLevel user`) into `%LOCALAPPDATA%\Programs\codec`: Start menu entry, uninstall entry in HKCU, "Open with codec" for `.yaml`/`.yml` in HKCU; no admin; a portable exe as well | |
| T6 | Release build and signing | Desktop build in the release workflow next to the CLI; version and product metadata on the exe; signing step through [SignPath](#code-signing) (off until approved) | |
| T7 | Docs | Install, desktop features, uninstall; README and docs site; `architecture.md` | |
| — | Release | Full checks; CHANGELOG cut as 3.0.0 | |

Later, with a Mac: build on a macOS runner (cgo, Xcode), Apple Developer
ID signing and notarization, macOS menu bar, Dock and window-close
behaviour.

#### Code signing
Unsigned builds show SmartScreen's "unknown publisher" warning. codec is
MIT-licensed and public, so it can apply to the SignPath Foundation for
free Windows signing. Their conditions, and what codec needs:

| Condition | codec |
|---|---|
| OSI licence, no proprietary components | MIT; dependencies are open source; WebView2 counts as a system library |
| Existing releases, functionality described on the download page | Releases since v2.0.0; README and docs site |
| Binaries built from source by CI, verifiably | Release workflow on GitHub Actions with SignPath's action |
| MFA on GitHub and SignPath for every team member | User to turn on |
| Roles: authors, reviewers, approvers; manual approval per release | The user holds all three |
| Code signing policy page on the homepage, with attribution and a privacy statement | A `docs/` page: "Free code signing provided by SignPath.io, certificate by SignPath Foundation"; codec sends no data anywhere |
| Product name and version metadata on signed binaries | Set by the Windows build (T6) |
| Uninstaller; no undisclosed data collection or system changes | Installer has one; nothing collected |

The certificate names SignPath Foundation as the publisher, not the
author. SignPath signs Windows only; macOS needs Apple's program.

### Later
The rest of "Next", picked up when it fits.

## Next

| Item | Size | Notes |
|---|---|---|
| [Expand matrix jobs in the pipeline graph](#expand-matrix-jobs-in-the-pipeline-graph) | S | A matrix job calling a reusable workflow shows as one node |
| [Kustomize helmCharts](#kustomize-helmcharts) | M | Helm SDK is already embedded |
| [Window long lists](#window-long-lists) | S | Problems and Resources cards, like TreeView |
| [Helm environment matrix](#helm-environment-matrix) | M | All profiles of a chart at once: what differs per environment |
| [Kubernetes resource totals](#kubernetes-resource-totals) | S | Requests and limits × replicas, summed |
| [RBAC view](#rbac-view) | M | Who can do what; flags wildcards and cluster-admin |
| [NetworkPolicy view](#networkpolicy-view) | M | Which pods can talk to which |

## Later

| Item | Notes |
|---|---|
| Schema-driven form for any kind | Phase 13 stretch; uses the schema cache |
| `codec lsp` | Provider concepts already mirror LSP |
| User-defined lint rules (CEL) | Per-user rules in settings, never in repos |
| Optional AI assist | Must stay opt-in and local-first |
| Several folders open at once | e.g. an app and an infra repository; search and compare across them |
| References across folders | e.g. an Argo Application pointing at a chart in another local folder; needs the item above |

## Done

| Version | Items |
|---|---|
| 2.3.0 | Utilities: Encode & hash (URL, hex, hash/HMAC, secrets & UUID, htpasswd), Time (timestamps, cron explainer and builder, schedule hover), Regex (tester and explainer); smart paste for their input |
| 2.2.0 | Ansible log analyzer: whole pipeline logs (UI and `codec ansible log`); Ansible playbook map coloured by a run |
| 2.1.0 | Compare: clear; Compare: paste as a source; Compare: pick files from the Explorer; Reload prompt after a server restart; Find in Files |
| 2.0.0 | Everything in [history.md](history.md) |

## Items

### Expand matrix jobs in the pipeline graph
- **Problem:** a GitHub job with a `matrix` that calls a local reusable
  workflow is drawn as one node; the CLI already knows it runs × N.
- **Result:** one node per combination (or a badge "× 3" with the list
  on click), consistent with plain matrix jobs.
- **Scope:** `internal/ci` graph building, the Pipeline view.
- **Done when:** the sample's `ci.yml` shows three `test` runs; ci tests cover it.

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

### Helm environment matrix
- **Problem:** Compare shows two renders at a time; seeing how dev, test
  and prod drift apart means many pairwise compares.
- **Result:** for a chart, render it with every saved profile (and the
  defaults) and show one table: objects present per profile, and the
  fields that differ (replicas, images, resources, env, hosts), one
  column per profile. A cell opens the pairwise Compare for those two.
- **Scope:** `internal/helm` (render each profile, reuse the semantic
  diff), `internal/web`, `features/helm`; docs `helm.html`. Out of scope:
  values files that aren't saved profiles; rendering in parallel beyond
  what the Helm view already does.
- **Done when:** the sample chart with two or more profiles shows the
  expected differences; helm tests cover the table; docs updated.
- **Decisions:** 44 (semantic diff identity); profiles stay in the user
  profile, never in the repository.

### Kubernetes resource totals
- **Problem:** how much a release asks of the cluster (CPU, memory) is
  scattered across containers and replica counts.
- **Result:** in Resources (and per Helm render): requests and limits
  per workload multiplied by replicas, summed per namespace and overall,
  with init containers counted as Kubernetes does (max, not sum) and
  containers without values listed as "not set". HPA min/max shown when
  present.
- **Scope:** `internal/kube` (quantity parsing and sums), `features/kube`;
  docs `kubernetes.html`.
- **Done when:** kube tests cover quantities (`100m`, `1.5`, `512Mi`,
  `1G`), replicas, init containers and missing values; the sample shows
  correct totals.
- **Decisions:** none new.

### RBAC view
- **Problem:** what a ServiceAccount, user or group may do is spread over
  Roles, ClusterRoles and their bindings.
- **Result:** a table per subject: verbs × resources × namespace, with
  the binding and role it comes from. Flags `*` verbs or resources,
  `cluster-admin` bindings and bindings to roles not in the repository.
- **Scope:** `internal/kube` (graph already links objects), `features/kube`,
  possibly lint rules; docs `kubernetes.html`. Out of scope: aggregated
  ClusterRoles from the cluster, built-in roles beyond a known list.
- **Done when:** kube tests cover Role vs ClusterRole bindings and
  wildcards; the sample shows its ServiceAccount's rights.
- **Decisions:** none new.

### NetworkPolicy view
- **Problem:** NetworkPolicy YAML is hard to read: which pods can reach
  which, on which ports, is not obvious.
- **Result:** per namespace, a diagram and a table: for each workload,
  allowed ingress and egress (pods, namespaces, CIDRs, ports), and
  workloads not selected by any policy (allow all). States clearly that
  it shows the repository's policies, not a cluster's.
- **Scope:** `internal/kube` (selector matching exists), `features/kube`;
  docs `kubernetes.html`. Out of scope: CNI-specific policies (Calico,
  Cilium CRDs).
- **Done when:** kube tests cover podSelector, namespaceSelector, ipBlock,
  ports and default deny; the sample has a policy to show.
- **Decisions:** none new.
