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
Branch `feature/mab/v2.3.0`. Shared rules in [Utilities](#utilities-applies-to-the-three-items-below).

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
| — | Release | | |

### Later
The rest of "Next", picked up after 2.3.0 or slotted in when it fits.

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
| [Utilities: Time](#utilities-time) | M | Timestamp and Cron (explainer + generator) in one sidebar entry |
| [Utilities: Encode & hash](#utilities-encode--hash) | S | URL, hex, hash/HMAC, secrets & UUID, htpasswd |
| [Utilities: Regex](#utilities-regex) | M | Tester and explainer; Go (RE2) and Python/.NET/JS styles |

## Later

| Item | Notes |
|---|---|
| Schema-driven form for any kind | Phase 13 stretch; uses the schema cache |
| `codec lsp` | Provider concepts already mirror LSP |
| User-defined lint rules (CEL) | Per-user rules in settings, never in repos |
| v3 desktop app (Wails) | The frontend talks to the backend only through `lib/api`; swap the transport |
| Optional AI assist | Must stay opt-in and local-first |
| Several folders open at once | e.g. an app and an infra repository; search and compare across them |
| References across folders | e.g. an Argo Application pointing at a chart in another local folder; needs the item above |

## Done

| Version | Items |
|---|---|
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

### Utilities (applies to the three items below)
- A new rail heading **Utilities** with three entries — Time, Encode &
  hash, Regex — each with sub-tabs (`SegmentedControl`, as in Compare).
  The last sub-tab is remembered (a UI preference, decision 19).
- Each sub-tab is in the command palette and on Home by its own name
  ("Cron" opens Time on the Cron tab). `lib/tools.ts` gains optional
  sub-tabs per tool so the rail, Home and palette stay generated from it.
- Same layout as the Base64 and JWT tools: input, options, output, copy.
  Base64, JSON and JWT stay where they are.
- Logic is Go, as for the existing tools (decision 18): a package in
  `internal/` per area, exposed through the API and as CLI commands.
  Input and output are never persisted (decision 19).
- Smart paste learns to recognise epoch numbers, URL-encoded text and
  cron expressions and opens the right sub-tab.

### Utilities: Time
- **Problem:** epoch timestamps and cron schedules are read by hand or
  in a browser tab elsewhere.
- **Result:** rail entry **Time** with two sub-tabs.
  - **Timestamp:** epoch seconds or milliseconds (detected) to local,
    UTC, ISO 8601 and relative time; a date back to epoch; a **Now**
    button with the current Unix timestamp.
  - **Cron:** explainer and generator in one: an expression shows plain
    words ("Every 15 minutes, 02:00–06:59, Monday to Friday") and the
    next 10 runs; a form (every N minutes, hourly at, daily at, weekdays,
    day of month) builds the expression live, and a pasted expression
    fills the form where it can. Standard 5 fields plus `@daily`-style
    shortcuts, with a time-zone choice (UTC for GitHub/Azure; a CronJob's
    `timeZone`). Quartz (6–7 fields, `?`) and Jenkins `H` are recognised
    and named as unsupported.
  - Hover on a schedule in the editor (CronJob `schedule`, GitHub
    `on.schedule.cron`, Azure `schedules.cron`) shows the explanation and
    next runs.
- **Scope:** `internal/timeutil` (or similar) and `internal/cron`
  (parser, next runs, describe, build), `internal/web`, CLI
  `codec time` / `codec cron`, `features/time`, the providers' hover;
  docs `tools.html`, `cli.html`.
- **Done when:** cron tests cover ranges, steps, lists, names (`MON`),
  shortcuts, day-of-month vs day-of-week, DST changes and the dialect
  messages; timestamp tests cover s/ms detection and time zones; hover
  works on the sample's CronJob and workflow; docs updated.
- **Decisions:** agreed 2026-10-03: standard 5 fields + shortcuts,
  explainer and generator as one tool, hover tips in the editor.

### Utilities: Encode & hash
- **Problem:** small encode/hash jobs need other tools or websites,
  which is risky with secrets.
- **Result:** rail entry **Encode & hash** with sub-tabs:
  - **URL:** encode/decode a value or a whole URL; a query string as a
    table.
  - **Hex:** hex ↔ text.
  - **Hash:** MD5 (marked "checksums only"), SHA-1, SHA-256, SHA-384,
    SHA-512 of text, and HMAC with a key.
  - **Secrets & UUID:** UUID v4; random secrets by length and character
    set, as text, hex or base64.
  - **htpasswd:** bcrypt `user:hash` lines for ingress basic auth.
- **Scope:** `internal/codec` (or a sibling package), `internal/web`,
  CLI commands (`codec url`, `codec hash`, …), `features/encode`; docs
  `tools.html`, `cli.html`. Out of scope: generic encrypt/decrypt (no
  interoperable format — agreed); Ansible Vault belongs to the Ansible work.
- **Done when:** tests use published test vectors for each hash and
  HMAC; bcrypt output verifies; docs updated.
- **Decisions:** agreed 2026-10-03: no generic encryption tool; bcrypt
  via `golang.org/x/crypto` (new dependency).

### Utilities: Regex
- **Problem:** regexes from Ansible, pipelines and tooling are tested by
  trial and error.
- **Result:** rail entry **Regex**, one screen: the pattern, flags as
  toggles, test lines with matches highlighted live, capture groups in a
  table, a replace preview (`$1`, `${name}`), and an explainer tree that
  describes each part in plain words; hovering a part highlights it in
  the pattern and the matches.
  - Two styles: **Go (RE2)** with the standard library, and
    **Python / .NET / JavaScript** with `dlclark/regexp2` (lookarounds,
    backreferences, `(?<name>)` and `(?P<name>)`). Each test has a time
    limit, so a pattern that backtracks badly stops with a message.
- **Scope:** `internal/regex` (run, explain: `regexp/syntax` for RE2,
  a small parser for the extra features), `internal/web`, CLI
  `codec regex`, `features/regex`; docs `tools.html`, `cli.html`.
- **Done when:** tests cover both styles, named groups, flags, replace,
  the time limit and the explainer for each construct; docs updated.
- **Decisions:** agreed 2026-10-03: logic in Go with two styles, tester
  and explainer as one tool; `dlclark/regexp2` (new dependency).
